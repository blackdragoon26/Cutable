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

        <div className="hidden items-center gap-6 md:flex">
          <span className="text-sm text-stone-500">
            AI workspace for React applications
          </span>
          <Link
            href="/docs"
            className="text-sm text-stone-600 transition-colors hover:text-stone-950"
          >
            Architecture
          </Link>
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
