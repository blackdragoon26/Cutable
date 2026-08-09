import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// Keychain (iOS) / EncryptedSharedPreferences (Android) storage for the
/// auth JWT and BYO provider keys — the mobile analog of the web app's
/// HttpOnly cookie and sessionStorage-based provider-credentials.ts.
class SecureStorage {
  SecureStorage() : _storage = const FlutterSecureStorage();

  final FlutterSecureStorage _storage;

  static const _authTokenKey = 'cutable.auth_token';
  static const _openRouterKeyKey = 'cutable.openrouter_key';
  static const _e2bKeyKey = 'cutable.e2b_key';

  Future<String?> readAuthToken() => _storage.read(key: _authTokenKey);
  Future<void> writeAuthToken(String token) => _storage.write(key: _authTokenKey, value: token);
  Future<void> deleteAuthToken() => _storage.delete(key: _authTokenKey);

  Future<String?> readOpenRouterKey() => _storage.read(key: _openRouterKeyKey);
  Future<String?> readE2BKey() => _storage.read(key: _e2bKeyKey);

  Future<void> writeProviderKeys({String? openRouterKey, String? e2bKey}) async {
    if (openRouterKey != null) {
      await _storage.write(key: _openRouterKeyKey, value: openRouterKey);
    }
    if (e2bKey != null) {
      await _storage.write(key: _e2bKeyKey, value: e2bKey);
    }
  }

  Future<void> forgetProviderKeys() async {
    await _storage.delete(key: _openRouterKeyKey);
    await _storage.delete(key: _e2bKeyKey);
  }
}
