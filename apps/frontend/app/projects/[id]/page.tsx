"use client";

import { use, useState, useRef, useEffect, useCallback } from "react";
import Link from "next/link";
import ProjectInfo from "@/app/components/ProjectInfo";
import FileExplorer from "@/app/components/FileExplorer";
import EditorSection from "@/app/components/EditorSection";
import ChatSection from "@/app/components/ChatSection";
import ProgressLoader from "@/app/components/ProgressLoader";
import ProviderKeyDialog from "@/app/components/ProviderKeyDialog";
import Brand from "@/app/components/Brand";
import ThemeToggle from "@/app/components/ThemeToggle";
import { useCreateSandbox, useProject, useProjectFiles, useSandboxInfo } from "@/app/hooks/useProjectQueries";
import { useWebSocket } from "@/app/hooks/useWebSocket";
import { useAgentSession } from "@/app/hooks/useAgentSession";
import { getAccountUsage, type DemoUsage, type FileNode } from "@/app/lib/api";
import {
  loadProviderCredentials,
  type ProviderCredentials,
} from "@/app/lib/provider-credentials";

interface ProjectPageProps {
  params: Promise<{ id: string }>;
}

export default function ProjectPage({ params }: ProjectPageProps) {
  const { id } = use(params);
  const editorRef = useRef<{ openFile: (filePath: string) => void } | null>(null);
  const [activeFile, setActiveFile] = useState<string | null>(null);
  const [showChat, setShowChat] = useState(false);
  const [credentials, setCredentials] = useState<ProviderCredentials | null>(null);
  const [demoUsage, setDemoUsage] = useState<DemoUsage | null>(null);
  const [usageReady, setUsageReady] = useState(false);
  const [showKeyDialog, setShowKeyDialog] = useState(false);
  const [keyDialogMessage, setKeyDialogMessage] = useState<string | null>(null);
  const [pendingPrompt, setPendingPrompt] = useState<string | null>(null);

  const { data: projectData, isLoading: projectLoading } = useProject(id);
  const { data: filesData, isLoading: filesLoading } = useProjectFiles(id);
  const { data: sandboxData } = useSandboxInfo(id);
  const restartPreview = useCreateSandbox(id);
  const project = projectData?.project;
  const files = filesData?.files ?? [];

  const {
    messages,
    sendUserMessage,
    handleAgentEvent,
    planSteps,
    isProcessing,
    isThinking,
    stageInfo,
    stageLabel,
  } = useAgentSession(id);
  const [hasBootstrappedPrompt, setHasBootstrappedPrompt] = useState(false);

  useEffect(() => {
    setCredentials(loadProviderCredentials());
    getAccountUsage()
      .then((response) => setDemoUsage(response.demo))
      .finally(() => setUsageReady(true));
  }, []);

  const handleSocketEvent = useCallback(
    (event: { e: string; [key: string]: any }) => {
      if (event.e === "credentials_required") {
        setKeyDialogMessage(
          event.message ||
            "Your two demo builds are used. Add your OpenRouter and E2B keys to continue."
        );
        if (event.demo) setDemoUsage(event.demo);
        setShowKeyDialog(true);
      }
      if (event.e === "usage_updated" && event.demo) {
        setDemoUsage(event.demo);
      }
      void handleAgentEvent(event);
    },
    [handleAgentEvent]
  );

  // WebSocket connection with error handling
  const { isConnected, connectionError, sendMessage } = useWebSocket({
    projectId: id,
    onMessage: handleSocketEvent,
  });

  const handleAgentPrompt = useCallback(
    (prompt: string, activeCredentials: ProviderCredentials | null) => {
      const trimmed = prompt.trim();
      if (!trimmed || !isConnected || !sendMessage) return;
      sendMessage({
        type: "start_agent",
        prompt: trimmed,
        ...(activeCredentials ? { credentials: activeCredentials } : {}),
      });
    },
    [sendMessage, isConnected]
  );

  const handleUserPrompt = useCallback(
    async (
      prompt: string,
      credentialOverride?: ProviderCredentials | null
    ) => {
      const trimmed = prompt.trim();
      if (!trimmed || !isConnected) return;
      const activeCredentials =
        credentialOverride === undefined ? credentials : credentialOverride;
      if (demoUsage?.requiresKeys && !activeCredentials) {
        setPendingPrompt(trimmed);
        setKeyDialogMessage(
          "Your two demo builds are complete. Add your OpenRouter and E2B API keys to continue."
        );
        setShowKeyDialog(true);
        return;
      }
      const persistPromise = sendUserMessage(trimmed);
      handleAgentPrompt(trimmed, activeCredentials);
      await persistPromise;
    },
    [credentials, demoUsage?.requiresKeys, sendUserMessage, handleAgentPrompt, isConnected]
  );

  const handleCredentialsSaved = useCallback(
    (nextCredentials: ProviderCredentials) => {
      setCredentials(nextCredentials);
      setShowKeyDialog(false);
      setKeyDialogMessage(null);
      if (pendingPrompt) {
        const prompt = pendingPrompt;
        setPendingPrompt(null);
        void handleUserPrompt(prompt, nextCredentials);
      }
    },
    [handleUserPrompt, pendingPrompt]
  );

  useEffect(() => {
    if (files.length > 0 && !activeFile) {
      const firstFile = findFirstFile(files);
      if (firstFile) {
        setActiveFile(firstFile.path);
        editorRef.current?.openFile(firstFile.path);
      }
    }
  }, [files, activeFile]);

  useEffect(() => {
    const projectReady = !projectLoading && project?.initialPrompt;
    const messagesEmpty = messages.length === 0;
    const hasNoGeneratedFiles = !filesLoading && files.length === 0;
    const canBootstrap =
      !hasBootstrappedPrompt &&
      projectReady &&
      isConnected &&
      usageReady &&
      messagesEmpty &&
      hasNoGeneratedFiles;

    if (canBootstrap) {
      setHasBootstrappedPrompt(true);
      const timer = window.setTimeout(() => {
        handleUserPrompt(project.initialPrompt);
      }, 200);
      return () => window.clearTimeout(timer);
    }
  }, [
    hasBootstrappedPrompt,
    projectLoading,
    project?.initialPrompt,
    isConnected,
    usageReady,
    messages.length,
    files.length,
    filesLoading,
    handleUserPrompt,
  ]);

  const handleFileSelect = (filePath: string) => {
    editorRef.current?.openFile(filePath);
    setActiveFile(filePath);
  };

  function findFirstFile(nodes: FileNode[]): { path: string } | null {
    for (const node of nodes) {
      if (node.type === "file" && node.path) {
        return { path: node.path };
      }
      if (node.children && node.children.length > 0) {
        const found = findFirstFile(node.children);
        if (found) return found;
      }
    }
    return null;
  }

  const fileContents: Record<string, string> = {};
  if (files.length > 0) {
    function extractFiles(nodes: FileNode[]) {
      for (const node of nodes) {
        if (node.type === "file" && node.path) {
          fileContents[node.path] = "";
        }
        if (node.children) {
          extractFiles(node.children);
        }
      }
    }
    extractFiles(files);
  }

  const previewUrl = sandboxData?.previewUrl || null;

  const getConnectionStatus = () => {
    if (connectionError) {
      return { text: connectionError, color: "text-red-600", dot: "bg-red-500" };
    }
    if (isConnected) {
      if (isProcessing) {
        return { text: stageLabel || "Processing...", color: "text-amber-600", dot: "bg-amber-500 animate-pulse" };
      }
      return { text: "Ready", color: "text-[#47665d]", dot: "bg-[#557b6f]" };
    }
    return { text: "Connecting...", color: "text-neutral-600", dot: "bg-neutral-400 animate-pulse" };
  };

  const connectionStatus = getConnectionStatus();
  const handleRestartPreview = () => {
    if (demoUsage?.requiresKeys && !credentials) {
      setKeyDialogMessage(
        "Add your E2B and OpenRouter keys before restarting this preview."
      );
      setShowKeyDialog(true);
      return;
    }
    restartPreview.mutate();
  };

  return (
    <main className="flex h-screen flex-col overflow-hidden bg-white">
      <header className="flex h-14 items-center justify-between border-b border-stone-200 bg-[#fafaf8] px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Brand compact />
          <span className="h-4 w-px bg-stone-300" />
          <span className="max-w-64 truncate text-xs text-stone-500">{project?.title || "Project workspace"}</span>
          <span className="h-4 w-px bg-stone-300" />
          <div className="flex items-center gap-1.5">
            <div className={`h-1.5 w-1.5 rounded-full ${connectionStatus.dot}`} />
            <span className={`text-xs ${connectionStatus.color}`}>
              {connectionStatus.text}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <Link
            href="/dashboard"
            className="hidden rounded-md px-3 py-1.5 text-xs font-medium text-stone-500 transition hover:bg-stone-100 hover:text-stone-950 sm:inline-flex"
          >
            All projects
          </Link>
          <ThemeToggle />
          {demoUsage && (
            <span className="hidden rounded-md border border-stone-200 bg-white px-2.5 py-1.5 text-xs text-stone-600 sm:inline">
              {demoUsage.remaining > 0
                ? `${demoUsage.remaining} demo build${demoUsage.remaining === 1 ? "" : "s"} left`
                : "Own keys required"}
            </span>
          )}
          <button
            type="button"
            onClick={() => {
              setKeyDialogMessage(null);
              setShowKeyDialog(true);
            }}
            className="rounded-md border border-stone-300 bg-white px-3 py-1.5 text-xs font-medium text-stone-700 hover:bg-stone-50"
          >
            API keys
          </button>
          {previewUrl && (
            <a href={previewUrl} target="_blank" rel="noopener noreferrer" className="rounded-md border border-stone-300 bg-white px-3 py-1.5 text-xs font-medium text-stone-700 hover:bg-stone-50">
              Open preview ↗
            </a>
          )}
          <button
            type="button"
            onClick={handleRestartPreview}
            disabled={restartPreview.isPending}
            className="rounded-md px-3 py-1.5 text-xs font-medium text-stone-500 transition hover:bg-stone-100 hover:text-stone-950 disabled:opacity-50"
          >
            {restartPreview.isPending ? "Restarting…" : "Restart preview"}
          </button>
          <button
            onClick={() => setShowChat(!showChat)}
            className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
              showChat
                ? "bg-stone-900 text-white"
                : "border border-stone-300 bg-white text-stone-700 hover:bg-stone-50"
            }`}
          >
            {showChat ? "Hide chat" : "Open chat"}
          </button>
        </div>
      </header>

      {/* Connection Error Banner */}
      {connectionError && (
        <div className="bg-red-50 border-b border-red-200 px-4 py-2 text-sm text-red-700">
          {connectionError}
        </div>
      )}

      {/* Progress Loader - Shows when processing */}
      {(isProcessing || stageInfo) && (
        <div className="border-b border-stone-200 bg-[#fafaf8] px-4 py-3">
          <ProgressLoader
            stageInfo={stageInfo}
            stageLabel={stageLabel}
            isProcessing={isProcessing}
          />
        </div>
      )}

      {/* Main Content Area - Three Panes */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Pane - Project Info */}
        <div className="w-80 flex-shrink-0 border-r border-neutral-200">
          <ProjectInfo
            title={project?.title}
            initialPrompt={project?.initialPrompt}
            attachmentNames={project?.attachments?.map((attachment) => attachment.name)}
            planSteps={planSteps}
            recentMessages={messages}
            onAskPrompt={handleUserPrompt}
            isAgentConnected={isConnected}
            isProcessing={isProcessing}
            isThinking={isThinking}
          />
        </div>

        {/* Middle Pane - File Explorer */}
        <div className="w-64 flex-shrink-0 border-r border-neutral-200">
          {filesLoading ? (
            <div className="p-4 text-sm text-neutral-500">Loading files...</div>
          ) : (
            <FileExplorer
              files={files}
              activeFile={activeFile || undefined}
              onFileSelect={handleFileSelect}
            />
          )}
        </div>

        {/* Right Pane - Editor and Chat */}
        <div className="flex-1 min-w-0 flex overflow-hidden">
          <div className={`flex-1 min-w-0 transition-all ${showChat ? 'w-2/3' : 'w-full'}`}>
            <EditorSection
              ref={editorRef}
              projectId={id}
              fileContents={fileContents}
              initialFile={activeFile ? { path: activeFile, code: "" } : undefined}
              onActiveFileChange={setActiveFile}
              sandboxPreviewUrl={previewUrl || null}
            />
          </div>
          {showChat && (
            <div className="w-1/3 border-l border-neutral-200 flex-shrink-0">
              <ChatSection
                messages={messages}
                onSubmitPrompt={handleUserPrompt}
                isAgentConnected={isConnected}
                isProcessing={isProcessing}
              />
            </div>
          )}
        </div>
      </div>
      <ProviderKeyDialog
        open={showKeyDialog}
        message={keyDialogMessage}
        onClose={() => {
          setShowKeyDialog(false);
          setPendingPrompt(null);
        }}
        onSaved={handleCredentialsSaved}
      />
    </main>
  );
}
