/// Mirrors apps/backend/internal/store.FileNode (store.go:88-93).
class FileNode {
  const FileNode({
    required this.name,
    required this.path,
    required this.type,
    this.children = const [],
  });

  final String name;
  final String path;
  final String type; // "file" | "directory"
  final List<FileNode> children;

  bool get isDirectory => type == 'directory';

  factory FileNode.fromJson(Map<String, dynamic> json) => FileNode(
        name: json['name'] as String,
        path: json['path'] as String,
        type: json['type'] as String,
        children: (json['children'] as List<dynamic>? ?? [])
            .map((item) => FileNode.fromJson(item as Map<String, dynamic>))
            .toList(),
      );
}
