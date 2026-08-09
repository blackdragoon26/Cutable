import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../providers/auth_providers.dart';

class SplashScreen extends ConsumerWidget {
  const SplashScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // go_router's redirect handles navigating away once AuthStatus resolves
    // (see routing/app_router.dart); this screen only needs to render while
    // AuthController bootstraps.
    ref.watch(authControllerProvider);
    return const Scaffold(
      body: Center(child: CircularProgressIndicator()),
    );
  }
}
