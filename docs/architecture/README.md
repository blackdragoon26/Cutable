# Cutable architecture

This is a code-backed system map, not a proposed architecture. Each material
claim links to the implementation that currently enforces it. The diagrams are
generated from one scene definition:

```sh
node scripts/generate-architecture-diagrams.mjs
```

Open any `.excalidraw` file in [Excalidraw](https://excalidraw.com) to edit it.
The matching PNG is for Markdown and the SVG is for the product documentation.

## 01 — System context

![System context](diagrams/system-context.png)

The browser renders the Next.js application on Vercel. All durable product
state and authorization decisions pass through the Go API deployed by Myprod.
The API coordinates three external systems:

- Neon stores users, projects, attachments, conversations, generated files,
  sandbox identifiers, and demo usage.
- OpenRouter returns plans and function calls. It does not execute tools.
- E2B hosts the generated React workspace and executes file, package, build,
  and preview commands.

Generated application code never executes in the Vercel frontend or the Go
container.

A Flutter client (`apps/mobile`) is a second, native front end talking to the
same Go API — see [05 — Mobile client](#05--mobile-client) below. The
diagram above predates it and still shows only the browser path; the API,
Neon, OpenRouter, and E2B relationships it depicts are unchanged for mobile.

## 02 — One user build

![User build flow](diagrams/user-build-flow.png)

1. A password login or Google OAuth callback creates the same HttpOnly JWT
   session.
2. The API validates the prompt and attachments before one database
   transaction creates the project.
3. The WebSocket handshake parses the project ID, checks user ownership, and
   restricts the accepted origin.
4. A run uses a complete OpenRouter + E2B key pair or atomically claims one of
   the account’s two demo runs.
5. Stage, thinking, tool, build, file, and completion events stream back over
   the authenticated socket.
6. The E2B preview remains separately addressable while the generated source
   is persisted in Neon.

BYOK credentials are held in browser `sessionStorage`, sent in the run message,
used to construct providers for that run, and never passed to a store method.
They still travel through the Go process and the respective providers; this is
session-only handling, not end-to-end provider encryption.

## 03 — AI execution loop

![AI execution loop](diagrams/ai-agent-loop.png)

The control split is deliberate:

| Layer | Authority |
| --- | --- |
| OpenRouter model | Proposes the plan, tool name, and JSON arguments |
| Go runner | Parses arguments, selects a registered tool, constrains file paths, controls step/time limits, persists results |
| E2B | Executes filesystem and shell operations inside `/home/user/react-app` |
| Neon | Preserves project state independently of sandbox lifetime |
| Browser | Observes events and renders the source/preview; it does not execute the agent |

`execute_command` is intentionally powerful inside E2B. The security boundary
is sandbox isolation, app-directory execution, request timeout, and the absence
of a host-shell path—not a shell-command allowlist.

### Tool surface

```text
write_file          write_multiple_files
read_file           delete_file           rename_file
list_directories    execute_command
add_dependency      check_missing_dependencies
test_build          start_dev_server
```

The model loop is serial: `parallel_tool_calls=false`. Each tool result is
appended to the model conversation before the next completion. A run ends when
the model returns no tool call, an operation fails irrecoverably, the maximum
step count is reached, or the agent timeout expires.

## 04 — Trust and delivery

![Trust and delivery](diagrams/trust-and-delivery.png)

### Runtime boundaries

- Auth cookies are `HttpOnly`, configurable `Secure`, and `SameSite=Lax`.
- Project reads and WebSocket upgrades include the authenticated user ID in
  the ownership query.
- WebSocket origins are reduced to the configured frontend host.
- Generated paths are normalized and rejected if they escape the React root.
- Image content is checked from decoded bytes, not only filename or MIME label.
- Provider and database secrets live in a mode `0400` host file, mounted
  read-only into the nonroot backend container.

### Delivery path

```text
Git push
  ├─ Vercel: apps/frontend → Next.js build → edge deployment
  └─ GitHub Actions: Docker buildx → GHCR digest
                                  → Myprod application spec
                                  → Nomad allocation
                                  → Traefik HTTPS route
```

The backend image is a statically linked Go binary copied into Distroless and
runs as `nonroot`. CI publishes AMD64 and ARM64 images with a commit tag and an
immutable digest.

## 05 — Mobile client

`apps/mobile` is a Flutter app for iOS and Android with the same auth,
project, and AI-build-streaming capabilities as the web app, talking to the
identical Go API and WebSocket endpoint. It does not have its own diagram
yet; the system context, build-flow, and execution-loop views above apply
unchanged once "browser" is read as "client."

What differs from the browser client, and why:

- **Auth transport.** A browser gets an `HttpOnly` session cookie
  automatically resent on every request. A native app has no cookie jar
  shared with a browser, so `s.auth` (`server.go`) also accepts an
  `Authorization: Bearer <jwt>` header — the same JWT, same 24h expiry, same
  `verifyJWT` validation path, just carried differently. Login/register
  responses now include `"token"` in the JSON body in addition to setting
  the cookie, so the mobile client has something to store. The token is kept
  in `flutter_secure_storage` (Keychain / EncryptedSharedPreferences)
  instead of a cookie jar.
- **WebSocket auth.** Browsers authenticate the `/ws` upgrade via the
  automatically-sent cookie. The Bearer header works for native WebSocket
  clients too (`bearerToken` in `server.go` checks the header first); a
  `?token=` query-parameter fallback exists for upgrade paths where setting
  a header proves unreliable. `bearerToken` only honors that query fallback
  when the request itself carries `Upgrade: websocket`, so it can't be used
  to bypass Bearer/cookie auth on ordinary REST calls.
- **Google OAuth.** The browser flow is a full page redirect ending in a
  `302` back to the frontend with the session cookie already set. The
  mobile flow (`google_oauth.go`, `platform=mobile` query param) reuses the
  identical PKCE/state exchange, then redirects to a `cutable://auth-callback`
  custom URL scheme carrying the JWT, captured in-app via
  `flutter_web_auth_2` (ASWebAuthenticationSession on iOS, Custom Tabs on
  Android) instead of landing in a browser tab.
- **BYOK credentials.** The web app keeps OpenRouter/E2B keys in
  `sessionStorage` (tab-scoped). Mobile has no equivalent session boundary,
  so it uses `flutter_secure_storage` with an explicit "forget keys" action
  in the settings sheet — same rule (session-only, user-controlled, never
  persisted server-side), different mechanism for the same effect.
- **No in-app code editor.** The mobile Files tab is read-only (view
  content, no Monaco-equivalent editing) — everything else in the AI
  execution loop (03) is identical, since editing was always a client-side
  affordance, not something the agent loop depends on.

## Data model

```text
users
  1 ─── * projects
           ├── * project_attachments
           ├── * conversations
           └── * project_files (unique project_id + path)
```

The project is the ownership aggregate. Child records use foreign keys with
cascade deletion. `demo_runs_used` belongs to the user because the allowance
is account-level, not project-level.

## Verification map

| Claim | Enforcement / evidence |
| --- | --- |
| Project access is owner-scoped | [`Store.Project`](../../apps/backend/internal/store/store.go), HTTP and WebSocket handlers |
| Demo claims are atomic | [`Store.ClaimDemoRun`](../../apps/backend/internal/store/store.go), `UPDATE … WHERE demo_runs_used < limit RETURNING` |
| BYOK is per-run and not persisted | [`websocket.go`](../../apps/backend/internal/httpapi/websocket.go), provider construction occurs in the socket handler |
| Generated file paths stay under the app root | [`safePath`](../../apps/backend/internal/agent/agent.go) |
| Tool execution happens in E2B | [`Runner.executeTool`](../../apps/backend/internal/agent/agent.go), [`provider/e2b.go`](../../apps/backend/internal/provider/e2b.go) |
| OpenRouter emits serial tool calls | [`provider/openrouter.go`](../../apps/backend/internal/provider/openrouter.go), `ParallelToolCalls: false` |
| Files survive sandbox replacement | [`Runner.ensureSandbox`](../../apps/backend/internal/agent/agent.go) restores `project_files` |
| Migrations run at service startup | [`cmd/server/main.go`](../../apps/backend/cmd/server/main.go) and embedded store migrations |
| Attachments are size/type/content checked | [`validateAttachments`](../../apps/backend/internal/httpapi/server.go) |
| Password and Google auth converge on one cookie | [`server.go`](../../apps/backend/internal/httpapi/server.go), [`google_oauth.go`](../../apps/backend/internal/httpapi/google_oauth.go) |
| Backend is nonroot and multi-architecture | [`Dockerfile`](../../apps/backend/Dockerfile), [`publish-backend.yml`](../../.github/workflows/publish-backend.yml) |
| Mobile Bearer/cookie auth share one verification path | [`verifyJWT`](../../apps/backend/internal/httpapi/server.go), used by both `s.auth` and the WebSocket upgrade |
| WS `?token=` fallback only applies to upgrade requests | [`bearerToken`](../../apps/backend/internal/httpapi/server.go) |
| Mobile OAuth reuses the same PKCE/state exchange as web | [`google_oauth.go`](../../apps/backend/internal/httpapi/google_oauth.go), `platform=mobile` branch in `googleCallback` |
| Mobile secret storage uses the platform keychain, not plain prefs | [`secure_storage.dart`](../../apps/mobile/lib/data/secure_storage.dart) (`flutter_secure_storage`) |

## Known boundaries

- E2B and OpenRouter are third-party processors and receive the data required
  for their roles.
- Conversation and file persistence is relational; there is no collaborative
  merge protocol or version-control history for generated files.
- Demo usage is a run allowance, not a token or compute budget.
- Sandbox preview availability follows E2B lifetime and successful Vite
  startup.
- The model may generate insecure application code. E2B isolates execution,
  but it is not a substitute for reviewing generated code before deployment.

## Diagram inventory

| View | Editable | Export |
| --- | --- | --- |
| System context | [`system-context.excalidraw`](diagrams/system-context.excalidraw) | [PNG](diagrams/system-context.png) · [SVG](diagrams/system-context.svg) |
| User build flow | [`user-build-flow.excalidraw`](diagrams/user-build-flow.excalidraw) | [PNG](diagrams/user-build-flow.png) · [SVG](diagrams/user-build-flow.svg) |
| AI execution loop | [`ai-agent-loop.excalidraw`](diagrams/ai-agent-loop.excalidraw) | [PNG](diagrams/ai-agent-loop.png) · [SVG](diagrams/ai-agent-loop.svg) |
| Trust and delivery | [`trust-and-delivery.excalidraw`](diagrams/trust-and-delivery.excalidraw) | [PNG](diagrams/trust-and-delivery.png) · [SVG](diagrams/trust-and-delivery.svg) |
