export default function HeroBanner() {
  return (
    <div className="flex max-w-fit items-center gap-2 rounded-full border border-stone-200 bg-white/70 px-3 py-1.5 shadow-sm">
      <span className="h-1.5 w-1.5 rounded-full bg-[#557b6f]" />
      <p className="text-xs font-medium text-stone-600 sm:text-sm">
        Private E2B workspace · Your OpenRouter model
      </p>
    </div>
  );
}
