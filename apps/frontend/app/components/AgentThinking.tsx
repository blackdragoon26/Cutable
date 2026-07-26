interface AgentThinkingProps {
  label?: string;
}

export default function AgentThinking({ label = "Cutable is reasoning" }: AgentThinkingProps) {
  return (
    <div
      className="overflow-hidden rounded-lg border border-[#cbd9d3] bg-[#eef3f0]"
      role="status"
      aria-live="polite"
    >
      <div className="flex items-center gap-3 px-3.5 py-3">
        <span className="flex h-5 items-end gap-0.5" aria-hidden="true">
          <span className="h-2 w-1 animate-pulse rounded-full bg-[#557b6f] [animation-delay:-300ms]" />
          <span className="h-4 w-1 animate-pulse rounded-full bg-[#557b6f] [animation-delay:-150ms]" />
          <span className="h-3 w-1 animate-pulse rounded-full bg-[#557b6f]" />
        </span>
        <div>
          <p className="text-xs font-semibold text-[#29463e]">{label}</p>
          <p className="mt-0.5 text-[11px] text-[#607a72]">Planning the next action…</p>
        </div>
      </div>
      <div className="h-0.5 w-full overflow-hidden bg-[#d9e5e0]">
        <div className="h-full w-1/3 animate-[thinking-slide_1.4s_ease-in-out_infinite] bg-[#557b6f]" />
      </div>
    </div>
  );
}
