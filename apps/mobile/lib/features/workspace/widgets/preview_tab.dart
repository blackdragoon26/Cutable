import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../../data/api_client.dart';
import '../../auth/providers/auth_providers.dart';
import '../providers/agent_session_provider.dart';
import '../providers/workspace_data_providers.dart';

/// Live preview WebView loading the sandbox's previewUrl — mirrors the
/// iframe in apps/frontend/app/projects/[id]/page.tsx's EditorSection.
class PreviewTab extends ConsumerStatefulWidget {
  const PreviewTab({super.key, required this.projectId});

  final String projectId;

  @override
  ConsumerState<PreviewTab> createState() => _PreviewTabState();
}

class _PreviewTabState extends ConsumerState<PreviewTab> {
  late final WebViewController _controller;
  bool _pageLoading = false;
  bool _restarting = false;
  String? _loadedUrl;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setNavigationDelegate(
        NavigationDelegate(
          onPageStarted: (_) => setState(() => _pageLoading = true),
          onPageFinished: (_) => setState(() => _pageLoading = false),
        ),
      );
  }

  /// The E2B sandbox that served [previewUrl] expires after a period of
  /// inactivity (independent of the project itself, which is persisted
  /// forever) — reopening an old project loaded a dead preview URL with no
  /// way to bring it back. This calls the same reconnect-or-recreate
  /// endpoint the web app's "Restart preview" button uses
  /// (apps/backend/internal/httpapi/sandbox.go createSandbox), which
  /// replays the project's persisted files onto a fresh sandbox if the old
  /// one is gone.
  Future<void> _restartPreview() async {
    setState(() => _restarting = true);
    try {
      final e2bApiKey = await ref.read(secureStorageProvider).readE2BKey();
      await ref.read(projectRepositoryProvider).restartSandbox(widget.projectId, e2bApiKey: e2bApiKey);
      ref.invalidate(sandboxInfoProvider(widget.projectId));
    } catch (error) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(ApiClient.errorMessage(error))),
        );
      }
    } finally {
      if (mounted) setState(() => _restarting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    ref.listen(agentSessionProvider(widget.projectId).select((state) => state.sandboxReady), (previous, ready) {
      if (ready && previous != true) {
        ref.invalidate(sandboxInfoProvider(widget.projectId));
      }
    });

    final sandboxAsync = ref.watch(sandboxInfoProvider(widget.projectId));
    return sandboxAsync.when(
      data: (sandbox) {
        final previewUrl = sandbox.previewUrl;
        if (previewUrl == null || previewUrl.isEmpty) {
          return const Center(
            child: Padding(
              padding: EdgeInsets.all(24),
              child: Text('Preview will appear here once the sandbox is ready.', textAlign: TextAlign.center),
            ),
          );
        }
        if (_loadedUrl != previewUrl) {
          _loadedUrl = previewUrl;
          _controller.loadRequest(Uri.parse(previewUrl));
        }
        return Stack(
          children: [
            WebViewWidget(controller: _controller),
            if (_pageLoading || _restarting) const LinearProgressIndicator(),
            Positioned(
              right: 12,
              bottom: 12,
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  FloatingActionButton.small(
                    heroTag: 'restart-preview',
                    tooltip: 'Restart preview (if the sandbox expired)',
                    onPressed: _restarting ? null : _restartPreview,
                    child: const Icon(Icons.restart_alt),
                  ),
                  const SizedBox(height: 8),
                  FloatingActionButton.small(
                    heroTag: 'refresh-preview',
                    tooltip: 'Reload page',
                    onPressed: () => _controller.reload(),
                    child: const Icon(Icons.refresh),
                  ),
                ],
              ),
            ),
          ],
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(child: Text('Could not load preview: $error')),
    );
  }
}
