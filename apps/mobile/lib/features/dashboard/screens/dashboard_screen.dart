import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/theme/theme_provider.dart';
import '../../../data/models/project.dart';
import '../../auth/providers/auth_providers.dart';
import '../../settings/screens/provider_keys_sheet.dart';
import '../widgets/project_card.dart';

final projectsProvider = FutureProvider.autoDispose<List<Project>>((ref) {
  return ref.watch(projectRepositoryProvider).listProjects();
});

/// Mirrors apps/frontend/app/dashboard/page.tsx.
class DashboardScreen extends ConsumerWidget {
  const DashboardScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final projectsAsync = ref.watch(projectsProvider);
    final user = ref.watch(authControllerProvider).user;
    final themeMode = ref.watch(themeModeProvider);

    return Scaffold(
      appBar: AppBar(
        title: Text(user != null ? 'Hi, ${user.name.split(' ').first}' : 'Projects'),
        actions: [
          IconButton(
            tooltip: 'Toggle theme',
            icon: Icon(themeMode == ThemeMode.dark ? Icons.dark_mode : Icons.light_mode),
            onPressed: () {
              final next = themeMode == ThemeMode.dark ? ThemeMode.light : ThemeMode.dark;
              ref.read(themeModeProvider.notifier).setThemeMode(next);
            },
          ),
          IconButton(
            tooltip: 'API keys',
            icon: const Icon(Icons.vpn_key_outlined),
            onPressed: () => showProviderKeysSheet(context, ref),
          ),
          IconButton(
            tooltip: 'Usage',
            icon: const Icon(Icons.speed_outlined),
            onPressed: () => context.push('/account/usage'),
          ),
          IconButton(
            tooltip: 'Sign out',
            icon: const Icon(Icons.logout),
            onPressed: () => ref.read(authControllerProvider.notifier).logout(),
          ),
        ],
      ),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: () => context.push('/projects/new'),
        icon: const Icon(Icons.add),
        label: const Text('New project'),
      ),
      body: RefreshIndicator(
        onRefresh: () => ref.refresh(projectsProvider.future),
        child: projectsAsync.when(
          data: (projects) {
            if (projects.isEmpty) {
              return ListView(
                padding: const EdgeInsets.all(24),
                children: const [
                  SizedBox(height: 80),
                  Center(child: Text('No projects yet — tap "New project" to start building.')),
                ],
              );
            }
            return GridView.builder(
              padding: const EdgeInsets.all(16),
              gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
                maxCrossAxisExtent: 420,
                mainAxisExtent: 150,
                crossAxisSpacing: 12,
                mainAxisSpacing: 12,
              ),
              itemCount: projects.length,
              itemBuilder: (context, index) {
                final project = projects[index];
                return ProjectCard(
                  project: project,
                  onTap: () => context.push('/projects/${project.id}'),
                );
              },
            );
          },
          error: (error, stackTrace) => ListView(
            padding: const EdgeInsets.all(24),
            children: [Center(child: Text('Could not load projects: $error'))],
          ),
          loading: () => const Center(child: CircularProgressIndicator()),
        ),
      ),
    );
  }
}
