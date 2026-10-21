import { useRouteContext } from "@tanstack/react-router";

export type Permissions = Record<string, Record<string, number>>;

export function can(
  kind: string | undefined,
  perms: Permissions | undefined,
  path: string,
  priv: string,
  tokenScoped = false,
): boolean {
  // Proxmox users are checked against their own ACL; a local admin against the
  // connection token's when Hypervane could read it, and unrestricted otherwise.
  if (kind !== "pve" && !tokenScoped) return true;
  for (const prefix of pathPrefixes(path)) {
    const entry = perms?.[prefix];
    if (!entry) continue;
    if (entry[priv]) return true;
    if (Object.keys(entry).some((k) => k === priv || k.startsWith(`${priv}.`))) return true;
  }
  return false;
}

function pathPrefixes(path: string): string[] {
  const trimmed = path.replace(/\/+$/, "");
  if (!trimmed || trimmed === "/") return ["/"];
  const parts = trimmed.replace(/^\//, "").split("/");
  const prefixes: string[] = [];
  let cur = "";
  for (const p of parts) {
    cur += `/${p}`;
    prefixes.push(cur);
  }
  prefixes.reverse();
  prefixes.push("/");
  return prefixes;
}

export function useCan() {
  const ctx = useRouteContext({ from: "/app" });
  return (path: string, priv: string) => can(ctx.kind, ctx.permissions, path, priv, ctx.tokenScoped);
}

/** True when any path grants something beyond read access. */
export function hasWritePrivilege(perms: Permissions): boolean {
  return Object.values(perms ?? {}).some((privs) =>
    Object.entries(privs ?? {}).some(([priv, on]) => !!on && !priv.endsWith(".Audit") && priv !== "Sys.Syslog"),
  );
}

/** True when the connection's API token can only read, so nothing can change. */
export function useReadOnly(): boolean {
  const ctx = useRouteContext({ from: "/app" });
  return !!ctx.tokenScoped && !hasWritePrivilege(ctx.permissions ?? {});
}
