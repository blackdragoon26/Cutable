// Unit tests for WsClient (lib/data/ws_client.dart).
//
// WsClient normally opens a real socket via IOWebSocketChannel.connect
// inside _open(). To make it testable without a network call, WsClient now
// accepts an injectable `channelFactory` (see lib/data/ws_client.dart) that
// defaults to the real implementation but lets tests supply a
// FakeWebSocketChannel instead. That gives us real coverage of:
//   - the exponential-backoff reconnect state machine (via fake_async, no
//     real waiting),
//   - the no-reconnect close codes (4000/4001/4003),
//   - outgoing message shapes (startAgent），
//   - inbound JSON frame parsing into WsEvent, including malformed frames.
//
// What's NOT covered here: the real IOWebSocketChannel.connect/network path
// itself (TLS handshake, real header delivery, actual socket errors) — that
// would require an integration test against a live/mock server, which is
// out of scope for these unit tests.
import 'dart:async';
import 'dart:convert';

import 'package:cutable_mobile/data/models/ws_event.dart';
import 'package:cutable_mobile/data/secure_storage.dart';
import 'package:cutable_mobile/data/ws_client.dart';
import 'package:fake_async/fake_async.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

import 'support/fake_web_socket_channel.dart';

class MockSecureStorage extends Mock implements SecureStorage {}

