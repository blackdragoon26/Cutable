import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../data/models/file_node.dart';
import '../providers/agent_session_provider.dart';
import '../providers/workspace_data_providers.dart';

/// Read-only file browser (GET /api/projects/{id}/files,
/// GET /api/projects/{id}/files/{path...}) — mirrors apps/frontend's
/// FileExplorer + Monaco viewer, minus in-app editing per scope.
class FilesTab extends ConsumerWidget {
  const FilesTab({super.key, required this.projectId});

  final String projectId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.listen(agentSessionProvider(projectId), (previous, next) {
      if (previous?.messages.length != next.messages.length) {
        ref.invalidate(fileTreeProvider(projectId));
      }
    });

    final treeAsync = ref.watch(fileTreeProvider(projectId));
    return treeAsync.when(
      data: (topLevelNodes) {
        if (topLevelNodes.isEmpty) {
          return const Center(child: Text('No files yet — files appear here as the agent builds.'));
        }
        return RefreshIndicator(
          onRefresh: () => ref.refresh(fileTreeProvider(projectId).future),
          child: ListView(
            padding: const EdgeInsets.all(8),
            children: topLevelNodes.map((node) => _FileTreeNode(projectId: projectId, node: node, depth: 0)).toList(),
          ),
        );
      },
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, _) => Center(child: Text('Could not load files: $error')),
    );
  }
}

class _FileTreeNode extends ConsumerStatefulWidget {
  const _FileTreeNode({required this.projectId, required this.node, required this.depth});

  final String projectId;
  final FileNode node;
  final int depth;

  @override
  ConsumerState<_FileTreeNode> createState() => _FileTreeNodeState();
}

class _FileTreeNodeState extends ConsumerState<_FileTreeNode> {
  bool _expanded = false;

  @override
  Widget build(BuildContext context) {
    final node = widget.node;
    if (node.isDirectory) {
      return Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          ListTile(
            contentPadding: EdgeInsets.only(left: 12.0 * widget.depth),
            dense: true,
            leading: Icon(_expanded ? Icons.folder_open : Icons.folder),
            title: Text(node.name),
            onTap: () => setState(() => _expanded = !_expanded),
          ),
          if (_expanded)
            ...node.children.map(
              (child) => _FileTreeNode(projectId: widget.projectId, node: child, depth: widget.depth + 1),
            ),
        ],
      );
    }
    return ListTile(
      contentPadding: EdgeInsets.only(left: 12.0 * widget.depth),
      dense: true,
      leading: const Icon(Icons.insert_drive_file_outlined),
      title: Text(node.name),
      onTap: () => Navigator.of(context).push(
        MaterialPageRoute(
          builder: (context) => _FileViewerScreen(projectId: widget.projectId, path: node.path),
        ),
      ),
    );
  }
}

class _FileViewerScreen extends ConsumerWidget {
  const _FileViewerScreen({required this.projectId, required this.path});

  final String projectId;
  final String path;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final contentAsync = ref.watch(fileContentProvider((projectId: projectId, path: path)));
    return Scaffold(
      appBar: AppBar(title: Text(path.split('/').last)),
      body: contentAsync.when(
        data: (content) => SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: SelectableText(
              content,
              style: const TextStyle(fontFamily: 'monospace', fontSize: 13),
            ),
          ),
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(child: Text('Could not load file: $error')),
      ),
    );
  }
}
