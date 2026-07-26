"use client";

import { useState, useEffect, forwardRef, useImperativeHandle } from "react";
import dynamic from "next/dynamic";
import { useFileContent } from "@/app/hooks/useProjectQueries";

// Dynamically import Monaco Editor to avoid SSR issues
const MonacoEditor = dynamic(() => import("@monaco-editor/react"), {
  ssr: false,
  loading: () => (
    <div className="flex items-center justify-center h-full bg-neutral-50">
      <div className="text-neutral-500">Loading editor...</div>
    </div>
  ),
});

interface OpenFile {
  path: string;
  code: string;
  language?: string;
}

interface EditorSectionProps {
  projectId: string;
  fileContents?: Record<string, string>;
  initialFile?: OpenFile;
  onFileSelect?: (filePath: string) => void;
  onActiveFileChange?: (filePath: string) => void;
  sandboxPreviewUrl?: string | null;
}

export interface EditorSectionRef {
  openFile: (filePath: string) => void;
}

const defaultFile: OpenFile = {
  path: "src/App.tsx",
  code: "// Select a generated file to inspect its source.\n",
  language: "typescript",
};

const EditorSection = forwardRef<EditorSectionRef, EditorSectionProps>(
  function EditorSection(
    {
      projectId,
      fileContents = {},
      initialFile = defaultFile,
      onFileSelect,
      onActiveFileChange,
      sandboxPreviewUrl,
    },
    ref
  ) {
    // Use fileContents if available for initial file, otherwise use initialFile prop
    const initialFileCode =
      fileContents[initialFile.path] || initialFile.code;
    const initialFileWithContent: OpenFile = {
      ...initialFile,
      code: initialFileCode,
    };

    const [activeViewTab, setActiveViewTab] = useState<"code" | "preview">("code");
    const [openFiles, setOpenFiles] = useState<OpenFile[]>([
      initialFileWithContent,
    ]);
    const [activeFile, setActiveFile] = useState<string>(initialFile.path);
    const [fileContent, setFileContent] = useState<Map<string, string>>(
      new Map([[initialFile.path, initialFileCode]])
    );
    const [previewHtml, setPreviewHtml] = useState("");
    const [hasOpenedPreview, setHasOpenedPreview] = useState(false);

    useEffect(() => {
      if (sandboxPreviewUrl && !hasOpenedPreview) {
        setActiveViewTab("preview");
        setHasOpenedPreview(true);
      }
    }, [sandboxPreviewUrl, hasOpenedPreview]);

    // Load file content for active file using React Query
    const { data: activeFileData } = useFileContent(projectId, activeFile);

    // Update file content when React Query data changes
    useEffect(() => {
      if (activeFileData?.content && activeFile) {
        setFileContent((prev) => {
          const newMap = new Map(prev);
          newMap.set(activeFile, activeFileData.content);
          return newMap;
        });
        // Update open file content
        setOpenFiles((prev) =>
          prev.map((f) =>
            f.path === activeFile
              ? { ...f, code: activeFileData.content }
              : f
          )
        );
      }
    }, [activeFileData, activeFile]);

    // Expose openFile method via ref
    useImperativeHandle(ref, () => ({
      openFile: (filePath: string) => {
        // Check if file is already open
        const existingFile = openFiles.find((f) => f.path === filePath);
        if (existingFile) {
          setActiveFile(filePath);
          return;
        }

        // Get file content from fileContents prop, existing content, or use placeholder
        // React Query will fetch the actual content when activeFile changes
        const code =
          fileContents[filePath] ||
          fileContent.get(filePath) ||
          `// Loading ${filePath}...\n\n`;

        // Create new file object
        const newFile: OpenFile = {
          path: filePath,
          code,
        };

        // Add to open files and set as active
        setOpenFiles((prev) => [...prev, newFile]);
        setFileContent((prev) => {
          const newMap = new Map(prev);
          newMap.set(filePath, code);
          return newMap;
        });
        setActiveFile(filePath);
        onFileSelect?.(filePath);
        // Switch to code tab when opening a file
        setActiveViewTab("code");
        // Notify parent of active file change
        onActiveFileChange?.(filePath);
      },
    }));

    // Notify parent when active file changes (from tab click)
    useEffect(() => {
      onActiveFileChange?.(activeFile);
    }, [activeFile, onActiveFileChange]);

    const currentCode = fileContent.get(activeFile) || "";

  const handleEditorChange = (value: string | undefined) => {
    setFileContent((prev) => {
      const newMap = new Map(prev);
      newMap.set(activeFile, value || "");
      return newMap;
    });
  };

  const closeFile = (path: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (openFiles.length === 1) return; // Don't close last file

    setOpenFiles((prev) => prev.filter((f) => f.path !== path));
    if (activeFile === path) {
      const remaining = openFiles.filter((f) => f.path !== path);
      setActiveFile(remaining[0]?.path || "");
    }
  };

  // Update preview HTML when code changes and preview tab is active
  useEffect(() => {
    if (activeViewTab === "preview" && currentCode) {
      // If code is HTML, use it directly, otherwise wrap it
      const isHtml =
        currentCode.trim().startsWith("<!DOCTYPE") ||
        currentCode.trim().startsWith("<html");
      if (isHtml) {
        setPreviewHtml(currentCode);
      } else {
        // Wrap in basic HTML structure for preview
        setPreviewHtml(`
          <!DOCTYPE html>
          <html>
            <head>
              <meta charset="UTF-8">
              <meta name="viewport" content="width=device-width, initial-scale=1.0">
              <title>Preview</title>
            </head>
            <body>
              <pre style="padding: 20px; font-family: monospace; white-space: pre-wrap;">${currentCode}</pre>
            </body>
          </html>
        `);
      }
    }
  }, [currentCode, activeViewTab]);

  const getLanguageFromPath = (path: string): string => {
    const ext = path.split(".").pop()?.toLowerCase();
    const langMap: Record<string, string> = {
      ts: "typescript",
      tsx: "typescript",
      js: "javascript",
      jsx: "javascript",
      html: "html",
      css: "css",
      json: "json",
      md: "markdown",
    };
    return langMap[ext || ""] || "typescript";
  };

  return (
    <div className="flex h-full flex-col bg-white">
      {/* File Tabs */}
      <div className="flex border-b border-neutral-200 bg-white overflow-x-auto">
        {openFiles.map((file) => (
          <div
            key={file.path}
            onClick={() => {
              setActiveFile(file.path);
              onFileSelect?.(file.path);
            }}
            className={`flex items-center gap-1.5 px-3 py-2 text-xs font-normal cursor-pointer border-r border-neutral-200 whitespace-nowrap ${
              activeFile === file.path
                ? "bg-white text-neutral-900"
                : "bg-white text-neutral-600 hover:bg-neutral-50"
            }`}
          >
            <span className="text-[11px]">{file.path}</span>
            {openFiles.length > 1 && (
              <button
                onClick={(e) => closeFile(file.path, e)}
                className="ml-0.5 hover:bg-neutral-200 rounded px-1 text-neutral-500 hover:text-neutral-900"
              >
                ×
              </button>
            )}
          </div>
        ))}
      </div>

      <div className="flex items-center justify-between border-b border-stone-200 bg-[#fafaf8]">
        <div className="flex">
        <button
          onClick={() => setActiveViewTab("code")}
          className={`px-4 py-2 text-xs font-medium transition-colors ${
            activeViewTab === "code"
              ? "border-b-2 border-stone-900 bg-white text-stone-950"
              : "text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100"
          }`}
        >
          Code
        </button>
        <button
          onClick={() => setActiveViewTab("preview")}
          className={`px-4 py-2 text-xs font-medium transition-colors ${
            activeViewTab === "preview"
              ? "border-b-2 border-stone-900 bg-white text-stone-950"
              : "text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100"
          }`}
        >
          Preview
        </button>
        </div>
        {sandboxPreviewUrl && (
          <a
            href={sandboxPreviewUrl}
            target="_blank"
            rel="noopener noreferrer"
            className="mr-3 text-xs font-medium text-stone-500 transition hover:text-stone-950"
          >
            Open preview ↗
          </a>
        )}
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-hidden">
        {activeViewTab === "code" ? (
          <MonacoEditor
            height="100%"
            language={getLanguageFromPath(activeFile)}
            theme="vs-light"
            value={currentCode}
            onChange={handleEditorChange}
            options={{
              minimap: { enabled: true },
              fontSize: 14,
              wordWrap: "on",
              automaticLayout: true,
              scrollBeyondLastLine: false,
              tabSize: 2,
              lineNumbers: "on",
              roundedSelection: false,
              cursorStyle: "line",
              formatOnPaste: true,
              formatOnType: true,
            }}
          />
        ) : (
          <div className="relative h-full w-full overflow-hidden bg-[#f6f6f3]">
            {sandboxPreviewUrl ? (
              <iframe
                key={sandboxPreviewUrl}
                src={`${sandboxPreviewUrl}?cutable=${encodeURIComponent(projectId)}`}
                className="h-full w-full border-0 bg-white"
                title="Sandbox Preview"
                sandbox="allow-same-origin allow-scripts allow-forms allow-modals allow-popups"
              />
            ) : (
              <iframe
                srcDoc={
                  previewHtml ||
                  "<html><body><p>No content to preview</p></body></html>"
                }
                className="w-full h-full border-0"
                title="Preview"
                sandbox="allow-scripts allow-same-origin allow-forms allow-modals"
              />
            )}
            {!sandboxPreviewUrl && (
              <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
                <div className="px-4 py-2 text-xs text-neutral-600 bg-white/80 border border-dashed border-neutral-300 rounded-lg">
                  Sandbox preview not ready yet
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
});

export default EditorSection;
