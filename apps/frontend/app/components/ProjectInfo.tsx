"use client";

import { FormEvent, useMemo, useState } from "react";
import type { AgentChatMessage } from "@/app/hooks/useAgentSession";
import AgentThinking from "./AgentThinking";

interface ProjectInfoProps {
  title?: string | null;
  initialPrompt?: string | null;
  attachmentNames?: string[];
  planSteps?: string[];
  recentMessages?: AgentChatMessage[];
  onAskPrompt?: (prompt: string) => void | Promise<void>;
  isAgentConnected?: boolean;
  isProcessing?: boolean;
  isThinking?: boolean;
}

export default function ProjectInfo({
  title,
  initialPrompt,
  attachmentNames = [],
  planSteps = [],
  recentMessages = [],
  onAskPrompt,
  isAgentConnected = false,
  isProcessing = false,
  isThinking = false,
}: ProjectInfoProps) {
  const [askPrompt, setAskPrompt] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);

  const recentActivity = useMemo(() => {
    return [...recentMessages].slice(-4).reverse();
  }, [recentMessages]);

  const handleAskSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const trimmed = askPrompt.trim();
    if (!trimmed || !onAskPrompt) return;
    try {
      setIsSubmitting(true);
      await Promise.resolve(onAskPrompt(trimmed));
      setAskPrompt("");
    } finally {
      setIsSubmitting(false);
    }
  };

  const askInputDisabled = !isAgentConnected || isSubmitting || isProcessing;

  return (
    <div className="flex h-full flex-col overflow-y-auto bg-[#fafaf8]">
      {/* Project Title Bubble */}
      <div className="px-4 pt-6 pb-4">
        <div className="rounded-lg border border-stone-200 bg-white px-4 py-3">
          <p className="mb-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-stone-400">Project brief</p>
          <h1 className="text-sm font-medium leading-relaxed text-stone-800">
            {title || "Untitled Cutable project"}
          </h1>
        </div>
      </div>

      {/* Initial Prompt */}
      <div className="px-4 pb-5">
        <h2 className="text-sm font-bold text-neutral-900 mb-2">
          Initial Prompt
        </h2>
        <p className="text-sm text-neutral-600 leading-relaxed whitespace-pre-wrap">
          {initialPrompt || "No initial prompt saved for this project yet."}
        </p>
      </div>

      {attachmentNames.length > 0 && (
        <div className="px-4 pb-5">
          <h2 className="text-sm font-bold text-neutral-900 mb-2">
            Reference Files
          </h2>
          <ul className="flex flex-wrap gap-2">
            {attachmentNames.map((name) => (
              <li
                key={name}
                className="rounded-full bg-neutral-100 px-3 py-1 text-xs text-neutral-700"
              >
                {name}
              </li>
            ))}
          </ul>
        </div>
      )}

      {/* Plan */}
      <div className="px-4 pb-5">
        <div className="flex items-center justify-between mb-2">
          <h2 className="text-sm font-bold text-neutral-900">Active Plan</h2>
          <span className="text-[11px] text-neutral-500">
            {planSteps.length ? `${planSteps.length} steps` : "No steps yet"}
          </span>
        </div>
        {planSteps.length > 0 ? (
          <ol className="space-y-1.5 list-decimal list-inside text-sm text-neutral-600">
            {planSteps.map((step, index) => (
              <li key={`${step}-${index}`} className="leading-relaxed">
                {step}
              </li>
            ))}
          </ol>
        ) : (
          <p className="text-sm text-neutral-500">
            Ask Cutable to generate a plan to see it here.
          </p>
        )}
      </div>

      {/* Recent Activity */}
      <div className="px-4 pb-6">
        <h2 className="text-sm font-bold text-neutral-900 mb-2">
          Recent Activity
        </h2>
        {isThinking && <div className="mb-3"><AgentThinking /></div>}
        {recentActivity.length > 0 ? (
          <ul className="space-y-2">
            {recentActivity.map((message) => (
              <li
                key={message.id}
                className="text-sm text-neutral-600 border border-neutral-200 rounded-lg px-3 py-2 bg-neutral-50"
              >
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs font-medium text-neutral-500 uppercase">
                    {message.from === "USER" ? "You" : "Cutable"}
                  </span>
                  <span className="text-[11px] text-neutral-400">
                    {new Date(message.createdAt).toLocaleTimeString()}
                  </span>
                </div>
                <p className="leading-relaxed whitespace-pre-wrap">
                  {message.contents}
                </p>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-neutral-500">
            No messages yet. Start the conversation to see updates here.
          </p>
        )}
      </div>

      {/* Bottom Input Area */}
      <div className="mt-auto px-4 py-4 border-t border-neutral-200 bg-white">
        {/* Manual Start Button - if initial prompt exists but hasn't been run */}
        {initialPrompt && !isProcessing && recentMessages.length === 0 && (
          <div className="mb-3">
            <button
              onClick={() => {
                console.log("[ProjectInfo] Start Building button clicked");
                if (initialPrompt && onAskPrompt) {
                  console.log("[ProjectInfo] Calling onAskPrompt with:", initialPrompt.slice(0, 50));
                  onAskPrompt(initialPrompt);
                }
              }}
              disabled={!isAgentConnected}
            className="w-full rounded-lg bg-stone-900 px-3 py-2 text-xs font-medium text-white transition-colors hover:bg-stone-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isAgentConnected ? "Start Building" : "Connecting..."}
            </button>
          </div>
        )}
        <form className="relative" onSubmit={handleAskSubmit}>
          <input
            type="text"
            value={askPrompt}
            onChange={(event) => setAskPrompt(event.target.value)}
            placeholder={
              !isAgentConnected
                ? "Connecting to Cutable…"
                : isProcessing
                  ? "Processing request…"
                  : "Ask Cutable..."
            }
            disabled={askInputDisabled}
            className="w-full rounded-lg border border-stone-300 bg-white px-3 py-2.5 pr-12 text-sm focus:border-[#557b6f] focus:outline-none focus:ring-2 focus:ring-[#557b6f]/15 disabled:cursor-not-allowed disabled:opacity-60"
          />
          <button
            type="submit"
            aria-label="Send request to Cutable"
            disabled={askInputDisabled || !askPrompt.trim()}
            className="absolute right-2 top-1/2 flex h-7 w-7 -translate-y-1/2 items-center justify-center rounded-md bg-stone-900 text-xs font-semibold text-white disabled:bg-stone-300"
          >
            {isSubmitting ? "…" : "↑"}
          </button>
        </form>
      </div>
    </div>
  );
}
