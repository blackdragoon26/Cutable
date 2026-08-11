import 'dart:async';

import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../data/models/conversation.dart';
import '../../../data/models/ws_event.dart';
import '../../../data/ws_client.dart';
import '../../auth/providers/auth_providers.dart';

enum ChatMessageType { text, tool, error }

class ChatMessage {
  const ChatMessage({required this.type, required this.contents, required this.createdAt});

  final ChatMessageType type;
  final String contents;
  final DateTime createdAt;
}

class StageInfo {
  const StageInfo({required this.stage, required this.message, required this.progress});

  final String stage;
  final String message;
  final num progress;
}

const _stageLabels = {
  'initializing': 'Initializing',
  'ingest': 'Loading Context',
  'planning': 'Creating Plan',
  'executing': 'Executing Plan',
  'verifying': 'Verifying Build',
  'finalizing': 'Wrapping Up',
  'complete': 'Complete',
  'error': 'Error',
};

class AgentSessionState {
  const AgentSessionState({
    this.messages = const [],
    this.stage,
    this.connectionState = WsConnectionState.disconnected,
    this.credentialsRequired = false,
    this.sandboxReady = false,
  });

  final List<ChatMessage> messages;
  final StageInfo? stage;
  final WsConnectionState connectionState;
  final bool credentialsRequired;
  final bool sandboxReady;

  AgentSessionState copyWith({
    List<ChatMessage>? messages,
    StageInfo? stage,
    WsConnectionState? connectionState,
    bool? credentialsRequired,
    bool? sandboxReady,
  }) {
    return AgentSessionState(
      messages: messages ?? this.messages,
      stage: stage ?? this.stage,
      connectionState: connectionState ?? this.connectionState,
      credentialsRequired: credentialsRequired ?? this.credentialsRequired,
      sandboxReady: sandboxReady ?? this.sandboxReady,
    );
  }
}

/// Mirrors apps/frontend/app/hooks/useAgentSession.ts's event formatting
/// switch statement and apps/frontend/app/hooks/useWebSocket.ts's
/// connection lifecycle, adapted to a Riverpod StateNotifier driving the
/// Chat tab, plus signals (sandboxReady) the Files/Preview tabs listen to.
class AgentSessionController extends StateNotifier<AgentSessionState> {
  AgentSessionController(this._client) : super(const AgentSessionState()) {
    _stateSub = _client.connectionState.listen((connectionState) {
      state = state.copyWith(connectionState: connectionState);
    });
    _eventSub = _client.events.listen(_handleEvent);
    _client.connect();
  }

  final WsClient _client;
  late final StreamSubscription<WsConnectionState> _stateSub;
  late final StreamSubscription<WsEvent> _eventSub;

  /// Populates the chat pane from persisted history (GET
  /// /api/projects/{id}/conversations) when reopening a project that was
  /// already built — otherwise the pane stays empty forever, since the
  /// agent only re-runs for brand-new projects with no generated files.
  /// A no-op if live messages have already started arriving over the
  /// socket, so this can't clobber an in-progress run.
  void seedHistory(List<Conversation> history) {
    if (state.messages.isNotEmpty || history.isEmpty) return;
    state = state.copyWith(
      messages: history
          .map((c) => ChatMessage(
                type: switch (c.type) {
                  'TOOL_CALL' => ChatMessageType.tool,
                  'ERROR_MESSAGE' => ChatMessageType.error,
                  _ => ChatMessageType.text,
                },
                contents: c.contents,
                createdAt: c.createdAt,
              ))
          .toList(),
    );
  }

  void sendPrompt(String prompt, {String? openRouterApiKey, String? e2bApiKey}) {
    _appendMessage(ChatMessageType.text, prompt);
    _client.startAgent(prompt: prompt, openRouterApiKey: openRouterApiKey, e2bApiKey: e2bApiKey);
  }

  void _appendMessage(ChatMessageType type, String contents) {
    state = state.copyWith(messages: [
      ...state.messages,
      ChatMessage(type: type, contents: contents, createdAt: DateTime.now()),
    ]);
  }

