import type { Metadata } from "next";
import Image from "next/image";
import Link from "next/link";
import Header from "../components/Header";

export const metadata: Metadata = {
  title: "Architecture — Cutable",
  description:
    "Code-backed system diagrams for Cutable's user flow, AI execution loop, data boundaries, and production delivery.",
};

const repository = "https://github.com/blackdragoon26/Cutable/blob/main";

const diagrams = [
  {
    number: "01",
    eyebrow: "Request path",
    title: "System at a glance",
    copy: "Next.js is the interface. Go owns policy and orchestration. Neon is durable state. OpenRouter proposes work. E2B executes generated applications.",
    src: "/docs/system-context.svg",
    alt: "Cutable system context from browser through Vercel and the Go API to Neon, OpenRouter, and E2B",
  },
  {
    number: "02",
    eyebrow: "Product path",
    title: "One user build",
    copy: "An authenticated, owner-scoped project moves through an atomic demo-or-BYOK gate, then streams agent events and an isolated preview back to the builder.",
    src: "/docs/user-build-flow.svg",
    alt: "Cutable user flow from sign in and prompt through the run gate, agent, preview, and iteration",
  },
  {
    number: "03",
    eyebrow: "Control loop",
    title: "Model proposes. Go controls. E2B executes.",
    copy: "The model returns one tool call at a time. Go constrains paths, dispatches registered tools, persists changed files, and returns results for the next decision.",
    src: "/docs/ai-agent-loop.svg",
    alt: "Cutable AI agent loop showing OpenRouter decisions, Go tool execution, E2B operations, and Neon persistence",
  },
  {
    number: "04",
    eyebrow: "Production path",
    title: "Trust and delivery",
    copy: "The runtime and delivery boundaries are explicit: browser session state, Vercel edge, Myprod control plane, Neon data plane, and digest-pinned container delivery.",
    src: "/docs/trust-and-delivery.svg",
    alt: "Cutable trust boundaries and deployment path through GitHub, GHCR, Myprod, Nomad, and Traefik",
  },
];

const evidence = [
  ["Owner-scoped access", "apps/backend/internal/store/store.go", "Store.Project includes user_id in the query."],
  ["Atomic demo claim", "apps/backend/internal/store/store.go", "One conditional UPDATE increments usage and returns the claim."],
  ["Session-only BYOK", "apps/backend/internal/httpapi/websocket.go", "Per-run providers are built in the socket handler; no store call receives keys."],
  ["React-root file paths", "apps/backend/internal/agent/agent.go", "safePath rejects traversal outside /home/user/react-app."],
  ["Isolated execution", "apps/backend/internal/provider/e2b.go", "Filesystem and process calls target the project’s E2B sandbox."],
  ["Serial model tools", "apps/backend/internal/provider/openrouter.go", "parallel_tool_calls is disabled."],
  ["Nonroot backend", "apps/backend/Dockerfile", "The final Distroless image runs as nonroot."],
  ["Immutable delivery", ".github/workflows/publish-backend.yml", "Buildx publishes commit tags and records the GHCR digest."],
];

