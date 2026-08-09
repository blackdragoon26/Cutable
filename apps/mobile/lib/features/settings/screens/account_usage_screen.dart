import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../auth/providers/auth_providers.dart';

final accountUsageProvider = FutureProvider.autoDispose((ref) {
  return ref.watch(projectRepositoryProvider).accountUsage();
});

/// Mirrors GET /api/account/usage consumers in the web app (demo-run
/// counters surfaced in dialogs/banners).
class AccountUsageScreen extends ConsumerWidget {
  const AccountUsageScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final usageAsync = ref.watch(accountUsageProvider);
    return Scaffold(
      appBar: AppBar(title: const Text('Usage')),
      body: usageAsync.when(
        data: (usage) {
          final used = usage['used'] as int? ?? 0;
          final limit = usage['limit'] as int? ?? 0;
          final remaining = usage['remaining'] as int? ?? 0;
          final requiresKeys = usage['requiresKeys'] as bool? ?? false;
          return ListView(
            padding: const EdgeInsets.all(20),
            children: [
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(20),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text('Demo builds', style: Theme.of(context).textTheme.titleMedium),
                      const SizedBox(height: 12),
                      LinearProgressIndicator(value: limit == 0 ? 0 : used / limit),
                      const SizedBox(height: 8),
                      Text('$used of $limit demo builds used · $remaining remaining'),
                      if (requiresKeys) ...[
                        const SizedBox(height: 12),
                        Text(
                          'Your demo builds are used. Add your own OpenRouter and E2B API keys in Settings to keep building.',
                          style: Theme.of(context).textTheme.bodySmall,
                        ),
                      ],
                    ],
                  ),
                ),
              ),
            ],
          );
        },
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => Center(child: Text('Could not load usage: $error')),
      ),
    );
  }
}
