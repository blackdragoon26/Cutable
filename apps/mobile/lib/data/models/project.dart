/// Mirrors apps/backend/internal/store.ProjectAttachment /
/// ProjectAttachmentInput (Name/Kind/MimeType/Content/Size,
/// apps/backend/internal/store/store.go:49-54) and
/// apps/frontend/app/lib/api.ts ProjectAttachmentInput.
class ProjectAttachment {
  const ProjectAttachment({
    required this.name,
    required this.kind,
    required this.mimeType,
    required this.content,
    required this.size,
  });

  final String name;
  final String kind; // "text" | "image"
  final String mimeType;
  final String content;
  final int size;

  factory ProjectAttachment.fromJson(Map<String, dynamic> json) => ProjectAttachment(
        name: json['name'] as String,
        kind: json['kind'] as String,
        mimeType: json['mimeType'] as String,
        content: json['content'] as String? ?? '',
        size: json['size'] as int? ?? 0,
      );

  Map<String, dynamic> toJson() => {
        'name': name,
        'kind': kind,
        'mimeType': mimeType,
        'content': content,
      };
}

class Project {
  const Project({
    required this.id,
    required this.title,
    required this.initialPrompt,
    required this.userId,
    this.sandboxId,
    this.sandboxUrl,
    this.attachments = const [],
    required this.createdAt,
    required this.updatedAt,
  });

  final String id;
  final String title;
  final String initialPrompt;
  final String userId;
  final String? sandboxId;
  final String? sandboxUrl;
  final List<ProjectAttachment> attachments;
  final DateTime createdAt;
  final DateTime updatedAt;

  factory Project.fromJson(Map<String, dynamic> json) => Project(
        id: json['id'] as String,
        title: json['title'] as String,
        initialPrompt: json['initialPrompt'] as String,
        userId: json['userId'] as String,
        sandboxId: json['sandboxId'] as String?,
        sandboxUrl: json['sandboxUrl'] as String?,
        attachments: (json['attachments'] as List<dynamic>? ?? [])
            .map((item) => ProjectAttachment.fromJson(item as Map<String, dynamic>))
            .toList(),
        createdAt: DateTime.parse(json['createdAt'] as String),
        updatedAt: DateTime.parse(json['updatedAt'] as String),
      );
}
