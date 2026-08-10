import 'package:dio/dio.dart';

import '../core/config.dart';
import 'secure_storage.dart';

/// Thin wrapper around apps/backend's REST API
/// (apps/backend/internal/httpapi/server.go routes()). Attaches the stored
/// Bearer token to every request and calls [onUnauthorized] on a 401 so the
/// app can force a logout, mirroring the web app's cookie-expiry handling.
class ApiClient {
  ApiClient(this._secureStorage) : dio = Dio(BaseOptions(baseUrl: AppConfig.apiBaseUrl)) {
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) async {
          final token = await _secureStorage.readAuthToken();
          if (token != null) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          handler.next(options);
        },
        onError: (error, handler) {
          if (error.response?.statusCode == 401) {
            onUnauthorized?.call();
          }
          handler.next(error);
        },
      ),
    );
  }

  final Dio dio;
  final SecureStorage _secureStorage;

  /// Set by the auth provider so a 401 anywhere in the app can trigger a
  /// clean sign-out, not just the request that surfaced it.
  void Function()? onUnauthorized;

  static String errorMessage(Object error) {
    if (error is DioException) {
      final data = error.response?.data;
      if (data is Map && data['error'] is String) {
        return data['error'] as String;
      }
      switch (error.type) {
        case DioExceptionType.connectionError:
        case DioExceptionType.connectionTimeout:
          return "Couldn't reach the server. Check your connection and try again.";
        case DioExceptionType.receiveTimeout:
        case DioExceptionType.sendTimeout:
          return 'The server took too long to respond. Please try again.';
        default:
          return error.message ?? 'Network request failed';
      }
    }
    return error.toString();
  }
}
