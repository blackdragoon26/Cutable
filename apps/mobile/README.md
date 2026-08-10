# Cutable Mobile

The Flutter client for [Cutable](../../README.md) — an AI web-app builder.
Sign in, create a project, and watch the AI agent build a React app in a
live-streamed chat, with a read-only file browser and an in-app preview,
from iOS or Android.

## What is included

- Email/password and Google sign-in against the Go API's Bearer-token auth
  (`apps/backend/internal/httpapi/server.go`, `google_oauth.go`)
- Project list, creation (with text/image attachments), and a tabbed
  workspace (Chat / Files / Preview)
- A WebSocket client mirroring the web app's reconnect/backoff and
  build-progress event handling (`lib/data/ws_client.dart`,
  `lib/features/workspace/providers/agent_session_provider.dart`)
- Light/dark themes matching the web app's palette
  (`lib/core/theme/app_theme.dart`)
- Riverpod for state management, `go_router` for navigation

## Prerequisites

- Flutter (stable channel) — see the repository root for the pinned version
  used in CI (`.github/workflows/mobile.yml`)
- A running Cutable backend — see the [repository root README](../../README.md)
  for local setup (Postgres, `.env`, `npm run dev:backend`)
- Xcode (iOS) and/or Android Studio + an emulator (Android)

## Running locally

Point the app at your backend with `--dart-define`. Defaults match the
backend's local dev port (`3010`).

```sh
flutter pub get

# iOS Simulator
flutter run -d <ios-simulator-id> \
  --dart-define=API_BASE_URL=http://localhost:3010 \
  --dart-define=WS_BASE_URL=ws://localhost:3010

# Android emulator (10.0.2.2 is the emulator's alias for the host machine)
flutter run -d <android-emulator-id> \
  --dart-define=API_BASE_URL=http://10.0.2.2:3010 \
  --dart-define=WS_BASE_URL=ws://10.0.2.2:3010
```

`lib/core/config.dart` documents the full set of `--dart-define` keys.

## Google sign-in

The mobile OAuth handoff uses a `cutable://auth-callback` custom URL scheme
(registered in `ios/Runner/Info.plist` and
`android/app/src/main/AndroidManifest.xml`), captured via
`flutter_web_auth_2`. The backend's `/api/auth/google` endpoint detects
`?platform=mobile` and redirects to that scheme instead of a web page after
completing the same PKCE flow used on the web
(`apps/backend/internal/httpapi/google_oauth.go`).

## Verification

```sh
flutter analyze
flutter test
```

`flutter test` covers the WebSocket reconnect/backoff state machine, auth
token storage flow, and the WS-event-to-chat-message formatting logic —
see `test/`.

## App icon

Generated from the same brand mark the web app uses for its favicon
(`assets/brand/cutable-mark.png`, sourced from
`apps/frontend/public/brand/cutable-mark-v3.png`) via `flutter_launcher_icons`
(configured in `pubspec.yaml`). Regenerate after changing the mark:

```sh
dart run flutter_launcher_icons
```

## Repository layout

```text
lib/
  core/           config, theme
  data/           REST/WebSocket clients, repositories, models
  features/       one directory per screen area (auth, dashboard,
                   new_project, workspace, settings)
  routing/        go_router configuration
test/             unit tests (see Verification above)
ios/, android/    native platform projects
```
