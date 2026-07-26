"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import Brand from "@/app/components/Brand";
import ThemeToggle from "@/app/components/ThemeToggle";
import {
  ApiError,
  getProjects,
  type Project,
} from "@/app/lib/api";

export default function DashboardPage() {
  const router = useRouter();
  const projectsQuery = useQuery({
    queryKey: ["projects"],
    queryFn: getProjects,
    retry: (count, error) =>
      !(error instanceof ApiError && error.status === 401) && count < 2,
  });

  useEffect(() => {
    if (
      projectsQuery.error instanceof ApiError &&
      projectsQuery.error.status === 401
    ) {
      router.replace("/sign-in?next=/dashboard");
    }
  }, [projectsQuery.error, router]);

  const projects = projectsQuery.data?.projects ?? [];
  const newestUpdate = projects.reduce<string | null>(
    (latest, project) =>
      !latest || project.updatedAt > latest ? project.updatedAt : latest,
    null
  );

  return (
    <main className="min-h-screen bg-[#f6f6f3] text-stone-950">
      <header className="border-b border-stone-200/90 bg-[#f6f6f3]/90 px-4 py-4 backdrop-blur-xl sm:px-6 lg:px-8">
        <nav className="mx-auto flex max-w-7xl items-center justify-between gap-4">
          <Brand />
          <div className="flex items-center gap-2">
            <ThemeToggle />
            <Link
              href="/"
              className="rounded-md bg-stone-900 px-4 py-2 text-sm font-medium text-white transition hover:bg-stone-700"
            >
              New project
            </Link>
          </div>
        </nav>
      </header>

      <section className="mx-auto max-w-7xl px-4 py-14 sm:px-6 sm:py-20 lg:px-8">
        <div className="grid gap-8 border-b border-stone-200 pb-12 lg:grid-cols-[1fr_auto] lg:items-end">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-[#a74b73]">
              Project library
            </p>
            <h1 className="mt-4 text-4xl font-semibold tracking-[-0.045em] sm:text-6xl">
              Pick up where you left off.
            </h1>
            <p className="mt-5 max-w-2xl text-base leading-7 text-stone-600">
              Every project is tied to your account. Reopen its saved prompt,
              files, conversation, and preview workspace from here.
            </p>
          </div>
          {!projectsQuery.isLoading && !projectsQuery.error && (
            <div className="grid grid-cols-2 gap-6 text-sm">
              <div>
                <p className="text-2xl font-semibold">{projects.length}</p>
                <p className="mt-1 text-xs text-stone-500">Saved projects</p>
              </div>
              <div>
                <p className="text-sm font-medium">
                  {newestUpdate ? formatDate(newestUpdate) : "—"}
                </p>
                <p className="mt-1 text-xs text-stone-500">Last activity</p>
              </div>
            </div>
          )}
        </div>

        {projectsQuery.isLoading && (
          <div
            className="grid gap-4 py-10 md:grid-cols-2 xl:grid-cols-3"
            aria-label="Loading projects"
          >
            {[0, 1, 2].map((item) => (
              <div
                key={item}
                className="h-52 animate-pulse rounded-xl border border-stone-200 bg-white"
              />
            ))}
          </div>
        )}

        {projectsQuery.error &&
          !(
            projectsQuery.error instanceof ApiError &&
            projectsQuery.error.status === 401
          ) && (
            <div
              className="mt-10 rounded-xl border border-red-200 bg-red-50 p-5 text-sm text-red-700"
              role="alert"
            >
              <p>Cutable could not load your projects.</p>
              <button
                type="button"
                onClick={() => projectsQuery.refetch()}
                className="mt-3 font-medium underline underline-offset-4"
              >
                Try again
              </button>
            </div>
          )}

        {!projectsQuery.isLoading &&
          !projectsQuery.error &&
          projects.length === 0 && (
            <div className="mt-10 rounded-2xl border border-dashed border-stone-300 bg-white px-6 py-20 text-center">
              <p className="text-lg font-medium">No projects yet.</p>
              <p className="mx-auto mt-2 max-w-md text-sm leading-6 text-stone-600">
                Start with a prompt or upload a reference. Your first saved
                workspace will appear here.
              </p>
              <Link
                href="/"
                className="mt-6 inline-flex rounded-md bg-stone-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-stone-700"
              >
                Build your first project
              </Link>
            </div>
          )}

        {projects.length > 0 && (
          <div className="grid gap-4 py-10 md:grid-cols-2 xl:grid-cols-3">
            {projects.map((project) => (
              <ProjectCard key={project.id} project={project} />
            ))}
          </div>
        )}
      </section>
    </main>
  );
}

function ProjectCard({ project }: { project: Project }) {
  return (
    <Link
      href={`/projects/${project.id}`}
      className="group flex min-h-56 flex-col rounded-xl border border-stone-200 bg-white p-5 transition hover:-translate-y-0.5 hover:border-stone-400 hover:shadow-[0_18px_45px_rgba(0,0,0,0.12)]"
    >
      <div className="flex items-center justify-between gap-4">
        <span className="rounded-full border border-stone-200 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-[0.12em] text-stone-500">
          Saved workspace
        </span>
        <span className="text-xs text-stone-500">
          {formatDate(project.updatedAt)}
        </span>
      </div>
      <h2 className="mt-6 text-xl font-semibold tracking-[-0.025em] transition group-hover:text-[#e6538b]">
        {project.title}
      </h2>
      <p className="mt-3 line-clamp-3 text-sm leading-6 text-stone-600">
        {project.initialPrompt}
      </p>
      <div className="mt-auto flex items-center justify-between pt-6 text-xs">
        <span className="text-stone-500">
          Created {formatDate(project.createdAt)}
        </span>
        <span className="font-medium text-stone-800">Open project →</span>
      </div>
    </Link>
  );
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat("en", {
    day: "numeric",
    month: "short",
    year: "numeric",
  }).format(new Date(value));
}
