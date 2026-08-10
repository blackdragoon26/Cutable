import 'dart:async';

import 'package:stream_channel/stream_channel.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

/// In-memory stand-in for [WebSocketChannel] used to unit test [WsClient]
/// (and anything built on top of it, e.g. AgentSessionController) without
/// opening a real socket.
///
/// Tests drive the "server" side via [addIncoming] (push a frame to the
/// client) and [simulateServerClose] (close the connection with a given
/// close code, mirroring what the backend sends for e.g. auth failures).
/// Frames the client sends are recorded in [sentMessages].
class FakeWebSocketChannel with StreamChannelMixin implements WebSocketChannel {
  FakeWebSocketChannel({this.headers}) {
    sink = FakeWebSocketSink(this);
  }

  /// Headers the client connected with, captured for assertions.
  final Map<String, String>? headers;

  final StreamController<dynamic> _incoming = StreamController<dynamic>.broadcast();
  int? _closeCode;
  String? _closeReason;

  @override
  late final FakeWebSocketSink sink;

  @override
  Stream get stream => _incoming.stream;

  @override
  int? get closeCode => _closeCode;

  @override
  String? get closeReason => _closeReason;

  @override
  String? get protocol => null;

  @override
  Future<void> get ready => Future.value();

  List<String> get sentMessages => sink.sent.cast<String>();

  /// Pushes a server->client frame (raw text, as [WsClient] expects).
  void addIncoming(String raw) => _incoming.add(raw);

  /// Simulates the server closing the connection, optionally with a close
  /// code (e.g. 4000/4001/4003 for the "don't reconnect" cases).
  Future<void> simulateServerClose([int? code, String? reason]) async {
    _closeCode = code;
    _closeReason = reason;
    if (!_incoming.isClosed) await _incoming.close();
  }
}

class FakeWebSocketSink implements WebSocketSink {
  FakeWebSocketSink(this._channel);

  final FakeWebSocketChannel _channel;
  final List<dynamic> sent = [];
  final Completer<void> _doneCompleter = Completer<void>();

  @override
  void add(dynamic data) => sent.add(data);

  @override
  void addError(Object error, [StackTrace? stackTrace]) {}

  @override
  Future addStream(Stream stream) => stream.forEach(add);

  @override
  Future close([int? closeCode, String? closeReason]) async {
    _channel._closeCode = closeCode;
    _channel._closeReason = closeReason;
    if (!_channel._incoming.isClosed) await _channel._incoming.close();
    if (!_doneCompleter.isCompleted) _doneCompleter.complete();
    return Future.value();
  }

  @override
  Future get done => _doneCompleter.future;
}
