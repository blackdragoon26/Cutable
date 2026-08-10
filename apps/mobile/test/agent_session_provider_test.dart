// Unit tests for AgentSessionController's WS event -> chat message
// formatting (lib/features/workspace/providers/agent_session_provider.dart),
// mirroring the frontend's useAgentSession.ts switch statement.
//
// AgentSessionController is driven by a real WsClient rather than a
// hand-rolled fake/interface: WsClient now accepts an injectable
// `channelFactory` (added for ws_client_test.dart), so we connect it to a
// FakeWebSocketChannel and push server frames through
// `channel.addIncoming(json)`. That exercises the real
// WsClient -> AgentSessionController._handleEvent pipeline end-to-end,
// without a real socket.
import 'dart:convert';

import 'package:cutable_mobile/data/secure_storage.dart';
import 'package:cutable_mobile/data/ws_client.dart';
import 'package:cutable_mobile/features/workspace/providers/agent_session_provider.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'support/fake_web_socket_channel.dart';

class MockSecureStorage extends Mock implements SecureStorage {}

void main() {
  late MockSecureStorage secureStorage;
  late FakeWebSocketChannel channel;
  late WsClient client;
  late AgentSessionController controller;

  setUp(() async {
    secureStorage = MockSecureStorage();
    when(() => secureStorage.readAuthToken()).thenAnswer((_) async => null);

    client = WsClient(
      secureStorage,
      projectId: 'proj-1',
      channelFactory: (uri, {headers}) {
        channel = FakeWebSocketChannel(headers: headers);
        return channel;
      },
    );
    controller = AgentSessionController(client);
    // The controller's constructor calls client.connect(); flush that.
    await Future<void>.delayed(Duration.zero);
  });

  tearDown(() {
    controller.dispose();
  });

  Future<void> send(Map<String, dynamic> event) async {
    channel.addIncoming(jsonEncode(event));
    await Future<void>.delayed(Duration.zero);
  }

  group('stage_update', () {
    test('maps known stage codes to their display label', () async {
      await send({'e': 'stage_update', 'stage': 'planning', 'message': 'Thinking...', 'progress': 42});

      expect(controller.state.stage?.stage, 'Creating Plan');
      expect(controller.state.stage?.message, 'Thinking...');
      expect(controller.state.stage?.progress, 42);
    });

    test('falls back to the raw stage code when unmapped', () async {
      await send({'e': 'stage_update', 'stage': 'mystery_stage'});

      expect(controller.state.stage?.stage, 'mystery_stage');
      expect(controller.state.stage?.message, '');
      expect(controller.state.stage?.progress, 0);
    });
  });

  group('tool lifecycle', () {
    test('tool_started appends a tool message', () async {
      await send({'e': 'tool_started', 'tool': 'file-writer'});

      expect(controller.state.messages, hasLength(1));
      final msg = controller.state.messages.single;
      expect(msg.type, ChatMessageType.tool);
      expect(msg.contents, 'Running: file writer...');
    });

    test('tool_completed appends a completion tool message', () async {
      await send({'e': 'tool_completed', 'tool': 'file-writer'});

      final msg = controller.state.messages.single;
      expect(msg.type, ChatMessageType.tool);
      expect(msg.contents, '✓ file writer completed');
    });

    test('tool_error appends an error message with the failure reason', () async {
      await send({
        'e': 'tool_error',
        'tool': 'sandbox',
        'error': 'timeout',
      });

      final msg = controller.state.messages.single;
      expect(msg.type, ChatMessageType.error);
      expect(msg.contents, '✗ sandbox failed: timeout');
    });
  });

  group('sandbox_created', () {
    test('sets sandboxReady and appends a confirmation message', () async {
      expect(controller.state.sandboxReady, isFalse);

      await send({'e': 'sandbox_created'});

      expect(controller.state.sandboxReady, isTrue);
      expect(controller.state.messages.single.contents, '✓ Development environment ready');
    });
  });

  group('credentials_required', () {
    test('sets the flag and appends the server-provided error message', () async {
      await send({'e': 'credentials_required', 'message': 'Add your OpenRouter key to continue.'});

      expect(controller.state.credentialsRequired, isTrue);
      final msg = controller.state.messages.single;
      expect(msg.type, ChatMessageType.error);
      expect(msg.contents, 'Add your OpenRouter key to continue.');
    });

    test('falls back to a default message when none is provided', () async {
      await send({'e': 'credentials_required'});

      expect(controller.state.messages.single.contents, 'Provider keys are required to continue.');
    });
  });

  group('unknown event types', () {
    test('are ignored without crashing or appending a message', () async {
      await send({'e': 'some_future_event_type', 'message': 'should be ignored'});

      expect(controller.state.messages, isEmpty);
      expect(controller.state.stage, isNull);
      expect(controller.state.sandboxReady, isFalse);
      expect(controller.state.credentialsRequired, isFalse);
    });

    test('a malformed frame (no "e" field) does not crash the controller', () async {
      await send({'message': 'no discriminator'});

      // WsEvent.fromJson defaults to type "unknown", handled by `default`.
      expect(controller.state.messages, isEmpty);
    });
  });

  group('sendPrompt', () {
    test('appends the prompt as a text message and forwards it to the client', () async {
      controller.sendPrompt('build me a todo app');

      expect(controller.state.messages, hasLength(1));
      expect(controller.state.messages.single.type, ChatMessageType.text);
      expect(controller.state.messages.single.contents, 'build me a todo app');

      final sent = jsonDecode(channel.sentMessages.single) as Map<String, dynamic>;
      expect(sent['type'], 'start_agent');
      expect(sent['prompt'], 'build me a todo app');
    });
  });
}
