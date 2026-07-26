import Link from "next/link";

export default function Header() {
  return (
    <header className="fixed inset-x-0 top-0 z-50 border-b border-stone-200/90 bg-[#f6f6f3]/90 px-4 py-4 backdrop-blur-xl sm:px-6 lg:px-8">
      <nav className="mx-auto flex max-w-7xl items-center justify-between">
        <Link
          href="/"
          className="text-base font-semibold tracking-tight text-stone-950"
        >
          Cutable
        </Link>

        <span className="hidden text-sm text-stone-500 md:block">
          AI workspace for React applications
        </span>

        <div className="flex items-center gap-2">
          <Link
            href="/sign-in"
            className="rounded-md px-3 py-2 text-sm text-stone-600 transition-colors hover:bg-white/70 hover:text-stone-950"
          >
            Sign in
          </Link>
          <Link
            href="/sign-up"
            className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-stone-700"
          >
            Create account
          </Link>
        </div>
      </nav>
    </header>
  );
}