void main() {
  late MockSecureStorage secureStorage;
  late List<FakeWebSocketChannel> channels;

  WebSocketChannelFactory fakeFactory() {
    return (Uri uri, {Map<String, String>? headers}) {
      final channel = FakeWebSocketChannel(headers: headers);
      channels.add(channel);
      return channel;
    };
  }

  setUp(() {
    secureStorage = MockSecureStorage();
    channels = [];
    when(() => secureStorage.readAuthToken()).thenAnswer((_) async => null);
  });

  group('reconnectDelayMs', () {
    test('doubles each attempt starting at 1000ms, capped at 30000ms', () {
      expect(WsClient.reconnectDelayMs(1), 1000);
      expect(WsClient.reconnectDelayMs(2), 2000);
      expect(WsClient.reconnectDelayMs(3), 4000);
      expect(WsClient.reconnectDelayMs(4), 8000);
      expect(WsClient.reconnectDelayMs(5), 16000);
      // Uncapped this would be 32000; formula caps at 30000.
      expect(WsClient.reconnectDelayMs(6), 30000);
      expect(WsClient.reconnectDelayMs(10), 30000);
    });
  });

  group('connect', () {
    test('opens with no Authorization header when no token is stored', () async {
      final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());
      final states = <WsConnectionState>[];
      client.connectionState.listen(states.add);

      await client.connect();
      // StreamController listener delivery is scheduled as a microtask, so
      // flush the queue once more to make sure the last `add()` (connected)
      // has actually reached the listener before asserting.
      await Future<void>.delayed(Duration.zero);

      expect(channels, hasLength(1));
      expect(channels.single.headers, isNull);
      expect(states, [WsConnectionState.connecting, WsConnectionState.connected]);

      await client.dispose();
    });

    test('attaches a Bearer header when a token is stored', () async {
      when(() => secureStorage.readAuthToken()).thenAnswer((_) async => 'tok-123');
      final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());

      await client.connect();

      expect(channels.single.headers, {'Authorization': 'Bearer tok-123'});

      await client.dispose();
    });
  });

  group('startAgent', () {
    test('sends a start_agent message with no credentials key when no keys given', () async {
      final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());
      await client.connect();

      client.startAgent(prompt: 'build a landing page');

      final sent = jsonDecode(channels.single.sentMessages.single) as Map<String, dynamic>;
      expect(sent, {'type': 'start_agent', 'prompt': 'build a landing page'});
      expect(sent.containsKey('credentials'), isFalse);

      await client.dispose();
    });

    test('includes both credential keys when both are provided', () async {
      final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());
      await client.connect();

      client.startAgent(prompt: 'x', openRouterApiKey: 'or-key', e2bApiKey: 'e2b-key');

      final sent = jsonDecode(channels.single.sentMessages.single) as Map<String, dynamic>;
      expect(sent['credentials'], {'openRouterApiKey': 'or-key', 'e2bApiKey': 'e2b-key'});

      await client.dispose();
    });

    test('includes only the provided credential key', () async {
      final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());
      await client.connect();

      client.startAgent(prompt: 'x', openRouterApiKey: 'or-key');

      final sent = jsonDecode(channels.single.sentMessages.single) as Map<String, dynamic>;
      expect(sent['credentials'], {'openRouterApiKey': 'or-key'});

      await client.dispose();
    });
  });

  group('incoming frame parsing', () {
    test('parses a well-formed JSON frame into a WsEvent', () async {
      final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());
      final events = <String>[];
      client.events.listen((e) => events.add(e.type));
      await client.connect();

      channels.single.addIncoming(jsonEncode({'e': 'tool_started', 'tool': 'search'}));
      await Future<void>.delayed(Duration.zero);

      expect(events, ['tool_started']);

      await client.dispose();
    });

    test('silently ignores malformed JSON frames instead of crashing', () async {
      final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());
      final events = <String>[];
      final connectionStates = <WsConnectionState>[];
      client.events.listen((e) => events.add(e.type));
      client.connectionState.listen(connectionStates.add);
      await client.connect();
      await Future<void>.delayed(Duration.zero);
      connectionStates.clear();

      channels.single.addIncoming('{not valid json');
      await Future<void>.delayed(Duration.zero);

      expect(events, isEmpty);
      // The connection itself must not be torn down by a bad frame.
      expect(connectionStates, isEmpty);

      await client.dispose();
    });
  });

  group('reconnect behavior', () {
    for (final code in [4000, 4001, 4003]) {
      test('does not reconnect on close code $code', () {
        fakeAsync((async) {
          channels = [];
          final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());
          unawaited(client.connect());
          async.flushMicrotasks();
          expect(channels, hasLength(1));

          unawaited(channels.single.simulateServerClose(code));
          async.flushMicrotasks();
          async.elapse(const Duration(seconds: 40));
          async.flushMicrotasks();

          expect(channels, hasLength(1), reason: 'close code $code must not trigger a reconnect');
          expect(client.reconnectAttempts, 0);
        });
      });
    }

    test('reconnects with exponential backoff while reconnection keeps failing', () {
      // A successful _open() resets the attempt counter, so to observe the
      // backoff actually ramping up (1s, 2s, 4s, ...) the reconnect attempts
      // themselves must keep failing, exactly like a server that's still
      // down. The first connect succeeds (establishing the initial
      // channel); every channelFactory call after that throws, which is
      // exactly the path WsClient._open's catch block handles.
      fakeAsync((async) {
        var callCount = 0;
        final client = WsClient(
          secureStorage,
          projectId: 'proj-1',
          channelFactory: (uri, {headers}) {
            callCount += 1;
            if (callCount == 1) {
              final channel = FakeWebSocketChannel(headers: headers);
              channels.add(channel);
              return channel;
            }
            throw Exception('connection refused');
          },
        );
        unawaited(client.connect());
        async.flushMicrotasks();
        expect(channels, hasLength(1));

        // Initial close triggers reconnect attempt 1 (1000ms).
        unawaited(channels.single.simulateServerClose(1006));
        async.flushMicrotasks();
        expect(client.reconnectAttempts, 1);

        async.elapse(const Duration(milliseconds: 999));
        expect(client.reconnectAttempts, 1, reason: 'the 1000ms backoff has not elapsed yet');
        async.elapse(const Duration(milliseconds: 1));
        async.flushMicrotasks();
        // callCount == 2 -> channelFactory threw -> attempt 2 scheduled (2000ms).
        expect(client.reconnectAttempts, 2);

        async.elapse(const Duration(milliseconds: 1999));
        expect(client.reconnectAttempts, 2, reason: 'the 2000ms backoff has not elapsed yet');
        async.elapse(const Duration(milliseconds: 1));
        async.flushMicrotasks();
        // callCount == 3 -> threw again -> attempt 3 scheduled (4000ms).
        expect(client.reconnectAttempts, 3);

        // No real channel was ever re-established after the first one.
        expect(channels, hasLength(1));
      });
    });

    test('gives up after the max of 5 reconnect attempts', () {
      fakeAsync((async) {
        var callCount = 0;
        final client = WsClient(
          secureStorage,
          projectId: 'proj-1',
          channelFactory: (uri, {headers}) {
            callCount += 1;
            if (callCount == 1) {
              final channel = FakeWebSocketChannel(headers: headers);
              channels.add(channel);
              return channel;
            }
            throw Exception('connection refused');
          },
        );
        unawaited(client.connect());
        async.flushMicrotasks();
        expect(channels, hasLength(1));

        unawaited(channels.single.simulateServerClose(1006));
        async.flushMicrotasks();

        // Each failed reconnect attempt schedules the next with a growing
        // delay (1s, 2s, 4s, 8s, 16s) until the 5-attempt cap is hit.
        for (var attempt = 1; attempt <= 5; attempt++) {
          expect(client.reconnectAttempts, attempt);
          async.elapse(Duration(milliseconds: WsClient.reconnectDelayMs(attempt)));
          async.flushMicrotasks();
        }

        // The 5th failed attempt must not schedule a 6th.
        expect(client.reconnectAttempts, 5);
        expect(callCount, 6, reason: 'initial connect + 5 failed reconnect attempts');

        async.elapse(const Duration(seconds: 40));
        async.flushMicrotasks();
        expect(client.reconnectAttempts, 5, reason: 'no further attempts are scheduled past the cap');
        expect(callCount, 6);
      });
    });

    test('a manual disconnect does not trigger a reconnect', () {
      fakeAsync((async) {
        final client = WsClient(secureStorage, projectId: 'proj-1', channelFactory: fakeFactory());
        unawaited(client.connect());
        async.flushMicrotasks();

        unawaited(client.disconnect());
        async.flushMicrotasks();
        async.elapse(const Duration(seconds: 40));
        async.flushMicrotasks();

        expect(channels, hasLength(1));
        expect(client.reconnectAttempts, 0);
      });
    });
  });
}
