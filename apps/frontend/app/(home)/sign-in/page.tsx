"use client";

import { FormEvent, useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import GoogleAuthButton from "@/app/components/GoogleAuthButton";
import { login } from "@/app/lib/api";

export default function SignInPage() {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const router = useRouter();

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsLoading(true);
    setError(null);
    try {
      await login(email, password);
      const requested = new URLSearchParams(window.location.search).get("next");
      router.push(
        requested?.startsWith("/") && !requested.startsWith("//")
          ? requested
          : "/dashboard"
      );
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : "Unable to sign in.");
    } finally {
      setIsLoading(false);
    }
  };

  const providerError =
    typeof window === "undefined"
      ? null
      : new URLSearchParams(window.location.search).get("error");

  return (
    <main className="grid min-h-screen bg-[#f6f6f3] pt-[73px] lg:grid-cols-[1.05fr_0.95fr]">
      <section className="relative hidden border-r border-stone-200 bg-[#e8ebe5] p-12 lg:flex lg:flex-col lg:justify-center">
        <div className="max-w-lg">
          <p className="mb-5 text-xs font-semibold uppercase tracking-[0.18em] text-[#47665d]">
            Focused build workspace
          </p>
          <h1 className="text-5xl font-semibold leading-[1.04] tracking-[-0.045em] text-stone-950">
            From prompt to working React code, without losing the thread.
          </h1>
          <p className="mt-6 max-w-md text-base leading-7 text-stone-600">
            Your projects, reference files, generated source, and live preview stay together.
          </p>
        </div>
        <p className="absolute bottom-12 text-sm text-stone-500">OpenRouter · E2B · Go</p>
      </section>

      <section className="flex items-center justify-center px-5 py-12 sm:px-10">
        <div className="w-full max-w-sm">
          <h2 className="text-3xl font-semibold tracking-[-0.035em] text-stone-950">Welcome back</h2>
          <p className="mt-2 text-sm leading-6 text-stone-600">Sign in to continue building.</p>

          <div className="mt-8">
            <GoogleAuthButton />
          </div>
          <div className="my-6 flex items-center gap-3 text-xs text-stone-400">
            <span className="h-px flex-1 bg-stone-200" />
            or use email
            <span className="h-px flex-1 bg-stone-200" />
          </div>

          {(error || providerError) && (
            <div className="mb-5 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700" role="alert">
              {error || providerError}
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <Field label="Email" id="email">
              <input
                id="email"
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                placeholder="you@example.com"
                disabled={isLoading}
                required
                className="w-full bg-transparent px-3.5 py-3 text-sm text-stone-900 outline-none placeholder:text-stone-400"
              />
            </Field>
            <Field label="Password" id="password">
              <input
                id="password"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                placeholder="Your password"
                disabled={isLoading}
                required
                className="w-full bg-transparent px-3.5 py-3 text-sm text-stone-900 outline-none placeholder:text-stone-400"
              />
            </Field>
            <button
              type="submit"
              disabled={isLoading}
              className="min-h-12 w-full rounded-lg bg-stone-900 px-4 text-sm font-medium text-white transition-colors hover:bg-stone-700 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isLoading ? "Signing in…" : "Sign in"}
            </button>
          </form>

          <p className="mt-7 text-center text-sm text-stone-600">
            New to Cutable?{" "}
            <Link href="/sign-up" className="font-medium text-stone-950 underline-offset-4 hover:underline">
              Create an account
            </Link>
          </p>
        </div>
      </section>
    </main>
  );
}

function Field({ label, id, children }: { label: string; id: string; children: React.ReactNode }) {
  return (
    <label htmlFor={id} className="block text-sm font-medium text-stone-700">
      <span className="mb-2 block">{label}</span>
      <span className="block rounded-lg border border-stone-300 bg-white shadow-sm transition focus-within:border-[#557b6f] focus-within:ring-2 focus-within:ring-[#557b6f]/15">
        {children}
      </span>
    </label>
  );
}