export default function DocsPage() {
  return (
    <main className="min-h-screen bg-[#f6f6f3] text-stone-950">
      <Header />

      <section className="border-b border-stone-200 px-4 pb-20 pt-36 sm:px-6 lg:px-8">
        <div className="mx-auto max-w-7xl">
          <div className="mb-8 flex items-center gap-3 font-mono text-xs uppercase tracking-[0.2em] text-[#2f5e50]">
            <span className="h-px w-10 bg-[#2f5e50]" />
            Architecture handbook
          </div>
          <div className="grid gap-12 lg:grid-cols-[1fr_0.7fr] lg:items-end">
            <h1 className="max-w-4xl text-5xl font-semibold leading-[0.98] tracking-[-0.055em] sm:text-7xl">
              Start abstract.
              <br />
              Follow every arrow.
            </h1>
            <div>
              <p className="max-w-xl text-lg leading-8 text-stone-600">
                Four code-backed views of the same production system. No future-state boxes.
                No invisible “AI magic” layer.
              </p>
              <div className="mt-7 flex flex-wrap gap-3">
                <a
                  href={`${repository}/docs/architecture/README.md`}
                  target="_blank"
                  rel="noreferrer"
                  className="rounded-md bg-stone-900 px-4 py-2.5 text-sm font-medium text-white hover:bg-stone-700"
                >
                  Read repository handbook ↗
                </a>
                <Link
                  href="/"
                  className="rounded-md border border-stone-300 bg-white px-4 py-2.5 text-sm font-medium text-stone-700 hover:border-stone-400"
                >
                  Build with Cutable
                </Link>
              </div>
            </div>
          </div>
        </div>
      </section>

      {diagrams.map((diagram, index) => (
        <section
          key={diagram.number}
          className={`px-4 py-20 sm:px-6 lg:px-8 ${index % 2 ? "bg-white" : "bg-[#f6f6f3]"}`}
        >
          <div className="mx-auto max-w-7xl">
            <div className="mb-10 grid gap-6 md:grid-cols-[110px_1fr_0.8fr] md:items-end">
              <span className="font-mono text-4xl text-stone-300">{diagram.number}</span>
              <div>
                <p className="mb-2 font-mono text-xs uppercase tracking-[0.18em] text-[#2f5e50]">
                  {diagram.eyebrow}
                </p>
                <h2 className="text-3xl font-semibold tracking-[-0.035em] sm:text-4xl">
                  {diagram.title}
                </h2>
              </div>
              <p className="text-sm leading-6 text-stone-600">{diagram.copy}</p>
            </div>
            <div className="overflow-hidden rounded-2xl border border-stone-300 bg-[#f7f5ef] shadow-[0_24px_70px_rgba(31,41,37,0.08)]">
              <Image
                src={diagram.src}
                alt={diagram.alt}
                width={1600}
                height={900}
                className="h-auto w-full"
                priority={index === 0}
              />
            </div>
          </div>
        </section>
      ))}

      <section className="border-y border-stone-200 bg-[#17211e] px-4 py-20 text-white sm:px-6 lg:px-8">
        <div className="mx-auto max-w-7xl">
          <div className="grid gap-10 lg:grid-cols-[0.55fr_1fr]">
            <div>
              <p className="mb-3 font-mono text-xs uppercase tracking-[0.18em] text-[#b9d0c7]">
                Verification map
              </p>
              <h2 className="text-4xl font-semibold tracking-[-0.04em]">
                Claims resolve to code.
              </h2>
              <p className="mt-5 max-w-md text-sm leading-6 text-stone-300">
                These links are the evidence behind the diagrams. Follow them when behavior
                changes; regenerate the views when the architecture changes.
              </p>
            </div>
            <div className="divide-y divide-white/10 border-y border-white/10">
              {evidence.map(([claim, file, detail]) => (
                <a
                  key={claim}
                  href={`${repository}/${file}`}
                  target="_blank"
                  rel="noreferrer"
                  className="grid gap-2 py-4 transition-colors hover:bg-white/[0.03] sm:grid-cols-[0.55fr_1fr_auto] sm:items-center sm:gap-5 sm:px-3"
                >
                  <span className="text-sm font-medium">{claim}</span>
                  <span className="font-mono text-xs text-stone-400">{file}</span>
                  <span className="hidden max-w-sm text-xs leading-5 text-stone-300 xl:block">
                    {detail}
                  </span>
                </a>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section className="px-4 py-20 sm:px-6 lg:px-8">
        <div className="mx-auto grid max-w-7xl gap-8 lg:grid-cols-3">
          <div>
            <p className="font-mono text-xs uppercase tracking-[0.18em] text-[#2f5e50]">
              Honest boundaries
            </p>
            <h2 className="mt-3 text-3xl font-semibold tracking-[-0.035em]">
              What the boxes do not promise.
            </h2>
          </div>
          <ul className="space-y-4 text-sm leading-6 text-stone-600">
            <li>OpenRouter and E2B are third-party processors for inference and execution.</li>
            <li>The two-run demo is an account allowance, not a token or compute budget.</li>
            <li>Preview availability depends on E2B lifetime and a successful Vite start.</li>
          </ul>
          <ul className="space-y-4 text-sm leading-6 text-stone-600">
            <li>Generated code still requires review before a real deployment.</li>
            <li>Neon persists current files; Cutable does not provide Git history per project.</li>
            <li>BYOK is session-only handling, not end-to-end provider encryption.</li>
          </ul>
        </div>
      </section>

      <footer className="border-t border-stone-200 px-4 py-8 sm:px-6 lg:px-8">
        <div className="mx-auto flex max-w-7xl flex-col gap-3 text-xs text-stone-500 sm:flex-row sm:items-center sm:justify-between">
          <span>Cutable architecture / generated from repository truth</span>
          <span>Editable Excalidraw · SVG · PNG sources are committed with the code</span>
        </div>
      </footer>
    </main>
  );
}
