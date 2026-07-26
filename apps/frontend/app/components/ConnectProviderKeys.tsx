"use client";

import { useEffect, useState } from "react";
import ProviderKeyDialog from "./ProviderKeyDialog";
import {
  loadProviderCredentials,
  type ProviderCredentials,
} from "@/app/lib/provider-credentials";

type ConnectProviderKeysProps = {
  variant?: "nav" | "hero";
};

export default function ConnectProviderKeys({
  variant = "nav",
}: ConnectProviderKeysProps) {
  const [open, setOpen] = useState(false);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    setConnected(Boolean(loadProviderCredentials()));
  }, []);

  const closeDialog = () => {
    setOpen(false);
    setConnected(Boolean(loadProviderCredentials()));
  };

  const saved = (_credentials: ProviderCredentials) => {
    setConnected(true);
    setOpen(false);
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={
          variant === "hero"
            ? "inline-flex items-center gap-2 rounded-lg border border-stone-300 bg-white px-4 py-2.5 text-sm font-medium text-stone-700 shadow-sm transition hover:border-stone-400 hover:bg-stone-50"
            : "rounded-md border border-stone-300 bg-white px-3 py-2 text-sm font-medium text-stone-700 transition hover:border-stone-400 hover:bg-stone-50"
        }
      >
        <span
          className={`h-2 w-2 rounded-full ${connected ? "bg-emerald-500" : "bg-stone-300"}`}
        />
        {connected
          ? variant === "hero"
            ? "Provider keys connected"
            : "Keys connected"
          : variant === "hero"
            ? "Connect OpenRouter + E2B keys"
            : "Connect keys"}
      </button>
      <ProviderKeyDialog
        open={open}
        message="Connect before or after signing in. These keys stay in this browser tab, are used automatically for your builds, and do not consume the two demo runs."
        onClose={closeDialog}
        onSaved={saved}
      />
    </>
  );
}
