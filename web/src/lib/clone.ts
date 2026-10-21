import type { StorageLike } from "./storage.ts";

const VM_VOLUME = /^(scsi|virtio|sata|ide|efidisk|tpmstate)\d+$/;
const CT_VOLUME = /^(rootfs|mp\d+)$/;

/** The storages a guest's volumes live on, as Proxmox counts them for a clone. */
export function guestStorages(kind: string, config: Record<string, unknown>): string[] {
  const out = new Set<string>();
  for (const [key, raw] of Object.entries(config)) {
    if (!(kind === "lxc" ? CT_VOLUME : VM_VOLUME).test(key)) continue;
    const volume = String(raw ?? "").split(",")[0];
    // An empty or physical drive, or a container bind mount, isn't a volume.
    if (!volume || volume === "none" || volume === "cdrom" || volume.startsWith("/")) continue;
    const sid = volume.includes(":") ? volume.split(":")[0] : "";
    if (sid) out.add(sid);
  }
  return [...out];
}

/**
 * Whether a clone may go to another node. Proxmox allows it only when every
 * volume is on shared storage; a container's full clone also may when it is
 * cloned onto a shared storage.
 */
export function cloneCanLeaveNode(
  kind: string,
  config: Record<string, unknown>,
  storages: (StorageLike & { shared?: number })[],
  full: boolean,
  targetStorage?: string,
): { allowed: boolean; local: string[] } {
  const shared = (sid: string) => !!storages.find((s) => s.storage === sid)?.shared;
  const local = guestStorages(kind, config).filter((sid) => !shared(sid));
  if (local.length === 0) return { allowed: true, local };
  if (kind === "lxc" && full && targetStorage && shared(targetStorage)) return { allowed: true, local };
  return { allowed: false, local };
}