  void _handleEvent(WsEvent event) {
    switch (event.type) {
      case 'stage_update':
        state = state.copyWith(
          stage: StageInfo(
            stage: _stageLabels[event.stage] ?? event.stage ?? '',
            message: event.message ?? '',
            progress: event.progress ?? 0,
          ),
        );
        return;
      case 'stage_error':
      case 'connected':
      case 'pong':
      case 'agent_thinking':
      case 'agent_completed':
      case 'graph_node_update':
      case 'context_loaded':
      case 'context_saved':
        return;
      case 'credentials_required':
        state = state.copyWith(credentialsRequired: true);
        _appendMessage(ChatMessageType.error, event.message ?? 'Provider keys are required to continue.');
        return;
      case 'plan_generated':
        final plan = (event.data['plan'] as List<dynamic>? ?? []).cast<String>();
        final planText = plan.isEmpty
            ? (event.message ?? 'Plan generated successfully.')
            : 'Plan generated:\n${plan.asMap().entries.map((e) => '${e.key + 1}. ${e.value}').join('\n')}';
        _appendMessage(ChatMessageType.text, planText);
        return;
      case 'plan_error':
        _appendMessage(ChatMessageType.error, event.message ?? 'Failed to generate plan.');
        return;
      case 'agent_started':
        _appendMessage(ChatMessageType.text, event.message ?? 'Starting to process your request...');
        return;
      case 'agent_response':
      case 'agent_final_response':
        if (event.message != null) _appendMessage(ChatMessageType.text, event.message!);
        return;
      case 'agent_error':
        _appendMessage(ChatMessageType.error, event.message ?? 'Agent encountered an error.');
        return;
      case 'step_completed':
        final remaining = event.data['remainingSteps'] as int? ?? 0;
        _appendMessage(ChatMessageType.text,
            '✓ ${event.data['step']}${remaining > 0 ? ' ($remaining steps remaining)' : ''}');
        return;
      case 'tool_started':
        _appendMessage(ChatMessageType.tool, 'Running: ${(event.tool ?? 'tool').replaceAll('-', ' ')}...');
        return;
      case 'tool_completed':
        _appendMessage(ChatMessageType.tool, '✓ ${(event.tool ?? 'tool').replaceAll('-', ' ')} completed');
        return;
      case 'tool_error':
        _appendMessage(ChatMessageType.error, '✗ ${event.tool ?? 'tool'} failed: ${event.data['error'] ?? 'Unknown error'}');
        return;
      case 'dependency_installation_started':
        _appendMessage(ChatMessageType.text, event.message ?? 'Installing dependency...');
        return;
      case 'dependency_installation_completed':
        _appendMessage(ChatMessageType.text, event.message ?? '✓ Dependency installed.');
        return;
      case 'dependency_error':
        _appendMessage(ChatMessageType.error, event.message ?? 'Dependency installation failed.');
        return;
      case 'verification_result':
        final status = event.data['status'] as String? ?? 'info';
        final isError = status == 'failure' || status == 'error';
        _appendMessage(isError ? ChatMessageType.error : ChatMessageType.text,
            event.message ?? 'Verification update.');
        return;
      case 'sandbox_created':
        state = state.copyWith(sandboxReady: true);
        _appendMessage(ChatMessageType.text, '✓ Development environment ready');
        return;
      case 'file_created':
        _appendMessage(ChatMessageType.text, '✓ Created: ${event.filepath ?? 'file'}');
        return;
      case 'files_created':
        final files = event.data['files'] as List<dynamic>?;
        final count = files?.length ?? event.data['count'] as int? ?? 0;
        _appendMessage(ChatMessageType.text, '✓ Created $count files');
        return;
      case 'file_deleted':
        _appendMessage(ChatMessageType.text, '✓ Deleted: ${event.filepath ?? 'file'}');
        return;
      case 'file_renamed':
        _appendMessage(ChatMessageType.text,
            '✓ Renamed: ${event.data['oldPath'] ?? 'file'} → ${event.data['newPath'] ?? 'new location'}');
        return;
      case 'build_started':
        _appendMessage(ChatMessageType.text, 'Building project...');
        return;
      case 'build_test_success':
        _appendMessage(ChatMessageType.text, '✓ Build successful!');
        return;
      case 'build_test_failed':
      case 'build_test_error':
        _appendMessage(ChatMessageType.error,
            '✗ Build failed: ${event.message ?? event.data['error'] ?? 'Unknown error'}');
        return;
      default:
        if (kDebugMode) debugPrint('Unknown WebSocket event: ${event.type}');
    }
  }

  @override
  void dispose() {
    _stateSub.cancel();
    _eventSub.cancel();
    _client.dispose();
    super.dispose();
  }
}

final agentSessionProvider =
    StateNotifierProvider.autoDispose.family<AgentSessionController, AgentSessionState, String>(
  (ref, projectId) {
    final client = WsClient(ref.watch(secureStorageProvider), projectId: projectId);
    return AgentSessionController(client);
  },
);
