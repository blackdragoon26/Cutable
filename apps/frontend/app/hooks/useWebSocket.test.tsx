import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, renderHook } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { useWebSocket } from "./useWebSocket";

const OPEN = 1;
const CONNECTING = 0;
const CLOSING = 2;
const CLOSED = 3;

class MockWebSocket {
  static OPEN = OPEN;
  static CONNECTING = CONNECTING;
  static CLOSING = CLOSING;
  static CLOSED = CLOSED;
  static instances: MockWebSocket[] = [];

  readonly OPEN = OPEN;
  readonly CONNECTING = CONNECTING;
  readonly CLOSING = CLOSING;
  readonly CLOSED = CLOSED;

  url: string;
  readyState = CONNECTING;
  sentMessages: string[] = [];
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onerror: ((event: unknown) => void) | null = null;
  onclose: ((event: { code: number; reason: string }) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  send(data: string) {
    this.sentMessages.push(data);
  }

  close(code = 1000, reason = "") {
    this.readyState = CLOSED;
    this.onclose?.({ code, reason });
  }

  // Test helper: simulate the server accepting the connection.
  triggerOpen() {
    this.readyState = OPEN;
    this.onopen?.();
  }

  // Test helper: simulate the server / network closing the connection
  // without the client having called close() itself.
  triggerServerClose(code: number, reason = "") {
    this.readyState = CLOSED;
    this.onclose?.({ code, reason });
  }
}

function wrapper({ children }: { children: ReactNode }) {
  const queryClient = new QueryClient();
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useWebSocket", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    MockWebSocket.instances = [];
    // @ts-expect-error - test mock does not implement the full WebSocket API
    global.WebSocket = MockWebSocket;
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  async function mountAndConnect() {
    const view = renderHook(
      () => useWebSocket({ projectId: "project-1" }),
      { wrapper }
    );
    // The hook delays the initial connect by a 0ms timeout.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(0);
    });
    const instance = MockWebSocket.instances[0];
    await act(async () => {
      instance.triggerOpen();
    });
    return { ...view, instance };
  }

  it("connects and starts a ping interval that sends periodic pings", async () => {
    const { instance, result } = await mountAndConnect();

    expect(result.current.isConnected).toBe(true);
    expect(instance.sentMessages).toHaveLength(0);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(instance.sentMessages).toHaveLength(1);
    expect(JSON.parse(instance.sentMessages[0])).toEqual({ type: "ping" });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(30000);
    });
    expect(instance.sentMessages).toHaveLength(2);
  });

  it("reconnects with exponential backoff on an abnormal close", async () => {
    const { instance: first } = await mountAndConnect();

    expect(MockWebSocket.instances).toHaveLength(1);

    // First abnormal close -> reconnect after 1000ms (2^0 * 1000).
    await act(async () => {
      first.triggerServerClose(1006, "abnormal");
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(999);
    });
    expect(MockWebSocket.instances).toHaveLength(1);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(MockWebSocket.instances).toHaveLength(2);

    // Second abnormal close -> reconnect after 2000ms (2^1 * 1000).
    const second = MockWebSocket.instances[1];
    await act(async () => {
      second.triggerServerClose(1006, "abnormal");
    });
    await act(async () => {
      await vi.advanceTimersByTimeAsync(1999);
    });
    expect(MockWebSocket.instances).toHaveLength(2);

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1);
    });
    expect(MockWebSocket.instances).toHaveLength(3);
  });

  it.each([4000, 4001, 4003])(
    "does not reconnect after close code %d",
    async (code) => {
      const { instance, result } = await mountAndConnect();

      await act(async () => {
        instance.triggerServerClose(code, "no reconnect");
      });

      await act(async () => {
        await vi.advanceTimersByTimeAsync(60000);
      });

      // No new WebSocket should have been created for this close code.
      expect(MockWebSocket.instances).toHaveLength(1);
      expect(result.current.isConnected).toBe(false);
      expect(result.current.connectionError).toBeTruthy();
    }
  );

  it("cleans up the socket, ping interval, and any pending reconnect on unmount", async () => {
    const { instance, unmount } = await mountAndConnect();

    // Schedule a reconnect so we can prove the pending timer is cancelled too.
    await act(async () => {
      instance.triggerServerClose(1006, "abnormal");
    });

    unmount();

    // Because the hook only calls disconnect() for OPEN/CONNECTING sockets,
    // and the socket was already CLOSED by the server-close above, closing
    // is a no-op here — what we're verifying is that no further reconnect
    // attempts fire after unmount.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(60000);
    });
    expect(MockWebSocket.instances).toHaveLength(1);
  });

  it("closes an actively open socket on unmount", async () => {
    const { instance, unmount } = await mountAndConnect();
    expect(instance.readyState).toBe(OPEN);

    unmount();

    expect(instance.readyState).toBe(CLOSED);
  });
});
