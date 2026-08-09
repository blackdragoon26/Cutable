import 'dart:convert';
import 'dart:typed_data';

import 'package:file_selector/file_selector.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:image_picker/image_picker.dart';

import '../../../core/theme/app_theme.dart';
import '../../../data/api_client.dart';
import '../../../data/models/project.dart';
import '../../auth/providers/auth_providers.dart';
import '../../dashboard/screens/dashboard_screen.dart';

const _maxAttachments = 3; // mirrors validateAttachments in server.go
const _maxTextFileBytes = 100 * 1024;
const _maxImageFileBytes = 2 * 1024 * 1024;
const _textExtensions = {
  'css', 'csv', 'go', 'html', 'java', 'js', 'json', 'jsx', 'md', 'py',
  'rs', 'sql', 'ts', 'tsx', 'txt', 'xml', 'yaml', 'yml',
};

/// Mirrors apps/frontend's prompt input + attachment flow
/// (PromptInput.tsx / api.ts ProjectAttachmentInput), sized down to a
/// single prompt-entry screen for mobile.
class NewProjectScreen extends ConsumerStatefulWidget {
  const NewProjectScreen({super.key});

  @override
  ConsumerState<NewProjectScreen> createState() => _NewProjectScreenState();
}

class _NewProjectScreenState extends ConsumerState<NewProjectScreen> {
  final _titleController = TextEditingController();
  final _promptController = TextEditingController();
  final List<ProjectAttachment> _attachments = [];
  bool _submitting = false;
  String? _error;

  @override
  void dispose() {
    _titleController.dispose();
    _promptController.dispose();
    super.dispose();
  }

  Future<void> _addTextFile() async {
    final file = await openFile(acceptedTypeGroups: const [
      XTypeGroup(label: 'source/text', extensions: [
        'css', 'csv', 'go', 'html', 'java', 'js', 'json', 'jsx', 'md', 'py',
        'rs', 'sql', 'ts', 'tsx', 'txt', 'xml', 'yaml', 'yml',
      ]),
    ]);
    if (file == null) return;
    final extension = file.name.split('.').last.toLowerCase();
    if (!_textExtensions.contains(extension)) {
      _showError('"${file.name}" is not a supported text or source file.');
      return;
    }
    final bytes = await file.readAsBytes();
    if (bytes.length > _maxTextFileBytes) {
      _showError('"${file.name}" exceeds the 100 KB text attachment limit.');
      return;
    }
    String content;
    try {
      content = utf8.decode(bytes);
    } catch (_) {
      _showError('"${file.name}" must be UTF-8 text.');
      return;
    }
    _addAttachment(ProjectAttachment(
      name: file.name,
      kind: 'text',
      mimeType: 'text/plain',
      content: content,
      size: bytes.length,
    ));
  }

  Future<void> _addImage() async {
    final picked = await ImagePicker().pickImage(source: ImageSource.gallery, imageQuality: 90);
    if (picked == null) return;
    final Uint8List bytes = await picked.readAsBytes();
    if (bytes.length > _maxImageFileBytes) {
      _showError('"${picked.name}" exceeds the 2 MB image attachment limit.');
      return;
    }
    final extension = picked.name.split('.').last.toLowerCase();
    final mimeType = switch (extension) {
      'png' => 'image/png',
      'jpg' || 'jpeg' => 'image/jpeg',
      'webp' => 'image/webp',
      _ => null,
    };
    if (mimeType == null) {
      _showError('Only PNG, JPEG, or WebP images are supported.');
      return;
    }
    final dataUrl = 'data:$mimeType;base64,${base64Encode(bytes)}';
    _addAttachment(ProjectAttachment(
      name: picked.name,
      kind: 'image',
      mimeType: mimeType,
      content: dataUrl,
      size: bytes.length,
    ));
  }

  void _addAttachment(ProjectAttachment attachment) {
    if (_attachments.length >= _maxAttachments) {
      _showError('A maximum of $_maxAttachments attachments is allowed.');
      return;
    }
    setState(() => _attachments.add(attachment));
  }

  void _showError(String message) {
    ScaffoldMessenger.of(context).showSnackBar(SnackBar(content: Text(message)));
  }

  Future<void> _submit() async {
    final title = _titleController.text.trim();
    final prompt = _promptController.text.trim();
    if (title.isEmpty || prompt.isEmpty) {
      setState(() => _error = 'Title and prompt are required.');
      return;
    }
    setState(() {
      _submitting = true;
      _error = null;
    });
    try {
      final project = await ref.read(projectRepositoryProvider).createProject(
            title: title,
            initialPrompt: prompt,
            attachments: _attachments,
          );
      if (mounted) {
        ref.invalidate(projectsProvider);
        context.go('/projects/${project.id}');
      }
    } catch (error) {
      setState(() => _error = ApiClient.errorMessage(error));
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('New project')),
      body: SafeArea(
        child: SingleChildScrollView(
          padding: const EdgeInsets.all(20),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              Text(
                'Turn a clear idea into a working React application.',
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const SizedBox(height: 16),
              TextField(
                controller: _titleController,
                decoration: const InputDecoration(labelText: 'Project title'),
              ),
              const SizedBox(height: 12),
              TextField(
                controller: _promptController,
                minLines: 5,
                maxLines: 10,
                decoration: const InputDecoration(
                  labelText: 'Describe what you need',
                  alignLabelWithHint: true,
                ),
              ),
              const SizedBox(height: 16),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  OutlinedButton.icon(
                    onPressed: _addTextFile,
                    icon: const Icon(Icons.description_outlined),
                    label: const Text('Add text/source file'),
                  ),
                  OutlinedButton.icon(
                    onPressed: _addImage,
                    icon: const Icon(Icons.image_outlined),
                    label: const Text('Add image reference'),
                  ),
                ],
              ),
              if (_attachments.isNotEmpty) ...[
                const SizedBox(height: 12),
                ..._attachments.map((attachment) => ListTile(
                      contentPadding: EdgeInsets.zero,
                      leading: Icon(attachment.kind == 'image' ? Icons.image : Icons.insert_drive_file),
                      title: Text(attachment.name),
                      subtitle: Text('${(attachment.size / 1024).toStringAsFixed(1)} KB'),
                      trailing: IconButton(
                        icon: const Icon(Icons.close),
                        onPressed: () => setState(() => _attachments.remove(attachment)),
                      ),
                    )),
              ],
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
              ],
              const SizedBox(height: 20),
              ElevatedButton(
                onPressed: _submitting ? null : _submit,
                child: _submitting
                    ? const SizedBox(width: 18, height: 18, child: CircularProgressIndicator(strokeWidth: 2))
                    : const Text('Start building'),
              ),
              const SizedBox(height: 8),
              Text(
                'Session-only storage. Your demo runs stay untouched.',
                textAlign: TextAlign.center,
                style: Theme.of(context).textTheme.bodySmall?.copyWith(color: AppTheme.mutedOf(context)),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
