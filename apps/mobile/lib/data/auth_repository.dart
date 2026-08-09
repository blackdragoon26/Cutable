import 'package:flutter_web_auth_2/flutter_web_auth_2.dart';

import '../core/config.dart';
import 'api_client.dart';
import 'models/user.dart';
import 'secure_storage.dart';

/// Wraps apps/backend's /api/auth/* endpoints
/// (apps/backend/internal/httpapi/server.go:91-174,
/// google_oauth.go). Login/register now return `token` in the JSON body
/// (added alongside the existing cookie for native clients) — this
/// repository stores that token and attaches it via ApiClient's
/// interceptor on every subsequent request.
class AuthRepository {
  AuthRepository(this._api, this._secureStorage);

  final ApiClient _api;
  final SecureStorage _secureStorage;

  Future<User> register({required String name, required String email, required String password}) async {
    final response = await _api.dio.post('/api/auth/register', data: {
      'name': name,
      'email': email,
      'password': password,
    });
    await _secureStorage.writeAuthToken(response.data['token'] as String);
    return currentUser();
  }

  Future<User> login({required String email, required String password}) async {
    final response = await _api.dio.post('/api/auth/login', data: {
      'email': email,
      'password': password,
    });
    await _secureStorage.writeAuthToken(response.data['token'] as String);
    return currentUser();
  }

  Future<bool> googleAuthEnabled() async {
    final response = await _api.dio.get('/api/auth/config');
    return response.data['google']?['enabled'] == true;
  }

  /// Opens the backend's Google OAuth flow in an in-app browser
  /// (ASWebAuthenticationSession / Chrome Custom Tabs). On success the
  /// backend redirects to `cutable://auth-callback?token=<jwt>`
  /// (apps/backend/internal/httpapi/google_oauth.go googleCallback, mobile
  /// branch), which flutter_web_auth_2 captures without leaving the app.
  Future<User> signInWithGoogle() async {
    final authorizeUrl = '${AppConfig.apiBaseUrl}/api/auth/google?platform=mobile';
    final result = await FlutterWebAuth2.authenticate(
      url: authorizeUrl,
      callbackUrlScheme: AppConfig.authCallbackUrlScheme,
    );
    final uri = Uri.parse(result);
    final error = uri.queryParameters['error'];
    if (error != null) {
      throw Exception(error);
    }
    final token = uri.queryParameters['token'];
    if (token == null) {
      throw Exception('Google sign-in did not return a token');
    }
    await _secureStorage.writeAuthToken(token);
    return currentUser();
  }

  Future<User> currentUser() async {
    final response = await _api.dio.get('/api/auth/me');
    return User.fromJson(response.data['user'] as Map<String, dynamic>);
  }

  Future<bool> hasStoredSession() async {
    final token = await _secureStorage.readAuthToken();
    return token != null && token.isNotEmpty;
  }

  Future<void> logout() async {
    try {
      await _api.dio.post('/api/auth/logout');
    } catch (_) {
      // Stateless JWT: even if this call fails (e.g. offline), deleting the
      // local token is sufficient to end the session on-device.
    }
    await _secureStorage.deleteAuthToken();
  }
}
