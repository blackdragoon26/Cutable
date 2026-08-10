import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../../data/models/file_node.dart';
import '../../../data/models/project.dart';
import '../../auth/providers/auth_providers.dart';

final projectDetailProvider =
    FutureProvider.autoDispose.family<Project, String>((ref, projectId) {
  return ref.watch(projectRepositoryProvider).getProject(projectId);
});

final fileTreeProvider =
    FutureProvider.autoDispose.family<List<FileNode>, String>((ref, projectId) {
  return ref.watch(projectRepositoryProvider).listFiles(projectId);
});

final fileContentProvider = FutureProvider.autoDispose
    .family<String, ({String projectId, String path})>((ref, args) {
  return ref.watch(projectRepositoryProvider).getFileContent(args.projectId, args.path);
});

final sandboxInfoProvider = FutureProvider.autoDispose
    .family<({String? sandboxId, String? previewUrl}), String>((ref, projectId) {
  return ref.watch(projectRepositoryProvider).getSandbox(projectId);
});
