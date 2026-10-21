/**
 * Cluster join information, in the same format as Proxmox's own "Join
 * Information" window, so a blob copied here pastes into Proxmox's "Join
 * Cluster" dialog and the other way round: base64 of
 * { ipAddress, fingerprint, peerLinks, ring_addr, totem }.
 */

export type JoinInfo = {
  ipAddress: string;
  fingerprint: string;
  peerLinks: Record<number, string>;
  ring_addr: string[];
  totem: Record<string, unknown>;
};

/** What the join form needs from a pasted blob. */
export type DecodedJoin = {
  ip: string;
  fingerprint: string;
  clusterName: string;
  /** One entry per corosync link the cluster uses, with the peer's address for it. */
  links: { number: number; peer: string }[];
};

const LINK_RE = /(?:ring|link)(\d+)_addr/;

/**
 * Builds join information from GET /cluster/config/join, for the node the
 * request went to (preferred_node), or null when this isn't a cluster.
 */
export function buildJoinInfo(data: unknown): JoinInfo | null {
  const d = (data ?? {}) as { preferred_node?: string; nodelist?: Record<string, unknown>[]; totem?: Record<string, unknown> };
  if (!d.totem?.cluster_name || !Array.isArray(d.nodelist) || !d.nodelist.length) return null;
  const node = d.nodelist.find((n) => n.name === d.preferred_node);
  if (!node) return null;
  const peerLinks: Record<number, string> = {};
  const ring_addr: string[] = [];
  for (const [key, value] of Object.entries(node)) {
    const m = LINK_RE.exec(key);
    if (m) {
      peerLinks[Number(m[1])] = String(value);
      ring_addr.push(String(value));
    }
  }
  return {
    ipAddress: String(node.pve_addr ?? ""),
    fingerprint: String(node.pve_fp ?? ""),
    peerLinks,
    ring_addr,
    totem: d.totem,
  };
}

export function encodeJoinInfo(info: JoinInfo): string {
  const bytes = new TextEncoder().encode(JSON.stringify(info));
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}

/** Reads a pasted blob, or returns null when it isn't join information. */
export function decodeJoinInfo(blob: string): DecodedJoin | null {
  try {
    const bin = atob(blob.replace(/\s+/g, ""));
    const bytes = Uint8Array.from(bin, (c) => c.charCodeAt(0));
    const info = JSON.parse(new TextDecoder().decode(bytes)) as Partial<JoinInfo>;
    if (!info?.totem || typeof info.totem !== "object") return null;
    const interfaces = Object.values((info.totem as { interface?: Record<string, { linknumber?: number | string }> }).interface ?? {});
    const links = interfaces
      .map((iface) => {
        const number = Number(iface.linknumber ?? 0);
        return { number, peer: String(info.peerLinks?.[number] ?? "") };
      })
      .sort((a, b) => a.number - b.number);
    return {
      ip: String(info.ipAddress ?? ""),
      fingerprint: String(info.fingerprint ?? ""),
      clusterName: String((info.totem as { cluster_name?: string }).cluster_name ?? ""),
      links: links.length ? links : [{ number: 0, peer: String(info.ring_addr?.[0] ?? "") }],
    };
  } catch {
    return null;
  }
}
