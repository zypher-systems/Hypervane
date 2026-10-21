const KEY = "hypervane:connection";
export const CONNECTION_EVENT = "hypervane:connection";

export function getActiveConnection(): string {
  try {
    return localStorage.getItem(KEY) ?? "";
  } catch {
    return "";
  }
}

export function setActiveConnection(id: string) {
  const next = id.trim();
  if (getActiveConnection() === next) return;
  try {
    if (next) localStorage.setItem(KEY, next);
    else localStorage.removeItem(KEY);
  } catch {
    /* ignore */
  }
  window.dispatchEvent(new CustomEvent(CONNECTION_EVENT, { detail: next }));
}

export function onActiveConnection(fn: (id: string) => void) {
  const handler = (e: Event) => fn((e as CustomEvent<string>).detail ?? getActiveConnection());
  window.addEventListener(CONNECTION_EVENT, handler);
  return () => window.removeEventListener(CONNECTION_EVENT, handler);
}
