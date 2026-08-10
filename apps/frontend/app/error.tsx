"use client";

import { useEffect } from "react";
import Link from "next/link";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("Unhandled application error:", error);
  }, [error]);

  return (
    <main className="flex min-h-screen items-center justify-center bg-[#f6f6f3] px-4">
      <section className="w-full max-w-md rounded-2xl border border-stone-200 bg-white p-8 text-center shadow-2xl">
        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-[#e6538b]">
          Something went wrong
        </p>
        <h1 className="mt-3 text-2xl font-semibold tracking-[-0.03em] text-stone-950">
          This page hit a snag
        </h1>
        <p className="mt-3 text-sm leading-6 text-stone-600">
          An unexpected error interrupted this view. You can try again, or
          head back home if the problem persists.
        </p>

        {error.message && (
          <p className="mt-4 rounded-lg border border-stone-200 bg-stone-50 px-3 py-2 text-left font-mono text-xs leading-5 text-stone-500">
            {error.message}
          </p>
        )}

        <div className="mt-6 flex items-center justify-center gap-3">
          <Link
            href="/"
            className="rounded-md px-4 py-2.5 text-sm font-medium text-stone-600 hover:bg-stone-100 hover:text-stone-950"
          >
            Go home
          </Link>
          <button
            type="button"
            onClick={() => reset()}
            className="rounded-md bg-stone-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-stone-700"
          >
            Try again
          </button>
        </div>
      </section>
    </main>
  );
}
