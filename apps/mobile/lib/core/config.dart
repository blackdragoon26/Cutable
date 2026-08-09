/// Backend base URLs, provided at build/run time via --dart-define so the
/// same app binary can point at local, staging, or production APIs without
/// a code change:
///
///   flutter run \
///     --dart-define=API_BASE_URL=http://localhost:3010 \
///     --dart-define=WS_BASE_URL=ws://localhost:3010
///
/// Defaults match apps/frontend's NEXT_PUBLIC_API_URL / NEXT_PUBLIC_WS_URL
/// for local development. Use 10.0.2.2 instead of localhost when running
/// against the Android emulator.
class AppConfig {
  AppConfig._();

  static const String apiBaseUrl = String.fromEnvironment(
    'API_BASE_URL',
    defaultValue: 'http://localhost:3010',
  );

  static const String wsBaseUrl = String.fromEnvironment(
    'WS_BASE_URL',
    defaultValue: 'ws://localhost:3010',
  );

  /// Must match the custom URL scheme registered in Info.plist /
  /// AndroidManifest.xml and the backend's mobileAuthCallbackURL constant
  /// (apps/backend/internal/httpapi/google_oauth.go).
  static const String authCallbackUrlScheme = 'cutable';
}
