import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:web_socket_channel/io.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

import '../core/config.dart';
import 'models/ws_event.dart';
import 'secure_storage.dart';

const _pingInterval = Duration(seconds: 30);
const _maxReconnectAttempts = 5;

/// Close codes the backend sends for conditions that should not trigger a
/// reconnect (invalid project / auth required / access denied) — mirrors
/// apps/frontend/app/hooks/useWebSocket.ts:170-181.
const _noReconnectCloseCodes = {4000, 4001, 4003};

/// Mirrors apps/frontend/app/hooks/useWebSocket.ts. Connects to
/// GET `/ws?projectId=<id>` (apps/backend/internal/httpapi/websocket.go),
/// authenticating via a Bearer header at connect time with a ?token= query
/// fallback (apps/backend/internal/httpapi/server.go bearerToken()).
class WsClient {
  WsClient(this._secureStorage, {required this.projectId});

  final SecureStorage _secureStorage;
  final String projectId;

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _subscription;
  Timer? _pingTimer;
  int _reconnectAttempts = 0;
  bool _manuallyClosed = false;

  final _eventController = StreamController<WsEvent>.broadcast();
  final _stateController = StreamController<WsConnectionState>.broadcast();

  Stream<WsEvent> get events => _eventController.stream;
  Stream<WsConnectionState> get connectionState => _stateController.stream;

  Future<void> connect() async {
    _manuallyClosed = false;
    await _open();
  }

  Future<void> _open() async {
    _stateController.add(WsConnectionState.connecting);
    final token = await _secureStorage.readAuthToken();
    final uri = Uri.parse('${AppConfig.wsBaseUrl}/ws').replace(queryParameters: {
      'projectId': projectId,
      if (token != null) 'token': token,
    });
    try {
      // IOWebSocketChannel lets us set headers on the upgrade request on
      // iOS/Android; the ?token= query param above is kept as a fallback in
      // case header injection proves unreliable on a given platform.
      _channel = IOWebSocketChannel.connect(
        uri,
        headers: token != null ? {'Authorization': 'Bearer $token'} : null,
        pingInterval: null,
      );
      _subscription = _channel!.stream.listen(
        _handleRaw,
        onDone: _handleDone,
        onError: (_) => _handleDone(),
        cancelOnError: true,
      );
      _stateController.add(WsConnectionState.connected);
      _reconnectAttempts = 0;
      _startPing();
    } catch (_) {
      _stateController.add(WsConnectionState.error);
      _scheduleReconnect();
    }
  }

  void _handleRaw(dynamic raw) {
    try {
      final json = jsonDecode(raw as String) as Map<String, dynamic>;
      _eventController.add(WsEvent.fromJson(json));
    } catch (_) {
      // Ignore malformed frames rather than tearing down the connection.
    }
  }

  void _startPing() {
    _pingTimer?.cancel();
    _pingTimer = Timer.periodic(_pingInterval, (_) {
      sendStartAgentRaw({'type': 'ping'});
    });
  }

  void _handleDone() {
    _pingTimer?.cancel();
    final code = _channel?.closeCode;
    _stateController.add(WsConnectionState.disconnected);
    if (_manuallyClosed) return;
    if (code != null && _noReconnectCloseCodes.contains(code)) return;
    _scheduleReconnect();
  }

  void _scheduleReconnect() {
    if (_reconnectAttempts >= _maxReconnectAttempts) return;
    _reconnectAttempts += 1;
    final delayMs = min(1000 * pow(2, _reconnectAttempts - 1).toInt(), 30000);
    Timer(Duration(milliseconds: delayMs), () {
      if (!_manuallyClosed) _open();
    });
  }

  void sendStartAgentRaw(Map<String, dynamic> message) {
    final channel = _channel;
    if (channel == null) return;
    channel.sink.add(jsonEncode(message));
  }

  void startAgent({required String prompt, String? openRouterApiKey, String? e2bApiKey}) {
    sendStartAgentRaw({
      'type': 'start_agent',
      'prompt': prompt,
      if (openRouterApiKey != null || e2bApiKey != null)
        'credentials': {
          if (openRouterApiKey != null) 'openRouterApiKey': openRouterApiKey,
          if (e2bApiKey != null) 'e2bApiKey': e2bApiKey,
        },
    });
  }

  Future<void> disconnect() async {
    _manuallyClosed = true;
    _pingTimer?.cancel();
    await _subscription?.cancel();
    await _channel?.sink.close();
  }

  Future<void> dispose() async {
    await disconnect();
    await _eventController.close();
    await _stateController.close();
  }
}
