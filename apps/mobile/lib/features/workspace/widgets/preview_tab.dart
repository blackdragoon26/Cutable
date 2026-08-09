import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:webview_flutter/webview_flutter.dart';

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
            if (_pageLoading) const LinearProgressIndicator(),
            Positioned(
              right: 12,
              bottom: 12,
              child: FloatingActionButton.small(
                onPressed: () => _controller.reload(),
                child: const Icon(Icons.refresh),
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
