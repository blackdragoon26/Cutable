import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../features/auth/providers/auth_providers.dart';
import '../features/auth/screens/sign_in_screen.dart';
import '../features/auth/screens/sign_up_screen.dart';
import '../features/auth/screens/splash_screen.dart';
import '../features/dashboard/screens/dashboard_screen.dart';
import '../features/new_project/screens/new_project_screen.dart';
import '../features/settings/screens/account_usage_screen.dart';
import '../features/workspace/screens/project_workspace_screen.dart';

/// go_router redirect gates routes on AuthController's status, mirroring
/// the web app's client-side auth check (dashboard/page.tsx redirecting to
/// /sign-in?next=/dashboard on a 401).
final appRouterProvider = Provider<GoRouter>((ref) {
  final authListenable = _AuthChangeNotifier(ref);
  return GoRouter(
    initialLocation: '/',
    refreshListenable: authListenable,
    redirect: (context, state) {
      final auth = ref.read(authControllerProvider);
      final path = state.matchedLocation;
      final isAuthRoute = path == '/sign-in' || path == '/sign-up';

      if (auth.status == AuthStatus.unknown) {
        return path == '/' ? null : '/';
      }
      if (auth.status == AuthStatus.unauthenticated) {
        return isAuthRoute ? null : '/sign-in';
      }
      // authenticated
      if (path == '/' || isAuthRoute) {
        return '/dashboard';
      }
      return null;
    },
    routes: [
      GoRoute(path: '/', builder: (context, state) => const SplashScreen()),
      GoRoute(path: '/sign-in', builder: (context, state) => const SignInScreen()),
      GoRoute(path: '/sign-up', builder: (context, state) => const SignUpScreen()),
      GoRoute(path: '/dashboard', builder: (context, state) => const DashboardScreen()),
      GoRoute(path: '/projects/new', builder: (context, state) => const NewProjectScreen()),
      GoRoute(
        path: '/projects/:id',
        builder: (context, state) => ProjectWorkspaceScreen(projectId: state.pathParameters['id']!),
      ),
      GoRoute(path: '/account/usage', builder: (context, state) => const AccountUsageScreen()),
    ],
  );
});

class _AuthChangeNotifier extends ChangeNotifier {
  _AuthChangeNotifier(this._ref) {
    _ref.listen(authControllerProvider, (previous, next) {
      if (previous?.status != next.status) notifyListeners();
    });
  }

  final Ref _ref;
}
