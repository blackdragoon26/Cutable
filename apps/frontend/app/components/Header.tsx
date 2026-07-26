import Link from "next/link";
import Brand from "./Brand";
import AuthNav from "./AuthNav";
import ConnectProviderKeys from "./ConnectProviderKeys";
import ThemeToggle from "./ThemeToggle";

export default function Header() {
  return (
    <header className="fixed inset-x-0 top-0 z-50 border-b border-stone-200/90 bg-[#f6f6f3]/90 px-4 py-4 backdrop-blur-xl sm:px-6 lg:px-8">
      <nav className="mx-auto flex max-w-7xl items-center justify-between">
        <Brand />

        <div className="hidden items-center gap-5 md:flex">
          <Link
            href="/docs"
            className="text-sm text-stone-600 transition-colors hover:text-stone-950"
          >
            Architecture
          </Link>
          <a
            href="https://github.com/blackdragoon26/Cutable"
            target="_blank"
            rel="noreferrer"
            className="text-sm text-stone-600 transition-colors hover:text-stone-950"
          >
            GitHub ↗
          </a>
          <a
            href="https://sankalpjha.dev/"
            target="_blank"
            rel="noreferrer"
            className="text-sm text-stone-600 transition-colors hover:text-stone-950"
          >
            By Sankalp ↗
          </a>
        </div>

        <div className="flex items-center gap-2">
          <ThemeToggle />
          <ConnectProviderKeys />
          <AuthNav />
        </div>
      </nav>
    </header>
  );
}
