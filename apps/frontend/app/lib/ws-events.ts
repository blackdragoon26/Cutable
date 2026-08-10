/**
 * Shared type definitions for WebSocket events flowing between the backend
 * agent runtime and the frontend. Every event carries a discriminating `e`
 * field; the fields below cover every event value handled by
 * `useWebSocket.ts` and `useAgentSession.ts`.
 *
 * This is intentionally a single "wide" interface (all fields optional)
 * rather than a strict discriminated union: the backend is free to add new
 * event types over time, and callers only read known fields off a given
 * event. An `unknown` index signature is kept as a catch-all instead of
 * `any` so any accidental untyped access is still caught by the compiler.
 */
export interface WebSocketEvent {
  /** Discriminator identifying the event type, e.g. "stage_update". */
  e: string;

  // connected
  authenticated?: boolean;

  // stage_update / stage_error
  stage?: string;
  message?: string;
  progress?: number;
  currentStep?: number;
  totalSteps?: number;

  // plan_generated
  plan?: string[];

  // step_completed
  step?: string;
  remainingSteps?: number;

  // tool_started / tool_completed / tool_error
  tool?: string;
  input?: unknown;
  output?: unknown;
  error?: string;

  // file_created / file_content_updated / file_read
  filepath?: string;

  // files_created
  files?: unknown[];
  count?: number;

  // file_renamed
  oldPath?: string;
  newPath?: string;

  // verification_result
  status?: string;

  // build_test_failed / build_test_error also reuse `message` / `error`

  // Catch-all for any additional fields on events not explicitly modeled
  // above. `unknown` forces call sites to narrow before use, unlike `any`.
  [key: string]: unknown;
}
