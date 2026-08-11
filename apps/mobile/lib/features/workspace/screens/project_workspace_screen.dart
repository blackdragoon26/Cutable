import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../data/models/ws_event.dart';
import '../../auth/providers/auth_providers.dart';
import '../providers/agent_session_provider.dart';
import '../providers/workspace_data_providers.dart';
import '../widgets/chat_tab.dart';
import '../widgets/files_tab.dart';
import '../widgets/preview_tab.dart';

/// Mirrors apps/frontend/app/projects/[id]/page.tsx's 3-pane workspace,
/// laid out as bottom tabs for mobile ergonomics. Connects to
/// GET /ws?projectId=... on entry via agentSessionProvider and
/// auto-bootstraps the agent run with the project's initialPrompt on first
/// load, matching the web app's behavior at projects/[id]/page.tsx:155-184.
class ProjectWorkspaceScreen extends ConsumerStatefulWidget {
  const ProjectWorkspaceScreen({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<ProjectWorkspaceScreen> createState() => _ProjectWorkspaceScreenState();
}

class _ProjectWorkspaceScreenState extends ConsumerState<ProjectWorkspaceScreen> {
  bool _bootstrapped = false;

  @override
  Widget build(BuildContext context) {
    final projectAsync = ref.watch(projectDetailProvider(widget.projectId));
    final session = ref.watch(agentSessionProvider(widget.projectId));

    // Auto-start the agent with the project's initial prompt once the
    // socket is connected and the project has no build output yet.
    if (!_bootstrapped &&
        session.connectionState == WsConnectionState.connected &&
        projectAsync.hasValue) {
      _bootstrapped = true;
      final project = projectAsync.requireValue;
      if (project.sandboxId == null) {
        Future.microtask(() async {
          final storage = ref.read(secureStorageProvider);
          final openRouterApiKey = await storage.readOpenRouterKey();
          final e2bApiKey = await storage.readE2BKey();
          if (!mounted) return;
          ref.read(agentSessionProvider(widget.projectId).notifier).sendPrompt(
                project.initialPrompt,
                openRouterApiKey: openRouterApiKey,
                e2bApiKey: e2bApiKey,
              );
        });
      }
    }

    return DefaultTabController(
      length: 3,
      child: Scaffold(
        appBar: AppBar(
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            tooltip: 'Back to dashboard',
            // New Project navigates here with context.go (replacing history,
            // since the prompt screen shouldn't stay in the back stack once
            // building starts), so there's often nothing to pop — fall back
            // to explicitly going home rather than leaving the user stuck
            // with no way back.
            onPressed: () => context.canPop() ? context.pop() : context.go('/dashboard'),
          ),
          title: Text(projectAsync.valueOrNull?.title ?? 'Project'),
          bottom: const TabBar(tabs: [
            Tab(icon: Icon(Icons.chat_bubble_outline), text: 'Chat'),
            Tab(icon: Icon(Icons.folder_outlined), text: 'Files'),
            Tab(icon: Icon(Icons.visibility_outlined), text: 'Preview'),
          ]),
        ),
        body: TabBarView(
          children: [
            ChatTab(projectId: widget.projectId),
            FilesTab(projectId: widget.projectId),
            PreviewTab(projectId: widget.projectId),
          ],
        ),
      ),
    );
  }
}
