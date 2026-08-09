/// A server->client WebSocket event. All events share an `e` discriminator
/// field (apps/backend/internal/httpapi/websocket.go,
/// apps/frontend/app/hooks/useAgentSession.ts:110-337 is the canonical list
/// of shapes this mirrors).
class WsEvent {
  const WsEvent(this.type, this.data);

  final String type;
  final Map<String, dynamic> data;

  factory WsEvent.fromJson(Map<String, dynamic> json) {
    final type = json['e'] as String? ?? 'unknown';
    return WsEvent(type, json);
  }

  String? get message => data['message'] as String?;
  String? get stage => data['stage'] as String?;
  num? get progress => data['progress'] as num?;
  String? get tool => data['tool'] as String?;
  String? get filepath => data['filepath'] as String?;
}

enum WsConnectionState { disconnected, connecting, connected, error }
