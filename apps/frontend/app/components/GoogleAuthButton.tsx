"use client";

import { useQuery } from "@tanstack/react-query";
import { getAuthConfig, googleAuthURL } from "@/app/lib/api";

export default function GoogleAuthButton() {
  const { data, isLoading } = useQuery({
    queryKey: ["authConfig"],
    queryFn: getAuthConfig,
    staleTime: 5 * 60 * 1000,
    retry: 1,
  });
  const enabled = data?.google.enabled ?? false;

  const next =
    typeof window === "undefined"
      ? "/"
      : new URLSearchParams(window.location.search).get("next") || "/";

  return (
    <div className="space-y-2">
      <a
        href={enabled ? googleAuthURL(next) : undefined}
        aria-disabled={!enabled || isLoading}
        className={`flex min-h-12 w-full items-center justify-center gap-3 rounded-lg border px-4 text-sm font-medium transition-colors ${
          enabled && !isLoading
            ? "border-stone-300 bg-white text-stone-800 hover:border-stone-400 hover:bg-stone-50"
            : "cursor-not-allowed border-stone-200 bg-stone-50 text-stone-400"
        }`}
      >
        <GoogleMark />
        {isLoading ? "Checking Google sign-in…" : "Continue with Google"}
      </a>
      {!isLoading && !enabled && (
        <p className="text-center text-xs text-stone-500">
          Add Google OAuth credentials to enable this option.
        </p>
      )}
    </div>
  );
}

function GoogleMark() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-5 w-5">
      <path fill="#4285F4" d="M21.6 12.23c0-.71-.06-1.4-.18-2.07H12v3.92h5.38a4.6 4.6 0 0 1-2 3.02v2.54h3.24c1.9-1.75 2.98-4.33 2.98-7.41Z" />
      <path fill="#34A853" d="M12 22c2.7 0 4.98-.9 6.63-2.36l-3.25-2.54c-.9.6-2.05.96-3.38.96-2.61 0-4.82-1.76-5.61-4.13H3.04v2.62A10 10 0 0 0 12 22Z" />
      <path fill="#FBBC05" d="M6.39 13.93A6 6 0 0 1 6.07 12c0-.67.12-1.32.32-1.93V7.45H3.04A10 10 0 0 0 2 12c0 1.61.39 3.14 1.04 4.55l3.35-2.62Z" />
      <path fill="#EA4335" d="M12 5.94c1.47 0 2.79.5 3.83 1.5l2.87-2.87A9.63 9.63 0 0 0 12 2a10 10 0 0 0-8.96 5.45l3.35 2.62C7.18 7.7 9.39 5.94 12 5.94Z" />
    </svg>
  );
}
