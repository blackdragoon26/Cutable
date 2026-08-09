import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../auth/providers/auth_providers.dart';

/// Mobile analog of apps/frontend's ConnectProviderKeys.tsx /
/// ProviderKeyDialog.tsx. The web app stores BYO OpenRouter/E2B keys in
/// sessionStorage only (never sent to the server outside build/sandbox
/// requests, never persisted server-side). Mobile has no browser-tab
/// session boundary, so keys go in flutter_secure_storage instead, with an
/// explicit "Forget keys" action to preserve the same "user controls
/// retention" spirit.
Future<void> showProviderKeysSheet(BuildContext context, WidgetRef ref) {
  return showModalBottomSheet(
    context: context,
    isScrollControlled: true,
    builder: (context) => const _ProviderKeysSheet(),
  );
}

class _ProviderKeysSheet extends ConsumerStatefulWidget {
  const _ProviderKeysSheet();

  @override
  ConsumerState<_ProviderKeysSheet> createState() => _ProviderKeysSheetState();
}

class _ProviderKeysSheetState extends ConsumerState<_ProviderKeysSheet> {
  final _openRouterController = TextEditingController();
  final _e2bController = TextEditingController();
  bool _saving = false;

  @override
  void initState() {
    super.initState();
    final storage = ref.read(secureStorageProvider);
    storage.readOpenRouterKey().then((value) {
      if (mounted && value != null) _openRouterController.text = value;
    });
    storage.readE2BKey().then((value) {
      if (mounted && value != null) _e2bController.text = value;
    });
  }

  @override
  void dispose() {
    _openRouterController.dispose();
    _e2bController.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    setState(() => _saving = true);
    await ref.read(secureStorageProvider).writeProviderKeys(
          openRouterKey: _openRouterController.text.trim(),
          e2bKey: _e2bController.text.trim(),
        );
    if (mounted) Navigator.of(context).pop();
  }

  Future<void> _forget() async {
    await ref.read(secureStorageProvider).forgetProviderKeys();
    _openRouterController.clear();
    _e2bController.clear();
    if (mounted) Navigator.of(context).pop();
  }

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: EdgeInsets.fromLTRB(20, 20, 20, MediaQuery.of(context).viewInsets.bottom + 20),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text('API keys', style: Theme.of(context).textTheme.titleLarge),
          const SizedBox(height: 8),
          Text(
            'Bring your own OpenRouter and E2B keys to keep building after your two demo runs. '
            'Stored securely on this device only.',
            style: Theme.of(context).textTheme.bodySmall,
          ),
          const SizedBox(height: 20),
          TextField(
            controller: _openRouterController,
            obscureText: true,
            decoration: const InputDecoration(labelText: 'OpenRouter API key'),
          ),
          const SizedBox(height: 12),
          TextField(
            controller: _e2bController,
            obscureText: true,
            decoration: const InputDecoration(labelText: 'E2B API key'),
          ),
          const SizedBox(height: 20),
          ElevatedButton(
            onPressed: _saving ? null : _save,
            child: const Text('Save keys'),
          ),
          const SizedBox(height: 8),
          TextButton(onPressed: _forget, child: const Text('Forget keys')),
        ],
      ),
    );
  }
}
