import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../data/api_client.dart';
import '../../../data/auth_repository.dart';
import '../../../data/project_repository.dart';
import '../../../data/secure_storage.dart';
import '../../../data/models/user.dart';

final secureStorageProvider = Provider<SecureStorage>((ref) => SecureStorage());

final apiClientProvider = Provider<ApiClient>((ref) => ApiClient(ref.watch(secureStorageProvider)));

final authRepositoryProvider = Provider<AuthRepository>(
  (ref) => AuthRepository(ref.watch(apiClientProvider), ref.watch(secureStorageProvider)),
);

final projectRepositoryProvider = Provider<ProjectRepository>(
  (ref) => ProjectRepository(ref.watch(apiClientProvider)),
);

enum AuthStatus { unknown, authenticated, unauthenticated }

class AuthState {
  const AuthState({required this.status, this.user});

  final AuthStatus status;
  final User? user;

  static const initial = AuthState(status: AuthStatus.unknown);
}

/// Session/auth state gate consumed by go_router's redirect callback.
/// Mirrors the web app's implicit client-side auth check (dashboard/page.tsx
/// redirecting to /sign-in on a 401).
class AuthController extends StateNotifier<AuthState> {
  AuthController(this._repository, ApiClient api) : super(AuthState.initial) {
    // Wired here (rather than inside apiClientProvider) to avoid a
    // circular provider dependency: apiClientProvider must not know about
    // authControllerProvider, since authControllerProvider already depends
    // on apiClientProvider transitively via authRepositoryProvider.
    api.onUnauthorized = handleUnauthorized;
    _bootstrap();
  }

  final AuthRepository _repository;

  Future<void> _bootstrap() async {
    if (!await _repository.hasStoredSession()) {
      state = const AuthState(status: AuthStatus.unauthenticated);
      return;
    }
    try {
      final user = await _repository.currentUser();
      state = AuthState(status: AuthStatus.authenticated, user: user);
    } catch (_) {
      await _repository.logout();
      state = const AuthState(status: AuthStatus.unauthenticated);
    }
  }

  Future<void> login({required String email, required String password}) async {
    final user = await _repository.login(email: email, password: password);
    state = AuthState(status: AuthStatus.authenticated, user: user);
  }

  Future<void> register({required String name, required String email, required String password}) async {
    final user = await _repository.register(name: name, email: email, password: password);
    state = AuthState(status: AuthStatus.authenticated, user: user);
  }

  Future<void> signInWithGoogle() async {
    final user = await _repository.signInWithGoogle();
    state = AuthState(status: AuthStatus.authenticated, user: user);
  }

  Future<void> logout() async {
    await _repository.logout();
    state = const AuthState(status: AuthStatus.unauthenticated);
  }

  void handleUnauthorized() {
    if (state.status == AuthStatus.authenticated) {
      state = const AuthState(status: AuthStatus.unauthenticated);
    }
  }
}

final authControllerProvider = StateNotifierProvider<AuthController, AuthState>(
  (ref) => AuthController(ref.watch(authRepositoryProvider), ref.watch(apiClientProvider)),
);
