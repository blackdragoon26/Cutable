// Unit tests for AuthRepository (lib/data/auth_repository.dart).
//
// AuthRepository's job is really "call the right endpoint, then store/read
// the token correctly" rather than actual HTTP behavior, so we mock
// ApiClient (stubbing its `dio` getter) and SecureStorage with mocktail
// rather than exercising a real Dio HTTP stack.
import 'package:cutable_mobile/data/api_client.dart';
import 'package:cutable_mobile/data/auth_repository.dart';
import 'package:cutable_mobile/data/secure_storage.dart';
import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:mocktail/mocktail.dart';

class MockApiClient extends Mock implements ApiClient {}

class MockDio extends Mock implements Dio {}

class MockSecureStorage extends Mock implements SecureStorage {}

Response<T> _response<T>(String path, T data, {int statusCode = 200}) {
  return Response<T>(
    requestOptions: RequestOptions(path: path),
    data: data,
    statusCode: statusCode,
  );
}

void main() {
  late MockApiClient apiClient;
  late MockDio dio;
  late MockSecureStorage secureStorage;
  late AuthRepository repository;

  setUpAll(() {
    registerFallbackValue(RequestOptions(path: '/'));
  });

  setUp(() {
    apiClient = MockApiClient();
    dio = MockDio();
    secureStorage = MockSecureStorage();
    when(() => apiClient.dio).thenReturn(dio);
    repository = AuthRepository(apiClient, secureStorage);
  });

  group('login', () {
    test('stores the returned token and then fetches the current user', () async {
      when(() => dio.post('/api/auth/login', data: any(named: 'data'))).thenAnswer(
        (_) async => _response('/api/auth/login', {'token': 'jwt-abc'}),
      );
      when(() => secureStorage.writeAuthToken(any())).thenAnswer((_) async {});
      when(() => dio.get('/api/auth/me')).thenAnswer(
        (_) async => _response('/api/auth/me', {
          'user': {'id': 'u1', 'name': 'Ada', 'email': 'ada@example.com'},
        }),
      );

      final user = await repository.login(email: 'ada@example.com', password: 'hunter2');

      expect(user.id, 'u1');
      expect(user.email, 'ada@example.com');
      final captured = verify(() => dio.post('/api/auth/login', data: captureAny(named: 'data'))).captured;
      expect(captured.single, {'email': 'ada@example.com', 'password': 'hunter2'});
      verify(() => secureStorage.writeAuthToken('jwt-abc')).called(1);
      verify(() => dio.get('/api/auth/me')).called(1);
    });

    test('does not store a token or fetch the user when the login call fails', () async {
      when(() => dio.post('/api/auth/login', data: any(named: 'data')))
          .thenThrow(DioException(requestOptions: RequestOptions(path: '/api/auth/login')));

      await expectLater(
        repository.login(email: 'ada@example.com', password: 'wrong'),
        throwsA(isA<DioException>()),
      );

      verifyNever(() => secureStorage.writeAuthToken(any()));
      verifyNever(() => dio.get('/api/auth/me'));
    });
  });

  group('register', () {
    test('stores the returned token and then fetches the current user', () async {
      when(() => dio.post('/api/auth/register', data: any(named: 'data'))).thenAnswer(
        (_) async => _response('/api/auth/register', {'token': 'jwt-new'}),
      );
      when(() => secureStorage.writeAuthToken(any())).thenAnswer((_) async {});
      when(() => dio.get('/api/auth/me')).thenAnswer(
        (_) async => _response('/api/auth/me', {
          'user': {'id': 'u2', 'name': 'Grace', 'email': 'grace@example.com'},
        }),
      );

      final user = await repository.register(name: 'Grace', email: 'grace@example.com', password: 'p4ssword');

      expect(user.name, 'Grace');
      verify(() => secureStorage.writeAuthToken('jwt-new')).called(1);
    });
  });

  group('logout', () {
    test('deletes the stored token even when the network logout call throws', () async {
      when(() => dio.post('/api/auth/logout')).thenThrow(
        DioException(requestOptions: RequestOptions(path: '/api/auth/logout')),
      );
      when(() => secureStorage.deleteAuthToken()).thenAnswer((_) async {});

      // Must not rethrow: logout() swallows the network failure so an
      // offline sign-out still clears the local session.
      await repository.logout();

      verify(() => secureStorage.deleteAuthToken()).called(1);
    });

    test('deletes the stored token when the network logout call succeeds', () async {
      when(() => dio.post('/api/auth/logout')).thenAnswer(
        (_) async => _response('/api/auth/logout', null),
      );
      when(() => secureStorage.deleteAuthToken()).thenAnswer((_) async {});

      await repository.logout();

      verify(() => secureStorage.deleteAuthToken()).called(1);
    });
  });

  group('hasStoredSession', () {
    test('is true when a non-empty token is stored', () async {
      when(() => secureStorage.readAuthToken()).thenAnswer((_) async => 'jwt-abc');
      expect(await repository.hasStoredSession(), isTrue);
    });

    test('is false when no token is stored', () async {
      when(() => secureStorage.readAuthToken()).thenAnswer((_) async => null);
      expect(await repository.hasStoredSession(), isFalse);
    });

    test('is false when the stored token is an empty string', () async {
      when(() => secureStorage.readAuthToken()).thenAnswer((_) async => '');
      expect(await repository.hasStoredSession(), isFalse);
    });
  });
}
