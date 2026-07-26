export interface ProviderCredentials {
  openRouterApiKey: string;
  e2bApiKey: string;
}

const STORAGE_KEY = "cutable-provider-credentials";

export function loadProviderCredentials(): ProviderCredentials | null {
  if (typeof window === "undefined") return null;
  const raw = window.sessionStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as Partial<ProviderCredentials>;
    const credentials = {
      openRouterApiKey: parsed.openRouterApiKey?.trim() ?? "",
      e2bApiKey: parsed.e2bApiKey?.trim() ?? "",
    };
    return credentials.openRouterApiKey && credentials.e2bApiKey
      ? credentials
      : null;
  } catch {
    window.sessionStorage.removeItem(STORAGE_KEY);
    return null;
  }
}

export function saveProviderCredentials(credentials: ProviderCredentials) {
  window.sessionStorage.setItem(
    STORAGE_KEY,
    JSON.stringify({
      openRouterApiKey: credentials.openRouterApiKey.trim(),
      e2bApiKey: credentials.e2bApiKey.trim(),
    })
  );
}

export function clearProviderCredentials() {
  if (typeof window !== "undefined") {
    window.sessionStorage.removeItem(STORAGE_KEY);
  }
}
