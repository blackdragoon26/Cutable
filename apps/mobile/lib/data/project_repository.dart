import 'api_client.dart';
import 'models/file_node.dart';
import 'models/project.dart';

/// Wraps apps/backend's project/file/sandbox/usage REST endpoints
/// (apps/backend/internal/httpapi/server.go:56-77, all authed via s.auth).
class ProjectRepository {
  ProjectRepository(this._api);

  final ApiClient _api;

  Future<List<Project>> listProjects() async {
    final response = await _api.dio.get('/api/projects');
    return (response.data['projects'] as List<dynamic>)
        .map((item) => Project.fromJson(item as Map<String, dynamic>))
        .toList();
  }

  Future<Project> getProject(String id) async {
    final response = await _api.dio.get('/api/projects/$id');
    return Project.fromJson(response.data['project'] as Map<String, dynamic>);
  }

  Future<Project> createProject({
    required String title,
    required String initialPrompt,
    List<ProjectAttachment> attachments = const [],
  }) async {
    final response = await _api.dio.post('/api/projects', data: {
      'title': title,
      'initialPrompt': initialPrompt,
      'attachments': attachments.map((attachment) => attachment.toJson()).toList(),
    });
    return Project.fromJson(response.data['project'] as Map<String, dynamic>);
  }

  Future<FileNode> listFiles(String projectId) async {
    final response = await _api.dio.get('/api/projects/$projectId/files');
    return FileNode.fromJson(response.data['files'] as Map<String, dynamic>);
  }

  Future<String> getFileContent(String projectId, String path) async {
    // The backend route is a wildcard path segment
    // (GET /api/projects/{id}/files/{path...}); encode each path component
    // individually so slashes stay as directory separators while other
    // special characters are safely escaped.
    final encodedPath = path.split('/').map(Uri.encodeComponent).join('/');
    final response = await _api.dio.get('/api/projects/$projectId/files/$encodedPath');
    return response.data['content'] as String? ?? '';
  }

  Future<({String? sandboxId, String? previewUrl})> getSandbox(String projectId) async {
    final response = await _api.dio.get('/api/projects/$projectId/sandbox');
    return (
      sandboxId: response.data['sandboxId'] as String?,
      previewUrl: response.data['previewUrl'] as String?,
    );
  }

  Future<Map<String, dynamic>> accountUsage() async {
    final response = await _api.dio.get('/api/account/usage');
    return response.data['demo'] as Map<String, dynamic>;
  }
}
