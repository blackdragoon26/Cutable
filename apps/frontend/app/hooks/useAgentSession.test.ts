import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { useAgentSession } from "./useAgentSession";

vi.mock("@/app/lib/api", () => ({
  createConversationMessage: vi.fn().mockResolvedValue({ message: "ok" }),
}));

describe("useAgentSession", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("stage_update produces no chat message but updates stage state", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    await act(async () => {
      await result.current.handleAgentEvent({
        e: "stage_update",
        stage: "planning",
        message: "Creating a plan",
        progress: 25,
        currentStep: 1,
        totalSteps: 4,
      });
    });

    expect(result.current.messages).toHaveLength(0);
    expect(result.current.stageInfo).toEqual({
      stage: "planning",
      message: "Creating a plan",
      progress: 25,
      currentStep: 1,
      totalSteps: 4,
    });
    expect(result.current.stageLabel).toBe("Creating Plan");
  });

  it("stage_update with stage 'complete' clears processing/thinking flags", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    await act(async () => {
      await result.current.sendUserMessage("build me a todo app");
    });
    expect(result.current.isProcessing).toBe(true);

    await act(async () => {
      await result.current.handleAgentEvent({
        e: "stage_update",
        stage: "complete",
        message: "Done",
        progress: 100,
      });
    });

    expect(result.current.isProcessing).toBe(false);
    expect(result.current.isThinking).toBe(false);
  });

  it("tool_started/tool_completed only produce TOOL_CALL messages for visible tools", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    // "write-file" maps to CREATE_FILE, which is in the visible set.
    await act(async () => {
      await result.current.handleAgentEvent({
        e: "tool_started",
        tool: "write-file",
        input: { path: "src/App.tsx" },
      });
    });
    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]).toMatchObject({
      type: "TOOL_CALL",
      from: "AGENT",
      contents: "Running: write file...",
    });

    await act(async () => {
      await result.current.handleAgentEvent({
        e: "tool_completed",
        tool: "write-file",
        output: { ok: true },
      });
    });
    expect(result.current.messages).toHaveLength(2);
    expect(result.current.messages[1]).toMatchObject({
      type: "TOOL_CALL",
      contents: "✓ write file completed",
    });

    // "get_context" maps to GET_CONTEXT, which is NOT in the visible set,
    // so no message should be appended for either event.
    await act(async () => {
      await result.current.handleAgentEvent({
        e: "tool_started",
        tool: "get_context",
      });
    });
    await act(async () => {
      await result.current.handleAgentEvent({
        e: "tool_completed",
        tool: "get_context",
      });
    });
    expect(result.current.messages).toHaveLength(2);

    // A tool name the frontend doesn't know about at all should also be
    // silently dropped rather than throwing.
    await act(async () => {
      await result.current.handleAgentEvent({
        e: "tool_started",
        tool: "some-unmapped-tool",
      });
    });
    expect(result.current.messages).toHaveLength(2);
  });

  it("agent_response produces a TEXT_MESSAGE", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    await act(async () => {
      await result.current.handleAgentEvent({
        e: "agent_response",
        message: "Here is your app!",
      });
    });

    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]).toMatchObject({
      type: "TEXT_MESSAGE",
      from: "AGENT",
      contents: "Here is your app!",
    });
  });

  it("agent_response with no message produces no chat message", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    await act(async () => {
      await result.current.handleAgentEvent({ e: "agent_response" });
    });

    expect(result.current.messages).toHaveLength(0);
  });

  it("tool_error produces an ERROR_MESSAGE", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    await act(async () => {
      await result.current.handleAgentEvent({
        e: "tool_error",
        tool: "execute-command",
        error: "command not found",
      });
    });

    expect(result.current.messages).toHaveLength(1);
    expect(result.current.messages[0]).toMatchObject({
      type: "ERROR_MESSAGE",
      contents: "✗ execute-command failed: command not found",
    });
  });

  it("ignores unknown event types gracefully", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    await act(async () => {
      await result.current.handleAgentEvent({
        e: "some_future_event_type",
        payload: { anything: true },
      });
    });

    expect(result.current.messages).toHaveLength(0);
    expect(result.current.isProcessing).toBe(false);
  });

  it("plan_generated updates planSteps and appends a formatted TEXT_MESSAGE", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    await act(async () => {
      await result.current.handleAgentEvent({
        e: "plan_generated",
        plan: ["Set up project", "Add components"],
      });
    });

    expect(result.current.planSteps).toEqual([
      "Set up project",
      "Add components",
    ]);
    expect(result.current.messages[0].contents).toBe(
      "Plan generated:\n1. Set up project\n2. Add components"
    );
  });

  it("clearMessages resets all session state", async () => {
    const { result } = renderHook(() => useAgentSession("project-1"));

    await act(async () => {
      await result.current.handleAgentEvent({
        e: "agent_response",
        message: "hi",
      });
    });
    expect(result.current.messages).toHaveLength(1);

    act(() => {
      result.current.clearMessages();
    });

    expect(result.current.messages).toHaveLength(0);
    expect(result.current.planSteps).toHaveLength(0);
    expect(result.current.stageInfo).toBeNull();
    expect(result.current.isProcessing).toBe(false);
    expect(result.current.isThinking).toBe(false);
  });
});
