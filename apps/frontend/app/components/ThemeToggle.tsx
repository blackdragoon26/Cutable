"use client";

import { useEffect, useState } from "react";

type ThemeSetting = "default" | "light" | "dark";

const THEME_KEY = "cutable-theme";

function resolveTheme(setting: ThemeSetting) {
  return setting === "default"
    ? window.matchMedia("(prefers-color-scheme: dark)").matches
      ? "dark"
      : "light"
    : setting;
}

function applyTheme(setting: ThemeSetting) {
  const resolved = resolveTheme(setting);
  document.documentElement.dataset.theme = resolved;
  document.documentElement.dataset.themeSetting = setting;
  window.localStorage.setItem(THEME_KEY, setting);
  window.dispatchEvent(
    new CustomEvent("cutable-theme-change", { detail: { resolved } })
  );
}

export default function ThemeToggle() {
  const [setting, setSetting] = useState<ThemeSetting>("default");

  useEffect(() => {
    const saved = window.localStorage.getItem(THEME_KEY);
    const initial =
      saved === "light" || saved === "dark" ? saved : "default";
    setSetting(initial);
    applyTheme(initial);

    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const syncDefault = () => {
      if (document.documentElement.dataset.themeSetting === "default") {
        applyTheme("default");
      }
    };
    media.addEventListener("change", syncDefault);
    return () => media.removeEventListener("change", syncDefault);
  }, []);

  return (
    <label className="relative">
      <span className="sr-only">Color theme</span>
      <select
        aria-label="Color theme"
        value={setting}
        onChange={(event) => {
          const next = event.target.value as ThemeSetting;
          setSetting(next);
          applyTheme(next);
        }}
        className="h-9 rounded-md border border-stone-300 bg-white px-2 text-xs font-medium text-stone-700 outline-none transition hover:border-stone-400 focus:border-[#e6538b]"
      >
        <option value="default">Default</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
    </label>
  );
}
