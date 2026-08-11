/// Mirrors apps/backend/internal/store.Conversation (store.go:68-77).
class Conversation {
  const Conversation({
    required this.from,
    required this.type,
    required this.contents,
    required this.createdAt,
  });

  final String from; // "USER" | "AGENT"
  final String type; // "TOOL_CALL" | "TEXT_MESSAGE" | "ERROR_MESSAGE"
  final String contents;
  final DateTime createdAt;

  factory Conversation.fromJson(Map<String, dynamic> json) => Conversation(
        from: json['from'] as String,
        type: json['type'] as String,
        contents: json['contents'] as String,
        createdAt: DateTime.parse(json['createdAt'] as String),
      );
}
