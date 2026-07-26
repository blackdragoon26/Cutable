"use client";

import type { StageInfo } from "@/app/hooks/useAgentSession";

interface ProgressLoaderProps {
  stageInfo: StageInfo | null;
  stageLabel: string | null;
  isProcessing: boolean;
}

export default function ProgressLoader({ stageInfo, stageLabel, isProcessing }: ProgressLoaderProps) {
  if (!isProcessing && !stageInfo) return null;

  const progress = stageInfo?.progress ?? 0;
  const stage = stageInfo?.stage ?? "initializing";
  const isComplete = stage === "complete";
  const isError = stage === "error";
  const tone = isError
    ? "border-red-200 bg-red-50 text-red-800"
    : isComplete
      ? "border-[#cbd9d3] bg-[#eef3f0] text-[#29463e]"
      : "border-stone-200 bg-white text-stone-800";
  const bar = isError ? "bg-red-500" : "bg-[#557b6f]";

  return (
    <div className={`rounded-lg border px-4 py-3 ${tone}`} role="status">
      <div className="mb-2.5 flex items-center justify-between gap-4">
        <div className="flex min-w-0 items-center gap-2.5">
          <span className={`h-2 w-2 shrink-0 rounded-full ${bar} ${isProcessing && !isComplete && !isError ? "animate-pulse" : ""}`} />
          <div className="min-w-0">
            <p className="truncate text-xs font-semibold uppercase tracking-[0.08em]">
              {stageLabel || "Preparing"}
            </p>
            <p className="mt-0.5 truncate text-xs opacity-70">{stageInfo?.message || "Starting workspace"}</p>
          </div>
        </div>
        <span className="font-mono text-xs tabular-nums opacity-70">{progress}%</span>
      </div>
      <div className="h-1 overflow-hidden rounded-full bg-black/10">
        <div className={`h-full rounded-full transition-[width] duration-500 ${bar}`} style={{ width: `${progress}%` }} />
      </div>
    </div>
  );
}
