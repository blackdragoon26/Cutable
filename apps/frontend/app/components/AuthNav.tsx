"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError, getCurrentUser, logout } from "@/app/lib/api";

export default function AuthNav() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const userQuery = useQuery({
    queryKey: ["currentUser"],
    queryFn: getCurrentUser,
    retry: false,
    staleTime: 0,
    refetchOnMount: "always",
    refetchOnWindowFocus: true,
  });
  const logoutMutation = useMutation({
    mutationFn: logout,
    onSuccess: async () => {
      queryClient.removeQueries({ queryKey: ["currentUser"] });
      queryClient.removeQueries({ queryKey: ["projects"] });
      router.push("/");
      router.refresh();
    },
  });

  if (userQuery.isPending) {
    return (
      <span
        aria-label="Checking session"
        className="hidden h-9 w-28 animate-pulse rounded-md bg-stone-200/70 sm:block"
      />
    );
  }

  if (
    userQuery.error instanceof ApiError &&
    userQuery.error.status === 401
  ) {
    return (
      <>
        <Link
          href="/sign-in"
          className="hidden rounded-md px-3 py-2 text-sm text-stone-600 transition-colors hover:bg-white/70 hover:text-stone-950 sm:inline-flex"
        >
          Sign in
        </Link>
        <Link
          href="/sign-up"
          className="hidden rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-stone-700 sm:inline-flex"
        >
          Create account
        </Link>
      </>
    );
  }

  const user = userQuery.data?.user;
  if (!user) return null;

  return (
    <>
      <Link
        href="/dashboard"
        className="hidden max-w-44 truncate rounded-md px-3 py-2 text-sm font-medium text-stone-700 transition-colors hover:bg-white/70 hover:text-stone-950 sm:inline-flex"
        title={user.email}
      >
        Hi, {displayName(user.name, user.email)}
      </Link>
      <button
        type="button"
        onClick={() => logoutMutation.mutate()}
        disabled={logoutMutation.isPending}
        className="hidden rounded-md border border-stone-300 px-3 py-2 text-sm text-stone-600 transition-colors hover:border-stone-400 hover:bg-white/70 hover:text-stone-950 disabled:cursor-wait disabled:opacity-60 sm:inline-flex"
      >
        {logoutMutation.isPending ? "Signing out…" : "Sign out"}
      </button>
    </>
  );
}

function displayName(name: string, email: string) {
  const firstName = name.trim().split(/\s+/)[0];
  return firstName || email.split("@")[0];
}
