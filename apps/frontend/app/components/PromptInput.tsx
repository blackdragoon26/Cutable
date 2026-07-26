"use client";

import { useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import {
  ApiError,
  createProject,
  type ProjectAttachmentInput,
} from "@/app/lib/api";

const DRAFT_KEY = "cutable-project-draft";
const MAX_FILES = 3;
const MAX_TEXT_FILE_BYTES = 100 * 1024;
const MAX_TEXT_TOTAL_BYTES = 200 * 1024;
const MAX_IMAGE_FILE_BYTES = 2 * 1024 * 1024;
const MAX_IMAGE_TOTAL_BYTES = 3 * 1024 * 1024;
const ACCEPTED_EXTENSIONS = new Set([
  "css", "csv", "go", "html", "java", "js", "json", "jsx", "md", "py",
  "rs", "sql", "ts", "tsx", "txt", "xml", "yaml", "yml",
  "jpeg", "jpg", "png", "webp",
]);
const IMAGE_TYPES: Record<string, string> = {
  jpeg: "image/jpeg",
  jpg: "image/jpeg",
  png: "image/png",
  webp: "image/webp",
};

interface ProjectDraft {
  prompt: string;
  attachments: ProjectAttachmentInput[];
}

export default function PromptInput() {
  const [prompt, setPrompt] = useState("");
  const [attachments, setAttachments] = useState<ProjectAttachmentInput[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const draftReadyRef = useRef(false);
  const router = useRouter();

  useEffect(() => {
    const saved = sessionStorage.getItem(DRAFT_KEY);
    if (!saved) {
      draftReadyRef.current = true;
      return;
    }
    try {
      const draft = JSON.parse(saved) as ProjectDraft;
      setPrompt(draft.prompt || "");
      setAttachments(
        Array.isArray(draft.attachments)
          ? draft.attachments.map((file) => ({
              ...file,
              kind: file.kind || (file.mimeType?.startsWith("image/") ? "image" : "text"),
            }))
          : []
      );
    } catch {
      sessionStorage.removeItem(DRAFT_KEY);
    } finally {
      draftReadyRef.current = true;
    }
  }, []);

  useEffect(() => {
    if (!draftReadyRef.current) return;
    if (!prompt && attachments.length === 0) {
      sessionStorage.removeItem(DRAFT_KEY);
      return;
    }
    sessionStorage.setItem(
      DRAFT_KEY,
      JSON.stringify({ prompt, attachments } satisfies ProjectDraft)
    );
  }, [prompt, attachments]);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!prompt.trim() || isLoading) return;

    setIsLoading(true);
    setError(null);

    try {
      const title = prompt.length > 50 ? `${prompt.substring(0, 50)}...` : prompt;
      const response = await createProject(title, prompt, attachments);

      if (response.project?.id) {
        sessionStorage.removeItem(DRAFT_KEY);
        router.push(`/projects/${response.project.id}`);
      } else {
        setError("Failed to create project. Please try again.");
      }
    } catch (caught) {
      if (caught instanceof ApiError && caught.status === 401) {
        router.push("/sign-in?next=/");
        return;
      }
      const message =
        caught instanceof Error ? caught.message : "Failed to create project.";
      setError(message);
    } finally {
      setIsLoading(false);
    }
  };

  const handleFiles = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const selected = Array.from(event.target.files ?? []);
    event.target.value = "";
    if (selected.length === 0) return;

    const remainingSlots = MAX_FILES - attachments.length;
    if (selected.length > remainingSlots) {
      setError(`You can attach up to ${MAX_FILES} files.`);
      return;
    }

    const existingNames = new Set(attachments.map((file) => file.name.toLowerCase()));
    const next: ProjectAttachmentInput[] = [];
    let textTotalBytes = attachments
      .filter((file) => file.kind === "text")
      .reduce((sum, file) => sum + file.size, 0);
    let imageTotalBytes = attachments
      .filter((file) => file.kind === "image")
      .reduce((sum, file) => sum + file.size, 0);

    for (const file of selected) {
      const extension = file.name.split(".").pop()?.toLowerCase() ?? "";
      if (!ACCEPTED_EXTENSIONS.has(extension)) {
        setError(`${file.name} is not a supported text or source file.`);
        return;
      }
      if (existingNames.has(file.name.toLowerCase())) {
        setError(`${file.name} is already attached.`);
        return;
      }
      const expectedImageType = IMAGE_TYPES[extension];
      if (expectedImageType) {
        if (file.type !== expectedImageType) {
          setError(`${file.name} does not match its image extension.`);
          return;
        }
        if (file.size === 0 || file.size > MAX_IMAGE_FILE_BYTES) {
          setError(`${file.name} must be between 1 byte and 2 MB.`);
          return;
        }
        imageTotalBytes += file.size;
        if (imageTotalBytes > MAX_IMAGE_TOTAL_BYTES) {
          setError("Images may not exceed 3 MB in total.");
          return;
        }
        next.push({
          name: file.name,
          kind: "image",
          mimeType: expectedImageType,
          content: await readAsDataURL(file),
          size: file.size,
        });
      } else {
        if (file.size === 0 || file.size > MAX_TEXT_FILE_BYTES) {
          setError(`${file.name} must be between 1 byte and 100 KB.`);
          return;
        }
        const content = await file.text();
        if (content.includes("\u0000")) {
          setError(`${file.name} is not a valid text file.`);
          return;
        }
        const size = new TextEncoder().encode(content).length;
        textTotalBytes += size;
        if (textTotalBytes > MAX_TEXT_TOTAL_BYTES) {
          setError("Text attachments may not exceed 200 KB in total.");
          return;
        }
        next.push({
          name: file.name,
          kind: "text",
          mimeType: file.type || "text/plain",
          content,
          size,
        });
      }
      existingNames.add(file.name.toLowerCase());
    }

    setAttachments((current) => [...current, ...next]);
    setError(null);
  };

  const removeAttachment = (name: string) => {
    setAttachments((current) => current.filter((file) => file.name !== name));
  };

  return (
    <form onSubmit={handleSubmit} className="w-full max-w-2xl">
      <div className="relative min-h-40 w-full rounded-xl border border-stone-300 bg-white p-4 pb-14 shadow-[0_14px_40px_rgba(38,38,32,0.08)] transition focus-within:border-[#78958c] focus-within:ring-4 focus-within:ring-[#557b6f]/10">
        <textarea
          value={prompt}
          onChange={(event) => setPrompt(event.target.value)}
          placeholder="Describe the application you want to build…"
          rows={3}
          className="w-full resize-none bg-transparent text-sm leading-6 text-stone-800 outline-none placeholder:text-stone-400 sm:text-base"
          aria-label="Enter your prompt"
          disabled={isLoading}
        />

        {attachments.length > 0 && (
          <div className="mt-4 flex flex-wrap gap-2" aria-label="Attached files">
            {attachments.map((file) => (
              <span
                key={file.name}
                className="inline-flex items-center gap-2 rounded-md border border-stone-200 bg-stone-50 px-2.5 py-1.5 text-xs text-stone-700"
              >
                {file.kind === "image" && (
                  <img
                    src={file.content}
                    alt=""
                    className="h-6 w-6 rounded object-cover"
                  />
                )}
                {file.name}
                <button
                  type="button"
                  onClick={() => removeAttachment(file.name)}
                  className="font-semibold hover:text-red-600"
                  aria-label={`Remove ${file.name}`}
                  disabled={isLoading}
                >
                  ×
                </button>
              </span>
            ))}
          </div>
        )}

        <input
          ref={fileInputRef}
          type="file"
          multiple
          accept=".css,.csv,.go,.html,.java,.js,.json,.jsx,.md,.py,.rs,.sql,.ts,.tsx,.txt,.xml,.yaml,.yml,.jpeg,.jpg,.png,.webp"
          onChange={handleFiles}
          className="sr-only"
          aria-label="Choose text or source files"
        />

        <div className="absolute bottom-3 left-3.5 right-3.5 flex items-center justify-between">
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="rounded-md px-2 py-1.5 text-xs font-medium text-stone-600 transition-colors hover:bg-stone-100 hover:text-stone-950 disabled:cursor-not-allowed disabled:opacity-50"
            disabled={isLoading || attachments.length >= MAX_FILES}
          >
            Attach files or images{attachments.length > 0 ? ` (${attachments.length}/${MAX_FILES})` : ""}
          </button>

          <button
            type="submit"
            disabled={!prompt.trim() || isLoading}
            className="rounded-md bg-stone-900 px-4 py-2 text-xs font-medium text-white transition-colors hover:bg-stone-700 disabled:cursor-not-allowed disabled:opacity-50"
            aria-label="Send message"
          >
            {isLoading ? "Creating..." : "Build"}
          </button>
        </div>
      </div>

      <p className="mt-3 text-center text-xs text-stone-500">
        Two demo builds are included with each account. Continue afterward with
        your own OpenRouter and E2B keys.
      </p>
      {error && (
        <p className="mt-2 text-center text-sm text-red-600" role="alert">
          {error}
        </p>
      )}
    </form>
  );
}

function readAsDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () =>
      typeof reader.result === "string"
        ? resolve(reader.result)
        : reject(new Error(`Could not read ${file.name}.`));
    reader.onerror = () => reject(new Error(`Could not read ${file.name}.`));
    reader.readAsDataURL(file);
  });
}
