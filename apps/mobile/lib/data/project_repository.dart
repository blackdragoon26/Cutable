import 'api_client.dart';
import 'models/conversation.dart';
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

  /// Returns the top-level file tree entries. The backend's
  /// store.BuildFileTree (apps/backend/internal/store/store.go:447) returns
  /// a JSON array of sibling FileNodes at the project root, not a single
  /// wrapper node with a "children" field.
  Future<List<FileNode>> listFiles(String projectId) async {
    final response = await _api.dio.get('/api/projects/$projectId/files');
    return (response.data['files'] as List<dynamic>)
        .map((item) => FileNode.fromJson(item as Map<String, dynamic>))
        .toList();
  }

  /// Past chat history for a project (GET /api/projects/{id}/conversations)
  /// so reopening an already-built project shows what happened instead of
  /// an empty chat pane — the agent only re-runs for brand-new projects.
  Future<List<Conversation>> getConversations(String projectId) async {
    final response = await _api.dio.get('/api/projects/$projectId/conversations');
    return (response.data['conversations'] as List<dynamic>)
        .map((item) => Conversation.fromJson(item as Map<String, dynamic>))
        .toList();
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

  /// Reconnects the project's E2B sandbox if it's still alive, or recreates
  /// it and replays the persisted files otherwise (mirrors the web app's
  /// "Restart preview" — apps/backend/internal/httpapi/sandbox.go
  /// createSandbox already handles both cases server-side; the mobile app
  /// just never called it, so a preview whose sandbox had expired stayed
  /// dead forever instead of coming back).
  Future<({String? sandboxId, String? previewUrl})> restartSandbox(
    String projectId, {
    String? e2bApiKey,
  }) async {
    // Only the E2B key matters here — restarting a sandbox never calls the
    // AI model — but the backend accepts the same {credentials} shape used
    // for a full build, so an OpenRouter key is simply omitted.
    final response = await _api.dio.post(
      '/api/projects/$projectId/sandbox',
      data: e2bApiKey != null
          ? {
              'credentials': {'e2bApiKey': e2bApiKey},
            }
          : null,
    );
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
