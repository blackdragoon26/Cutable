import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../core/theme/app_theme.dart';
import '../../auth/providers/auth_providers.dart';
import '../providers/agent_session_provider.dart';

class ChatTab extends ConsumerStatefulWidget {
  const ChatTab({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<ChatTab> createState() => _ChatTabState();
}

class _ChatTabState extends ConsumerState<ChatTab> {
  final _askController = TextEditingController();
  final _scrollController = ScrollController();

  @override
  void dispose() {
    _askController.dispose();
    _scrollController.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    final prompt = _askController.text.trim();
    if (prompt.isEmpty) return;
    _askController.clear();
    final storage = ref.read(secureStorageProvider);
    final openRouterApiKey = await storage.readOpenRouterKey();
    final e2bApiKey = await storage.readE2BKey();
    if (!mounted) return;
    ref.read(agentSessionProvider(widget.projectId).notifier).sendPrompt(
          prompt,
          openRouterApiKey: openRouterApiKey,
          e2bApiKey: e2bApiKey,
        );
  }

  @override
  Widget build(BuildContext context) {
    final session = ref.watch(agentSessionProvider(widget.projectId));

    ref.listen(agentSessionProvider(widget.projectId), (previous, next) {
      if (previous?.messages.length != next.messages.length && _scrollController.hasClients) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          _scrollController.animateTo(
            _scrollController.position.maxScrollExtent,
            duration: const Duration(milliseconds: 200),
            curve: Curves.easeOut,
          );
        });
      }
    });

    return Column(
      children: [
        if (session.stage != null)
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 4),
            child: Row(
              children: [
                Expanded(
                  child: LinearProgressIndicator(
                    value: (session.stage!.progress.toDouble()).clamp(0, 100) / 100,
                  ),
                ),
                const SizedBox(width: 12),
                Text(session.stage!.stage, style: Theme.of(context).textTheme.labelSmall),
              ],
            ),
          ),
        Expanded(
          child: session.messages.isEmpty
              ? Center(
                  child: Text(
                    'Watching for build updates…',
                    style: TextStyle(color: AppTheme.mutedOf(context)),
                  ),
                )
              : ListView.builder(
                  controller: _scrollController,
                  padding: const EdgeInsets.all(16),
                  itemCount: session.messages.length,
                  itemBuilder: (context, index) {
                    final message = session.messages[index];
                    final color = switch (message.type) {
                      ChatMessageType.error => Theme.of(context).colorScheme.error,
                      ChatMessageType.tool => AppTheme.mutedOf(context),
                      ChatMessageType.text => null,
                    };
                    return Padding(
                      padding: const EdgeInsets.symmetric(vertical: 6),
                      child: Text(message.contents, style: TextStyle(color: color)),
                    );
                  },
                ),
        ),
        SafeArea(
          top: false,
          child: Padding(
            padding: const EdgeInsets.all(12),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    controller: _askController,
                    decoration: const InputDecoration(hintText: 'Ask a follow-up…'),
                    onSubmitted: (_) => _send(),
                  ),
                ),
                const SizedBox(width: 8),
                IconButton.filled(onPressed: _send, icon: const Icon(Icons.send)),
              ],
            ),
          ),
        ),
      ],
    );
  }
}
