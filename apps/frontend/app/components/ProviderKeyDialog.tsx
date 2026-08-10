"use client";

import { useEffect, useRef, useState } from "react";
import {
  clearProviderCredentials,
  loadProviderCredentials,
  saveProviderCredentials,
  type ProviderCredentials,
} from "@/app/lib/provider-credentials";

interface ProviderKeyDialogProps {
  open: boolean;
  message?: string | null;
  onClose: () => void;
  onSaved: (credentials: ProviderCredentials) => void;
}

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])';

export default function ProviderKeyDialog({
  open,
  message,
  onClose,
  onSaved,
}: ProviderKeyDialogProps) {
  const [openRouterApiKey, setOpenRouterApiKey] = useState("");
  const [e2bApiKey, setE2bApiKey] = useState("");
  const dialogRef = useRef<HTMLElement | null>(null);
  const previouslyFocusedRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    if (!open) return;
    const saved = loadProviderCredentials();
    setOpenRouterApiKey(saved?.openRouterApiKey ?? "");
    setE2bApiKey(saved?.e2bApiKey ?? "");
  }, [open]);

  // Escape-to-close, focus trap, and focus restoration on close.
  useEffect(() => {
    if (!open) return;

    previouslyFocusedRef.current =
      (document.activeElement as HTMLElement | null) ?? null;

    const dialogNode = dialogRef.current;
    const getFocusable = () =>
      dialogNode
        ? Array.from(
            dialogNode.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)
          ).filter((el) => el.offsetParent !== null || el === document.activeElement)
        : [];

    // Move initial focus into the dialog.
    const focusable = getFocusable();
    (focusable[0] ?? dialogNode)?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        onClose();
        return;
      }

      if (event.key !== "Tab") return;

      const elements = getFocusable();
      if (elements.length === 0) {
        event.preventDefault();
        return;
      }

      const first = elements[0];
      const last = elements[elements.length - 1];
      const active = document.activeElement as HTMLElement | null;

      if (event.shiftKey) {
        if (active === first || !dialogNode?.contains(active)) {
          event.preventDefault();
          last.focus();
        }
      } else {
        if (active === last || !dialogNode?.contains(active)) {
          event.preventDefault();
          first.focus();
        }
      }
    };

    document.addEventListener("keydown", handleKeyDown, true);

    return () => {
      document.removeEventListener("keydown", handleKeyDown, true);
      // Restore focus to whatever triggered the dialog.
      previouslyFocusedRef.current?.focus();
    };
  }, [open, onClose]);

  if (!open) return null;

  const canSave =
    openRouterApiKey.trim().length >= 16 && e2bApiKey.trim().length >= 16;

  const handleSave = () => {
    if (!canSave) return;
    const credentials = {
      openRouterApiKey: openRouterApiKey.trim(),
      e2bApiKey: e2bApiKey.trim(),
    };
    saveProviderCredentials(credentials);
    onSaved(credentials);
  };

  const handleClear = () => {
    clearProviderCredentials();
    setOpenRouterApiKey("");
    setE2bApiKey("");
  };

  return (
    <div className="fixed inset-0 z-[100] flex items-center justify-center bg-stone-950/35 px-4 backdrop-blur-sm">
      <section
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="provider-key-title"
        tabIndex={-1}
        className="w-full max-w-lg rounded-2xl border border-stone-200 bg-[#fafaf8] p-6 shadow-2xl outline-none sm:p-7"
      >
        <div className="flex items-start justify-between gap-6">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#47665d]">
              Bring your own providers
            </p>
            <h2
              id="provider-key-title"
              className="mt-2 text-2xl font-semibold tracking-[-0.03em] text-stone-950"
            >
              Continue with your API keys
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md px-2 py-1 text-stone-500 hover:bg-stone-200 hover:text-stone-950"
            aria-label="Close API key settings"
          >
            ×
          </button>
        </div>

        <p className="mt-3 text-sm leading-6 text-stone-600">
          {message ||
            "Every account includes two demo builds. Add your own provider keys for further builds."}
        </p>

        <div className="mt-6 space-y-4">
          <label className="block text-sm font-medium text-stone-700">
            OpenRouter API key
            <input
              type="password"
              autoComplete="off"
              spellCheck={false}
              value={openRouterApiKey}
              onChange={(event) => setOpenRouterApiKey(event.target.value)}
              placeholder="sk-or-…"
              className="mt-2 w-full rounded-lg border border-stone-300 bg-white px-3.5 py-3 font-mono text-sm outline-none focus:border-[#557b6f] focus:ring-2 focus:ring-[#557b6f]/15"
            />
          </label>
          <label className="block text-sm font-medium text-stone-700">
            E2B API key
            <input
              type="password"
              autoComplete="off"
              spellCheck={false}
              value={e2bApiKey}
              onChange={(event) => setE2bApiKey(event.target.value)}
              placeholder="E2B key"
              className="mt-2 w-full rounded-lg border border-stone-300 bg-white px-3.5 py-3 font-mono text-sm outline-none focus:border-[#557b6f] focus:ring-2 focus:ring-[#557b6f]/15"
            />
          </label>
        </div>

        <div className="mt-5 rounded-lg border border-[#cad8d2] bg-[#eef3f0] px-4 py-3 text-xs leading-5 text-[#365047]">
          Keys stay in this browser tab only. Cutable sends them to the Go
          backend for the active build and does not save them in its database.
        </div>

        <div className="mt-6 flex items-center justify-between gap-3">
          <button
            type="button"
            onClick={handleClear}
            className="rounded-md px-3 py-2 text-xs font-medium text-stone-500 hover:bg-stone-200 hover:text-stone-950"
          >
            Clear saved keys
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={!canSave}
            className="rounded-md bg-stone-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-stone-700 disabled:cursor-not-allowed disabled:opacity-40"
          >
            Save for this tab
          </button>
        </div>
      </section>
    </div>
  );
}
