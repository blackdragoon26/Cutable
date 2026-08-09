import HeroBanner from "./HeroBanner";
import PromptInput from "./PromptInput";
import ConnectProviderKeys from "./ConnectProviderKeys";
import MobileDownloadButtons from "./MobileDownloadButtons";

export default function HeroSection() {
  return (
    <section className="w-full px-4 py-14 sm:px-6 sm:py-20 lg:px-8 lg:py-24">
      <div className="mx-auto flex max-w-4xl flex-col items-center gap-9">
        <div className="flex w-full flex-col items-center gap-5">
          <HeroBanner />

          <h1 className="w-full max-w-3xl text-balance text-center text-4xl font-semibold leading-[1.04] tracking-[-0.045em] text-stone-950 sm:text-5xl md:text-6xl">
            Turn a clear idea into a working React application.
          </h1>

          <p className="max-w-2xl text-balance text-center text-base leading-7 text-stone-600 sm:text-lg">
            Describe what you need, add visual or technical references, and review the generated code and live preview in one workspace.
          </p>
        </div>

        <PromptInput />

        <div className="flex flex-col items-center gap-3 text-center sm:flex-row">
          <ConnectProviderKeys variant="hero" />
          <span className="text-xs text-stone-500">
            Session-only storage. Your demo runs stay untouched.
          </span>
        </div>

        <MobileDownloadButtons />
      </div>
    </section>
  );
}
