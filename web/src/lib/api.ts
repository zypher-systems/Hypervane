import { getActiveConnection } from "@/lib/connection";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const headers = new Headers(init?.headers);
  if (init?.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  const connection = getActiveConnection();
  if (connection && !headers.has("X-Hypervane-Connection")) {
    headers.set("X-Hypervane-Connection", connection);
  }
  const res = await fetch(`/api${path}`, {
    credentials: "include",
    ...init,
    headers,
  });
  if (res.status === 204) return undefined as T;
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(res.status, data.error || data.message || res.statusText);
  }
  return data as T;
}

export const api = {
  me: () => request<import("@/types").Me>("/auth/me"),
  setup: (body: Record<string, string>) =>
    request("/auth/setup", { method: "POST", body: JSON.stringify(body) }),
  login: (username: string, password: string, realm?: string, otp?: string, tfaChallenge?: string, connection?: string) =>
    request<import("@/types").Me>("/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password, realm, otp, tfaChallenge, connection }),
    }),
  logout: () => request("/auth/logout", { method: "POST" }),
  authRealms: (connection?: string) =>
    request<import("@/types").AuthRealm[]>(`/auth/realms${connection ? `?connection=${encodeURIComponent(connection)}` : ""}`),
  connections: () => request<import("@/types").Connection[]>("/connections"),
  inventory: () => request<{ connections: import("@/types").InventoryGroup[] }>("/inventory"),
  adminConnections: () => request<import("@/types").Connection[]>("/admin/connections"),
  createConnection: (body: Record<string, unknown>) =>
    request<import("@/types").Connection>("/admin/connections", { method: "POST", body: JSON.stringify(body) }),
  updateConnection: (id: string, body: Record<string, unknown>) =>
    request<import("@/types").Connection>(`/admin/connections/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteConnection: (id: string) =>
    request(`/admin/connections/${encodeURIComponent(id)}`, { method: "DELETE" }),
  testConnection: (body: Record<string, unknown>) =>
    request<import("@/types").Connection>("/admin/connections/test", { method: "POST", body: JSON.stringify(body) }),
  testSavedConnection: (id: string) =>
    request<import("@/types").Connection>(`/admin/connections/${encodeURIComponent(id)}/test`, { method: "POST" }),
  summary: () =>
    request<{ status: import("@/types").ClusterStatus[]; resources: import("@/types").Resource[] }>(
      "/cluster/summary",
    ),
  nextid: () => request<{ vmid: number }>("/cluster/nextid"),
  node: (node: string) => request(`/nodes/${encodeURIComponent(node)}`),
  nodeRrd: (node: string, timeframe: string) =>
    request<Record<string, number>[]>(`/nodes/${encodeURIComponent(node)}/rrd?timeframe=${timeframe}`),
  nodeIoRrd: (node: string, timeframe: string) =>
    request<{ disk: Record<string, number>[]; iops: Record<string, number>[] }>(
      `/nodes/${encodeURIComponent(node)}/io-rrd?timeframe=${timeframe}`,
    ),
  ring: (key: string) => request<import("@/types").RingSample[]>(`/metrics/ring?key=${encodeURIComponent(key)}`),
  guestIopsLatest: () =>
    request<{ samples: Record<string, { rd: number; wr: number; wait: number }> }>("/metrics/guest-iops-latest"),
  scripts: () => request<{ recipes: import("@/types").ScriptRecipe[] }>("/scripts"),
  scriptPlan: (id: string, params: Record<string, unknown>) =>
    request<import("@/types").ScriptPlan>(`/scripts/${encodeURIComponent(id)}/plan`, { method: "POST", body: JSON.stringify(params) }),
  scriptRun: (id: string, params: Record<string, unknown>) =>
    request<import("@/types").ScriptJob>(`/scripts/${encodeURIComponent(id)}/run`, { method: "POST", body: JSON.stringify(params) }),
  scriptJobs: () => request<{ jobs: import("@/types").ScriptJob[] }>("/scripts/jobs"),
  scriptJob: (id: string) => request<import("@/types").ScriptJob>(`/scripts/jobs/${encodeURIComponent(id)}`),
  audit: () => request<{ events: import("@/types").AuditEvent[] }>("/audit"),
  metricHistory: (key: string, from?: string, to?: string) => {
    const q = new URLSearchParams({ key });
    if (from) q.set("from", from);
    if (to) q.set("to", to);
    return request<{ points: import("@/types").HistoryPoint[] }>(`/metrics/history?${q}`);
  },
  guest: (vmid: number, node?: string, type?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    const qs = q.toString();
    return request<{
      node: string;
      type: string;
      vmid: number;
      status: import("@/types").GuestStatus;
      config: Record<string, unknown>;
      pending: import("@/types").PendingChange[];
    }>(`/guests/${vmid}${qs ? `?${qs}` : ""}`);
  },
  createGuest: (body: Record<string, unknown>) =>
    request<{ upid: string; vmid: number }>("/guests", { method: "POST", body: JSON.stringify(body) }),
  sendKey: (vmid: number, key: string, node?: string, type?: string) =>
    request(`/guests/${vmid}/sendkey?${guestQ(node, type)}`, { method: "POST", body: JSON.stringify({ key }) }),
  qemuMonitor: (vmid: number, command: string, node?: string, type?: string) =>
    request<{ output: string }>(`/guests/${vmid}/monitor?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify({ command }),
    }),
  updateGuest: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    return request(`/guests/${vmid}?${q}`, { method: "PUT", body: JSON.stringify(body) });
  },
  deleteGuest: (vmid: number, node?: string, type?: string, opts?: { purge?: boolean; force?: boolean; destroyUnreferencedDisks?: boolean }) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    if (opts?.purge) q.set("purge", "1");
    if (opts?.force) q.set("force", "1");
    if (opts?.destroyUnreferencedDisks) q.set("destroy-unreferenced-disks", "1");
    return request<{ upid: string }>(`/guests/${vmid}?${q}`, { method: "DELETE" });
  },
  power: (vmid: number, action: string, node?: string, type?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    return request<{ upid: string }>(`/guests/${vmid}/status/${action}?${q}`, { method: "POST" });
  },
  guestRrd: (vmid: number, timeframe: string, node?: string, type?: string) => {
    const q = new URLSearchParams({ timeframe });
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    return request<Record<string, number>[]>(`/guests/${vmid}/rrd?${q}`);
  },
  guestIoRrd: (vmid: number, timeframe: string, node?: string, type?: string) => {
    const q = new URLSearchParams({ timeframe });
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    return request<{ iops: Record<string, number>[] }>(`/guests/${vmid}/io-rrd?${q}`);
  },
  snapshots: (vmid: number, node?: string, type?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    return request<import("@/types").Snapshot[]>(`/guests/${vmid}/snapshots?${q}`);
  },
  createSnapshot: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    return request<{ upid: string }>(`/guests/${vmid}/snapshots?${q}`, {
      method: "POST",
      body: JSON.stringify(body),
    });
  },
  deleteSnapshot: (vmid: number, name: string, node?: string, type?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    return request<{ upid: string }>(`/guests/${vmid}/snapshots/${encodeURIComponent(name)}?${q}`, {
      method: "DELETE",
    });
  },
  rollbackSnapshot: (vmid: number, name: string, node?: string, type?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (type) q.set("type", type);
    return request<{ upid: string }>(`/guests/${vmid}/snapshots/${encodeURIComponent(name)}/rollback?${q}`, {
      method: "POST",
    });
  },
  updateSnapshot: (vmid: number, name: string, body: { description: string }, node?: string, type?: string) =>
    request(`/guests/${vmid}/snapshots/${encodeURIComponent(name)}?${guestQ(node, type)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  revertPending: (vmid: number, digest: string, node?: string, type?: string) => {
    const q = new URLSearchParams(guestQ(node, type));
    if (digest) q.set("digest", digest);
    return request(`/guests/${vmid}/pending?${q}`, { method: "DELETE" });
  },
  guestFeature: (vmid: number, feature: "snapshot" | "clone", node?: string, type?: string) => {
    const q = new URLSearchParams(guestQ(node, type));
    q.set("feature", feature);
    return request<{ supported: boolean; hasFeature: boolean; nodes?: string[] }>(`/guests/${vmid}/feature?${q}`);
  },
  storages: () => request<import("@/types").Storage[]>("/storage"),
  createStorage: (body: Record<string, unknown>) =>
    request("/storage", { method: "POST", body: JSON.stringify(body) }),
  updateStorage: (storage: string, body: Record<string, unknown>) =>
    request(`/storage/${encodeURIComponent(storage)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteStorage: (storage: string) =>
    request(`/storage/${encodeURIComponent(storage)}`, { method: "DELETE" }),
  storageConfig: (storage: string) =>
    request<Record<string, unknown>>(`/storage/${encodeURIComponent(storage)}`),
  updateVolumeNotes: (storage: string, node: string, volume: string, notes: string) =>
    request<import("@/types").DiskTaskResult>(
      `/storage/${encodeURIComponent(storage)}/content/notes?node=${encodeURIComponent(node)}&volume=${encodeURIComponent(volume)}`,
      { method: "PUT", body: JSON.stringify({ notes }) },
    ),
  ociPull: (storage: string, node: string, body: Record<string, unknown>) =>
    request<import("@/types").DiskTaskResult>(
      `/storage/${encodeURIComponent(storage)}/oci-pull?node=${encodeURIComponent(node)}`,
      { method: "POST", body: JSON.stringify(body) },
    ),
  storageIdentity: (storage: string, node: string) =>
    request<import("@/types").FeatureObject>(
      `/storage/${encodeURIComponent(storage)}/identity?node=${encodeURIComponent(node)}`,
    ),
  updateStorageIdentity: (storage: string, node: string, body: Record<string, unknown>) =>
    request<import("@/types").DiskTaskResult>(
      `/storage/${encodeURIComponent(storage)}/identity?node=${encodeURIComponent(node)}`,
      { method: "PUT", body: JSON.stringify(body) },
    ),
  scanStorage: (kind: string, node: string, query: Record<string, string> = {}) => {
    const q = new URLSearchParams({ node, ...query });
    return request<import("@/types").FeatureList>(`/storage/scan/${encodeURIComponent(kind)}?${q}`);
  },
  storageContent: (storage: string, node: string, content?: string) => {
    const q = new URLSearchParams({ node });
    if (content) q.set("content", content);
    return request<import("@/types").StorageContent[]>(`/storage/${encodeURIComponent(storage)}/content?${q}`);
  },
  deleteVolume: (storage: string, node: string, volume: string) =>
    request<{ upid: string }>(
      `/storage/${encodeURIComponent(storage)}/content?node=${encodeURIComponent(node)}&volume=${encodeURIComponent(volume)}`,
      { method: "DELETE" },
    ),
  storageRrd: (storage: string, node: string, timeframe: string) =>
    request<Record<string, number>[]>(
      `/storage/${encodeURIComponent(storage)}/rrd?node=${encodeURIComponent(node)}&timeframe=${timeframe}`,
    ),
  tasks: (opts?: string | { node?: string; guest?: string; storage?: string; source?: string; user?: string; type?: string }) => {
    const q = new URLSearchParams();
    if (typeof opts === "string") {
      q.set("node", opts);
    } else if (opts) {
      if (opts.node) q.set("node", opts.node);
      if (opts.guest) q.set("guest", opts.guest);
      if (opts.storage) q.set("storage", opts.storage);
      if (opts.source) q.set("source", opts.source);
      if (opts.user) q.set("user", opts.user);
      if (opts.type) q.set("type", opts.type);
    }
    const qs = q.toString();
    return request<import("@/types").Task[]>(`/tasks${qs ? `?${qs}` : ""}`);
  },
  task: (upid: string) => request<{ status: { status: string; exitstatus?: string }; log: import("@/types").TaskLogLine[] }>(`/tasks/${encodeURIComponent(upid)}`),
  stopTask: (upid: string, node?: string) => {
    const q = node ? `?node=${encodeURIComponent(node)}` : "";
    return request(`/tasks/${encodeURIComponent(upid)}${q}`, { method: "DELETE" });
  },
  clone: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request<{ upid: string; vmid: number }>(`/guests/${vmid}/clone?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  template: (vmid: number, node?: string, type?: string) =>
    request<{ upid: string }>(`/guests/${vmid}/template?${guestQ(node, type)}`, { method: "POST" }),
  migrate: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request<{ upid: string }>(`/guests/${vmid}/migrate?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  remoteMigrate: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request<{ upid: string }>(`/guests/${vmid}/remote-migrate?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  clusterRemotes: () => request<import("@/types").FeatureList>("/datacenter/remotes"),
  createClusterRemote: (body: Record<string, unknown>) =>
    request("/datacenter/remotes", { method: "POST", body: JSON.stringify(body) }),
  updateClusterRemote: (id: string, body: Record<string, unknown>) =>
    request(`/datacenter/remotes/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteClusterRemote: (id: string) =>
    request(`/datacenter/remotes/${encodeURIComponent(id)}`, { method: "DELETE" }),
  resizeDisk: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request<{ upid: string }>(`/guests/${vmid}/disks/resize?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  moveDisk: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request<{ upid: string }>(`/guests/${vmid}/disks/move?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  unlinkDisk: (vmid: number, idlist: string, node?: string, type?: string) =>
    request(`/guests/${vmid}/disks/unlink?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify({ idlist }),
    }),
  rescanDisks: (vmid: number, node?: string, type?: string) =>
    request<{ supported: boolean; upid?: string }>(`/guests/${vmid}/disks/rescan?${guestQ(node, type)}`, {
      method: "POST",
    }),
  cloudinit: (vmid: number, node?: string, type?: string) =>
    request<Record<string, unknown>>(`/guests/${vmid}/cloudinit?${guestQ(node, type)}`),
  saveCloudinit: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request(`/guests/${vmid}/cloudinit?${guestQ(node, type)}`, { method: "PUT", body: JSON.stringify(body) }),
  cloudinitDump: (vmid: number, typ: "user" | "network" | "meta", node?: string, type?: string) =>
    request<{ supported: boolean; dump?: string }>(`/guests/${vmid}/cloudinit/dump?${guestQ(node, type)}&type=${typ}`),
  regenerateCloudinit: (vmid: number, node?: string, type?: string) =>
    request<{ supported: boolean; upid?: string }>(`/guests/${vmid}/cloudinit/regenerate?${guestQ(node, type)}`, {
      method: "POST",
    }),
  agent: (vmid: number, node?: string, type?: string) =>
    request<Record<string, unknown>>(`/guests/${vmid}/agent?${guestQ(node, type)}`),
  agentFstrim: (vmid: number, node?: string, type?: string) =>
    request(`/guests/${vmid}/agent/fstrim?${guestQ(node, type)}`, { method: "POST" }),
  agentPassword: (vmid: number, username: string, password: string, node?: string, type?: string) =>
    request(`/guests/${vmid}/agent/password?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }),
  agentCommand: (vmid: number, command: string, node?: string, type?: string, query: Record<string, string> = {}) => {
    const q = new URLSearchParams(guestQ(node, type));
    for (const [k, v] of Object.entries(query)) q.set(k, v);
    return request<Record<string, unknown>>(`/guests/${vmid}/agent/${encodeURIComponent(command)}?${q}`);
  },
  agentCommandPost: (vmid: number, command: string, body: Record<string, unknown> = {}, node?: string, type?: string) =>
    request<Record<string, unknown>>(`/guests/${vmid}/agent/${encodeURIComponent(command)}?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  importMetadata: (storage: string, node: string, volume: string) =>
    request<{ supported: boolean; data?: Record<string, unknown> }>(
      `/storage/${encodeURIComponent(storage)}/import-metadata?node=${encodeURIComponent(node)}&volume=${encodeURIComponent(volume)}`,
    ),
  queryUrlMetadata: (storage: string, node: string, url: string) =>
    request<{ supported: boolean; data?: Record<string, unknown> }>(
      `/storage/${encodeURIComponent(storage)}/query-url-metadata?node=${encodeURIComponent(node)}&url=${encodeURIComponent(url)}`,
    ),
  capabilities: (node: string) =>
    request<import("@/types").NodeCapabilities>(`/nodes/${encodeURIComponent(node)}/capabilities`),
  media: (content: string, node?: string) => {
    const q = new URLSearchParams({ content });
    if (node) q.set("node", node);
    return request<import("@/types").MediaItem[]>(`/media?${q}`);
  },
  downloadUrl: (storage: string, node: string, body: Record<string, unknown>) =>
    request<{ upid: string }>(`/storage/${encodeURIComponent(storage)}/download-url?node=${encodeURIComponent(node)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  upload: async (storage: string, node: string, file: File, content: string, onProgress?: (pct: number) => void) => {
    const fd = new FormData();
    fd.append("content", content);
    fd.append("file", file, file.name);
    await new Promise<void>((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", `/api/storage/${encodeURIComponent(storage)}/upload?node=${encodeURIComponent(node)}`);
      xhr.withCredentials = true;
      xhr.upload.onprogress = (ev) => {
        if (ev.lengthComputable && onProgress) onProgress(Math.round((100 * ev.loaded) / ev.total));
      };
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) resolve();
        else {
          try {
            reject(new ApiError(xhr.status, JSON.parse(xhr.responseText).error || xhr.statusText));
          } catch {
            reject(new ApiError(xhr.status, xhr.statusText));
          }
        }
      };
      xhr.onerror = () => reject(new ApiError(0, "upload failed"));
      xhr.send(fd);
    });
  },
  bulk: (action: string, guests: import("@/types").GuestRef[], extras?: Record<string, unknown>) =>
    request<{ upids: string[] }>(`/guests/bulk/${action}`, {
      method: "POST",
      body: JSON.stringify({ guests, ...extras }),
    }),
  backupJobs: () => request<import("@/types").BackupJob[]>("/backups/jobs"),
  createBackupJob: (body: import("@/types").BackupJobRequest) =>
    request<import("@/types").BackupJob>("/backups/jobs", { method: "POST", body: JSON.stringify(body) }),
  updateBackupJob: (id: string, body: import("@/types").BackupJobRequest) =>
    request(`/backups/jobs/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteBackupJob: (id: string) => request(`/backups/jobs/${encodeURIComponent(id)}`, { method: "DELETE" }),
  jobVolumes: (id: string) =>
    request<import("@/types").IncludedVolume>(`/backups/jobs/${encodeURIComponent(id)}/volumes`),
  coverage: () => request<import("@/types").CoverageReport>("/backups/coverage"),
  insights: () => request<import("@/types").InsightsReport>("/insights"),
  guestBackup: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request<{ upid: string }>(`/guests/${vmid}/backup?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  restore: (body: Record<string, unknown>) =>
    request<{ upid: string; vmid: number }>("/backups/restore", { method: "POST", body: JSON.stringify(body) }),
  extractConfig: (node: string, volume: string) =>
    request<{ config: string; type: string }>(
      `/backups/extractconfig?node=${encodeURIComponent(node)}&volume=${encodeURIComponent(volume)}`,
    ),
  fileRestore: (storage: string, node: string, volume: string, filepath?: string) => {
    const q = new URLSearchParams({ node, volume, filepath: filepath || "/" });
    return request<import("@/types").FileRestoreEntry[]>(
      `/storage/${encodeURIComponent(storage)}/file-restore?${q}`,
    );
  },
  fileRestoreDownloadUrl: (storage: string, node: string, volume: string, filepath: string) => {
    const q = new URLSearchParams({ node, volume, filepath });
    const connection = getActiveConnection();
    if (connection) q.set("connection", connection);
    return `/api/storage/${encodeURIComponent(storage)}/file-restore/download?${q}`;
  },
  volumeDownloadUrl: (storage: string, node: string, volume: string) => {
    const q = new URLSearchParams({ node, volume });
    const connection = getActiveConnection();
    if (connection) q.set("connection", connection);
    return `/api/storage/${encodeURIComponent(storage)}/download?${q}`;
  },
  prunePreview: (storage: string, node: string, prune: string, vmid?: number, type?: string) => {
    const q = new URLSearchParams({ node, prune });
    if (vmid) q.set("vmid", String(vmid));
    if (type) q.set("type", type);
    return request<import("@/types").PrunePreview[]>(`/storage/${encodeURIComponent(storage)}/prune?${q}`);
  },
  prune: (storage: string, node: string, prune: string, vmid?: number, type?: string) => {
    const q = new URLSearchParams({ node, prune });
    if (vmid) q.set("vmid", String(vmid));
    if (type) q.set("type", type);
    return request<{ upid: string }>(`/storage/${encodeURIComponent(storage)}/prune?${q}`, { method: "DELETE" });
  },
  replication: () => request<import("@/types").ReplicationJob[]>("/replication"),
  createReplication: (body: Record<string, unknown>) =>
    request("/replication", { method: "POST", body: JSON.stringify(body) }),
  updateReplication: (id: string, body: Record<string, unknown>) =>
    request(`/replication/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteReplication: (id: string, opts?: { force?: boolean; keep?: boolean }) => {
    const q = new URLSearchParams();
    if (opts?.force) q.set("force", "1");
    if (opts?.keep) q.set("keep", "1");
    const suffix = q.size ? `?${q}` : "";
    return request(`/replication/${encodeURIComponent(id)}${suffix}`, { method: "DELETE" });
  },
  replicationStatus: (id: string, node?: string) =>
    request<import("@/types").ReplicationStatus>(
      `/replication/${encodeURIComponent(id)}/status${node ? `?node=${encodeURIComponent(node)}` : ""}`,
    ),
  replicationLog: (id: string, node?: string) =>
    request<import("@/types").TaskLogLine[]>(
      `/replication/${encodeURIComponent(id)}/log${node ? `?node=${encodeURIComponent(node)}` : ""}`,
    ),
  replicationNow: (id: string, node?: string) =>
    request<{ upid: string }>(
      `/replication/${encodeURIComponent(id)}/schedule-now${node ? `?node=${encodeURIComponent(node)}` : ""}`,
      { method: "POST" },
    ),
  haStatus: () => request<import("@/types").HAStatus>("/ha/status"),
  armHA: () => request("/ha/arm", { method: "POST" }),
  disarmHA: (resourceMode: "freeze" | "ignore") =>
    request("/ha/disarm", { method: "POST", body: JSON.stringify({ "resource-mode": resourceMode }) }),
  haResources: () => request<import("@/types").HAResource[]>("/ha/resources"),
  haResource: (sid: string) => request<import("@/types").HAResource>(`/ha/resources/${encodeURIComponent(sid)}`),
  createHAResource: (body: Record<string, unknown>) =>
    request("/ha/resources", { method: "POST", body: JSON.stringify(body) }),
  updateHAResource: (sid: string, body: Record<string, unknown>) =>
    request(`/ha/resources/${encodeURIComponent(sid)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteHAResource: (sid: string, opts?: { purge?: boolean }) => {
    const q = new URLSearchParams();
    if (opts?.purge !== false) q.set("purge", "1");
    else q.set("purge", "0");
    return request(`/ha/resources/${encodeURIComponent(sid)}?${q}`, { method: "DELETE" });
  },
  haMigrate: (sid: string, node: string) =>
    request<{ upid?: string }>(`/ha/resources/${encodeURIComponent(sid)}/migrate`, {
      method: "POST",
      body: JSON.stringify({ node }),
    }),
  haRelocate: (sid: string, node: string) =>
    request<{ upid?: string }>(`/ha/resources/${encodeURIComponent(sid)}/relocate`, {
      method: "POST",
      body: JSON.stringify({ node }),
    }),
  haGroups: () => request<import("@/types").HAGroup[]>("/ha/groups"),
  createHAGroup: (body: Record<string, unknown>) =>
    request("/ha/groups", { method: "POST", body: JSON.stringify(body) }),
  updateHAGroup: (group: string, body: Record<string, unknown>) =>
    request(`/ha/groups/${encodeURIComponent(group)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteHAGroup: (group: string) =>
    request(`/ha/groups/${encodeURIComponent(group)}`, { method: "DELETE" }),
  haRules: () => request<import("@/types").HARulesResponse>("/ha/rules"),
  createHARule: (body: Record<string, unknown>) =>
    request("/ha/rules", { method: "POST", body: JSON.stringify(body) }),
  updateHARule: (rule: string, body: Record<string, unknown>) =>
    request(`/ha/rules/${encodeURIComponent(rule)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteHARule: (rule: string) =>
    request(`/ha/rules/${encodeURIComponent(rule)}`, { method: "DELETE" }),
  accessUsers: () => request<import("@/types").AccessUser[]>("/access/users"),
  accessUser: (userid: string) =>
    request<import("@/types").AccessUser>(`/access/users/${encodeURIComponent(userid)}`),
  createAccessUser: (body: Record<string, unknown>) =>
    request("/access/users", { method: "POST", body: JSON.stringify(body) }),
  updateAccessUser: (userid: string, body: Record<string, unknown>) =>
    request(`/access/users/${encodeURIComponent(userid)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteAccessUser: (userid: string) =>
    request(`/access/users/${encodeURIComponent(userid)}`, { method: "DELETE" }),
  setAccessPassword: (userid: string, password: string) =>
    request(`/access/users/${encodeURIComponent(userid)}/password`, {
      method: "PUT",
      body: JSON.stringify({ password }),
    }),
  accessTokens: (userid: string) =>
    request<import("@/types").AccessToken[]>(`/access/users/${encodeURIComponent(userid)}/tokens`),
  createAccessToken: (userid: string, body: Record<string, unknown>) =>
    request<import("@/types").AccessToken>(`/access/users/${encodeURIComponent(userid)}/tokens`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteAccessToken: (userid: string, tokenid: string) =>
    request(`/access/users/${encodeURIComponent(userid)}/tokens/${encodeURIComponent(tokenid)}`, {
      method: "DELETE",
    }),
  accessTfa: (userid: string) =>
    request<import("@/types").TFAEntry[]>(`/access/users/${encodeURIComponent(userid)}/tfa`),
  enrollAccessTfa: (userid: string, body: Record<string, unknown>) =>
    request<import("@/types").TFAEnroll>(`/access/users/${encodeURIComponent(userid)}/tfa`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteAccessTfa: (userid: string, id: string) =>
    request(`/access/users/${encodeURIComponent(userid)}/tfa/${encodeURIComponent(id)}`, { method: "DELETE" }),
  accessGroups: () => request<import("@/types").AccessGroup[]>("/access/groups"),
  createAccessGroup: (body: Record<string, unknown>) =>
    request("/access/groups", { method: "POST", body: JSON.stringify(body) }),
  updateAccessGroup: (group: string, body: Record<string, unknown>) =>
    request(`/access/groups/${encodeURIComponent(group)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteAccessGroup: (group: string) =>
    request(`/access/groups/${encodeURIComponent(group)}`, { method: "DELETE" }),
  accessRoles: () => request<import("@/types").AccessRole[]>("/access/roles"),
  createAccessRole: (body: Record<string, unknown>) =>
    request("/access/roles", { method: "POST", body: JSON.stringify(body) }),
  updateAccessRole: (role: string, body: Record<string, unknown>) =>
    request(`/access/roles/${encodeURIComponent(role)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteAccessRole: (role: string) =>
    request(`/access/roles/${encodeURIComponent(role)}`, { method: "DELETE" }),
  accessAcl: () => request<import("@/types").ACLEntry[]>("/access/acl"),
  updateAccessAcl: (body: Record<string, unknown>) =>
    request("/access/acl", { method: "PUT", body: JSON.stringify(body) }),
  accessRealms: () => request<import("@/types").AuthRealm[]>("/access/realms"),
  accessRealm: (realm: string) => request<Record<string, unknown>>(`/access/realms/${encodeURIComponent(realm)}`),
  createAccessRealm: (body: Record<string, unknown>) =>
    request("/access/realms", { method: "POST", body: JSON.stringify(body) }),
  updateAccessRealm: (realm: string, body: Record<string, unknown>) =>
    request(`/access/realms/${encodeURIComponent(realm)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteAccessRealm: (realm: string) =>
    request(`/access/realms/${encodeURIComponent(realm)}`, { method: "DELETE" }),
  syncAccessRealm: (realm: string, body?: Record<string, unknown>) =>
    request<{ upid?: string }>(`/access/realms/${encodeURIComponent(realm)}/sync`, {
      method: "POST",
      body: JSON.stringify(body ?? {}),
    }),
  fwDCRules: () => request<import("@/types").FWRule[]>("/firewall/dc/rules"),
  createFWDCRule: (body: Record<string, unknown>) =>
    request("/firewall/dc/rules", { method: "POST", body: JSON.stringify(body) }),
  updateFWDCRule: (pos: number, body: Record<string, unknown>) =>
    request(`/firewall/dc/rules/${pos}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteFWDCRule: (pos: number, digest?: string) =>
    request(`/firewall/dc/rules/${pos}${digest ? `?digest=${encodeURIComponent(digest)}` : ""}`, { method: "DELETE" }),
  fwDCOptions: () => request<import("@/types").FWOptions>("/firewall/dc/options"),
  updateFWDCOptions: (body: Record<string, unknown>) =>
    request("/firewall/dc/options", { method: "PUT", body: JSON.stringify(body) }),
  fwDCLog: (start?: number) =>
    request<import("@/types").FWLogLine[]>(`/firewall/dc/log${start ? `?start=${start}` : ""}`),
  fwDCAliases: () => request<import("@/types").FWAlias[]>("/firewall/dc/aliases"),
  createFWDCAlias: (body: Record<string, unknown>) =>
    request("/firewall/dc/aliases", { method: "POST", body: JSON.stringify(body) }),
  updateFWDCAlias: (name: string, body: Record<string, unknown>) =>
    request(`/firewall/dc/aliases/${encodeURIComponent(name)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteFWDCAlias: (name: string, digest?: string) =>
    request(`/firewall/dc/aliases/${encodeURIComponent(name)}${digest ? `?digest=${encodeURIComponent(digest)}` : ""}`, {
      method: "DELETE",
    }),
  fwDCIPSets: () => request<import("@/types").FWIPSet[]>("/firewall/dc/ipset"),
  createFWDCIPSet: (body: Record<string, unknown>) =>
    request("/firewall/dc/ipset", { method: "POST", body: JSON.stringify(body) }),
  deleteFWDCIPSet: (name: string) =>
    request(`/firewall/dc/ipset/${encodeURIComponent(name)}`, { method: "DELETE" }),
  fwDCIPSetCIDRs: (name: string) =>
    request<import("@/types").FWIPSetCIDR[]>(`/firewall/dc/ipset/${encodeURIComponent(name)}`),
  createFWDCIPSetCIDR: (name: string, body: Record<string, unknown>) =>
    request(`/firewall/dc/ipset/${encodeURIComponent(name)}`, { method: "POST", body: JSON.stringify(body) }),
  deleteFWDCIPSetCIDR: (name: string, cidr: string, digest?: string) => {
    const q = new URLSearchParams({ cidr });
    if (digest) q.set("digest", digest);
    return request(`/firewall/dc/ipset/${encodeURIComponent(name)}/cidr?${q}`, { method: "DELETE" });
  },
  fwMacros: () => request<import("@/types").FWMacro[]>("/firewall/dc/macros"),
  fwDCRefs: () => request<import("@/types").FWRefsResponse>("/firewall/dc/refs"),
  fwGroups: () => request<import("@/types").FWGroup[]>("/firewall/dc/groups"),
  createFWGroup: (body: Record<string, unknown>) =>
    request("/firewall/dc/groups", { method: "POST", body: JSON.stringify(body) }),
  deleteFWGroup: (group: string) =>
    request(`/firewall/dc/groups/${encodeURIComponent(group)}`, { method: "DELETE" }),
  fwGroupRules: (group: string) =>
    request<import("@/types").FWRule[]>(`/firewall/dc/groups/${encodeURIComponent(group)}`),
  createFWGroupRule: (group: string, body: Record<string, unknown>) =>
    request(`/firewall/dc/groups/${encodeURIComponent(group)}`, { method: "POST", body: JSON.stringify(body) }),
  updateFWGroupRule: (group: string, pos: number, body: Record<string, unknown>) =>
    request(`/firewall/dc/groups/${encodeURIComponent(group)}/${pos}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteFWGroupRule: (group: string, pos: number, digest?: string) =>
    request(
      `/firewall/dc/groups/${encodeURIComponent(group)}/${pos}${digest ? `?digest=${encodeURIComponent(digest)}` : ""}`,
      { method: "DELETE" },
    ),
  fwNodeRules: (node: string) =>
    request<import("@/types").FWRule[]>(`/firewall/nodes/${encodeURIComponent(node)}/rules`),
  createFWNodeRule: (node: string, body: Record<string, unknown>) =>
    request(`/firewall/nodes/${encodeURIComponent(node)}/rules`, { method: "POST", body: JSON.stringify(body) }),
  updateFWNodeRule: (node: string, pos: number, body: Record<string, unknown>) =>
    request(`/firewall/nodes/${encodeURIComponent(node)}/rules/${pos}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteFWNodeRule: (node: string, pos: number, digest?: string) =>
    request(
      `/firewall/nodes/${encodeURIComponent(node)}/rules/${pos}${digest ? `?digest=${encodeURIComponent(digest)}` : ""}`,
      { method: "DELETE" },
    ),
  fwNodeOptions: (node: string) =>
    request<import("@/types").FWOptions>(`/firewall/nodes/${encodeURIComponent(node)}/options`),
  updateFWNodeOptions: (node: string, body: Record<string, unknown>) =>
    request(`/firewall/nodes/${encodeURIComponent(node)}/options`, { method: "PUT", body: JSON.stringify(body) }),
  fwNodeLog: (node: string, start?: number) =>
    request<import("@/types").FWLogLine[]>(
      `/firewall/nodes/${encodeURIComponent(node)}/log${start ? `?start=${start}` : ""}`,
    ),
  fwGuestRules: (vmid: number, node?: string, type?: string) =>
    request<import("@/types").FWRule[]>(`/firewall/guests/${vmid}/rules?${guestQ(node, type)}`),
  createFWGuestRule: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/rules?${guestQ(node, type)}`, { method: "POST", body: JSON.stringify(body) }),
  updateFWGuestRule: (vmid: number, pos: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/rules/${pos}?${guestQ(node, type)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteFWGuestRule: (vmid: number, pos: number, digest?: string, node?: string, type?: string) => {
    const q = new URLSearchParams(guestQ(node, type));
    if (digest) q.set("digest", digest);
    return request(`/firewall/guests/${vmid}/rules/${pos}?${q}`, { method: "DELETE" });
  },
  fwGuestOptions: (vmid: number, node?: string, type?: string) =>
    request<import("@/types").FWOptions>(`/firewall/guests/${vmid}/options?${guestQ(node, type)}`),
  updateFWGuestOptions: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/options?${guestQ(node, type)}`, { method: "PUT", body: JSON.stringify(body) }),
  fwGuestLog: (vmid: number, node?: string, type?: string, start?: number) => {
    const q = new URLSearchParams(guestQ(node, type));
    if (start) q.set("start", String(start));
    return request<import("@/types").FWLogLine[]>(`/firewall/guests/${vmid}/log?${q}`);
  },
  fwGuestAliases: (vmid: number, node?: string, type?: string) =>
    request<import("@/types").FWAlias[]>(`/firewall/guests/${vmid}/aliases?${guestQ(node, type)}`),
  createFWGuestAlias: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/aliases?${guestQ(node, type)}`, { method: "POST", body: JSON.stringify(body) }),
  updateFWGuestAlias: (vmid: number, name: string, body: Record<string, unknown>, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/aliases/${encodeURIComponent(name)}?${guestQ(node, type)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteFWGuestAlias: (vmid: number, name: string, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/aliases/${encodeURIComponent(name)}?${guestQ(node, type)}`, { method: "DELETE" }),
  fwGuestIPSets: (vmid: number, node?: string, type?: string) =>
    request<import("@/types").FWIPSet[]>(`/firewall/guests/${vmid}/ipset?${guestQ(node, type)}`),
  createFWGuestIPSet: (vmid: number, body: Record<string, unknown>, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/ipset?${guestQ(node, type)}`, { method: "POST", body: JSON.stringify(body) }),
  deleteFWGuestIPSet: (vmid: number, name: string, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/ipset/${encodeURIComponent(name)}?${guestQ(node, type)}`, { method: "DELETE" }),
  fwGuestIPSetCIDRs: (vmid: number, name: string, node?: string, type?: string) =>
    request<import("@/types").FWIPSetCIDR[]>(`/firewall/guests/${vmid}/ipset/${encodeURIComponent(name)}?${guestQ(node, type)}`),
  createFWGuestIPSetCIDR: (vmid: number, name: string, body: Record<string, unknown>, node?: string, type?: string) =>
    request(`/firewall/guests/${vmid}/ipset/${encodeURIComponent(name)}?${guestQ(node, type)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteFWGuestIPSetCIDR: (vmid: number, name: string, cidr: string, node?: string, type?: string) => {
    const q = new URLSearchParams(guestQ(node, type));
    q.set("cidr", cidr);
    return request(`/firewall/guests/${vmid}/ipset/${encodeURIComponent(name)}/cidr?${q}`, { method: "DELETE" });
  },
  fwGuestRefs: (vmid: number, node?: string, type?: string) =>
    request<import("@/types").FWRefsResponse>(`/firewall/guests/${vmid}/refs?${guestQ(node, type)}`),
  nodeNetwork: (node: string) =>
    request<import("@/types").NetIface[]>(`/nodes/${encodeURIComponent(node)}/network`),
  createNodeNetwork: (node: string, body: Record<string, unknown>) =>
    request(`/nodes/${encodeURIComponent(node)}/network`, { method: "POST", body: JSON.stringify(body) }),
  updateNodeNetwork: (node: string, iface: string, body: Record<string, unknown>) =>
    request(`/nodes/${encodeURIComponent(node)}/network/${encodeURIComponent(iface)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteNodeNetwork: (node: string, iface: string, digest?: string) =>
    request(
      `/nodes/${encodeURIComponent(node)}/network/${encodeURIComponent(iface)}${digest ? `?digest=${encodeURIComponent(digest)}` : ""}`,
      { method: "DELETE" },
    ),
  applyNodeNetwork: (node: string) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/network/apply`, { method: "PUT" }),
  revertNodeNetwork: (node: string) =>
    request(`/nodes/${encodeURIComponent(node)}/network/revert`, { method: "DELETE" }),
  nodeHardware: (node: string, kind: "pci" | "usb") =>
    request<import("@/types").FeatureList>(`/nodes/${encodeURIComponent(node)}/hardware/${kind}`),
  nodeMdev: (node: string, id: string) =>
    request<import("@/types").FeatureList>(`/nodes/${encodeURIComponent(node)}/hardware/pci/${encodeURIComponent(id)}/mdev`),
  nodeReport: (node: string) =>
    request<import("@/types").NodeReport>(`/nodes/${encodeURIComponent(node)}/report`),
  wakeOnLan: (node: string) =>
    request<import("@/types").WakeOnLANResult>(`/nodes/${encodeURIComponent(node)}/wakeonlan`, { method: "POST" }),
  nodeConfig: (node: string) =>
    request<Record<string, unknown>>(`/nodes/${encodeURIComponent(node)}/config`),
  updateNodeConfig: (node: string, body: Record<string, unknown>) =>
    request(`/nodes/${encodeURIComponent(node)}/config`, { method: "PUT", body: JSON.stringify(body) }),
  nodeAPLInfo: (node: string) =>
    request<import("@/types").FeatureList>(`/nodes/${encodeURIComponent(node)}/aplinfo`),
  nodeGuestAll: (node: string, action: "startall" | "stopall" | "suspendall" | "migrateall", body: Record<string, unknown> = {}) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/guests/${action}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  initGPT: (node: string, disk: string) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/disks/initgpt`, {
      method: "POST",
      body: JSON.stringify({ disk }),
    }),
  zfsStatus: (node: string, name: string) =>
    request<import("@/types").ZFSPoolStatus>(`/nodes/${encodeURIComponent(node)}/disks/zfs/${encodeURIComponent(name)}`),
  zfsScrub: (node: string, name: string) =>
    request<import("@/types").DiskTaskResult>(`/nodes/${encodeURIComponent(node)}/disks/zfs/${encodeURIComponent(name)}/scrub`, {
      method: "POST",
    }),
  zfsImport: (node: string, body: Record<string, unknown>) =>
    request<import("@/types").DiskTaskResult>(`/nodes/${encodeURIComponent(node)}/disks/zfs/import`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  nodeDisks: (node: string) =>
    request<import("@/types").DiskInfo[]>(`/nodes/${encodeURIComponent(node)}/disks`),
  nodeDiskSMART: (node: string, disk: string) =>
    request<Record<string, unknown>>(`/nodes/${encodeURIComponent(node)}/disks/smart?disk=${encodeURIComponent(disk)}`),
  wipeNodeDisk: (node: string, disk: string) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/disks/wipe`, {
      method: "POST",
      body: JSON.stringify({ disk }),
    }),
  nodeDiskBackends: (node: string, kind: string) =>
    request<Record<string, unknown>[]>(`/nodes/${encodeURIComponent(node)}/disks/${encodeURIComponent(kind)}`),
  createNodeDiskBackend: (node: string, kind: string, body: Record<string, unknown>) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/disks/${encodeURIComponent(kind)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteNodeDiskBackend: (node: string, kind: string, name: string) =>
    request<{ upid?: string }>(
      `/nodes/${encodeURIComponent(node)}/disks/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`,
      { method: "DELETE" },
    ),
  aptUpdates: (node: string) =>
    request<Record<string, unknown>[]>(`/nodes/${encodeURIComponent(node)}/apt`),
  aptUpdate: (node: string) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/apt/update`, { method: "POST" }),
  aptVersions: (node: string) =>
    request<Record<string, unknown>[]>(`/nodes/${encodeURIComponent(node)}/apt/versions`),
  aptChangelog: (node: string, name: string) =>
    request<{ changelog: string }>(`/nodes/${encodeURIComponent(node)}/apt/changelog?name=${encodeURIComponent(name)}`),
  aptRepositories: (node: string) =>
    request<import("@/types").APTRepositories>(`/nodes/${encodeURIComponent(node)}/apt/repositories`),
  updateAPTRepository: (node: string, body: Record<string, unknown>) =>
    request(`/nodes/${encodeURIComponent(node)}/apt/repositories`, { method: "POST", body: JSON.stringify(body) }),
  aptUpgradeSession: (node: string, body: { password: string; otp?: string; tfaChallenge?: string }) =>
    request<{ grant?: string; needTfa?: boolean; tfaChallenge?: string }>(
      `/nodes/${encodeURIComponent(node)}/apt/upgrade-session`,
      { method: "POST", body: JSON.stringify(body) },
    ),
  nodeServices: (node: string) =>
    request<import("@/types").NodeService[]>(`/nodes/${encodeURIComponent(node)}/services`),
  nodeServiceAction: (node: string, service: string, action: string) =>
    request<{ upid?: string }>(
      `/nodes/${encodeURIComponent(node)}/services/${encodeURIComponent(service)}/${encodeURIComponent(action)}`,
      { method: "POST" },
    ),
  nodeDNS: (node: string) =>
    request<import("@/types").NodeDNS>(`/nodes/${encodeURIComponent(node)}/dns`),
  updateNodeDNS: (node: string, body: import("@/types").NodeDNS) =>
    request(`/nodes/${encodeURIComponent(node)}/dns`, { method: "PUT", body: JSON.stringify(body) }),
  nodeTime: (node: string) =>
    request<import("@/types").NodeTime>(`/nodes/${encodeURIComponent(node)}/timezone`),
  updateNodeTime: (node: string, timezone: string) =>
    request(`/nodes/${encodeURIComponent(node)}/timezone`, { method: "PUT", body: JSON.stringify({ timezone }) }),
  nodeHosts: (node: string) =>
    request<import("@/types").NodeHosts>(`/nodes/${encodeURIComponent(node)}/hosts`),
  updateNodeHosts: (node: string, body: import("@/types").NodeHosts) =>
    request(`/nodes/${encodeURIComponent(node)}/hosts`, { method: "PUT", body: JSON.stringify(body) }),
  nodeJournal: (node: string, start?: number) =>
    request<import("@/types").JournalLine[]>(
      `/nodes/${encodeURIComponent(node)}/journal${start ? `?start=${start}` : ""}`,
    ),
  nodeExecute: (node: string, commands: unknown) =>
    request<Record<string, unknown>[]>(`/nodes/${encodeURIComponent(node)}/execute`, {
      method: "POST",
      body: JSON.stringify({ commands }),
    }),
  nodeSubscription: (node: string) =>
    request<import("@/types").NodeSubscription>(`/nodes/${encodeURIComponent(node)}/subscription`),
  setNodeSubscription: (node: string, key: string) =>
    request(`/nodes/${encodeURIComponent(node)}/subscription`, { method: "PUT", body: JSON.stringify({ key }) }),
  nodeCertificates: (node: string) =>
    request<import("@/types").CertInfo[]>(`/nodes/${encodeURIComponent(node)}/certificates`),
  uploadNodeCertificate: (node: string, body: Record<string, unknown>) =>
    request(`/nodes/${encodeURIComponent(node)}/certificates`, { method: "POST", body: JSON.stringify(body) }),
  deleteNodeCertificate: (node: string) =>
    request(`/nodes/${encodeURIComponent(node)}/certificates`, { method: "DELETE" }),
  orderACMECertificate: (node: string, force?: boolean) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/certificates/acme`, {
      method: "POST",
      body: JSON.stringify({ force }),
    }),
  renewACMECertificate: (node: string) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/certificates/acme`, { method: "PUT" }),
  nodeCommand: (node: string, command: "reboot" | "shutdown") =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/status/${command}`, { method: "POST" }),
  dcOptions: () => request<Record<string, unknown>>("/datacenter/options"),
  updateDCOptions: (body: Record<string, unknown>) =>
    request("/datacenter/options", { method: "PUT", body: JSON.stringify(body) }),
  dcPools: () => request<import("@/types").Pool[]>("/datacenter/pools"),
  dcPool: (poolid: string) => request<import("@/types").Pool>(`/datacenter/pools/${encodeURIComponent(poolid)}`),
  createDCPool: (body: Record<string, unknown>) =>
    request("/datacenter/pools", { method: "POST", body: JSON.stringify(body) }),
  updateDCPool: (poolid: string, body: Record<string, unknown>) =>
    request(`/datacenter/pools/${encodeURIComponent(poolid)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteDCPool: (poolid: string) =>
    request(`/datacenter/pools/${encodeURIComponent(poolid)}`, { method: "DELETE" }),
  acmeAccounts: () => request<import("@/types").FeatureList>("/datacenter/acme/accounts"),
  createACMEAccount: (body: Record<string, unknown>) =>
    request<{ upid?: string }>("/datacenter/acme/accounts", { method: "POST", body: JSON.stringify(body) }),
  updateACMEAccount: (name: string, body: Record<string, unknown>) =>
    request<{ upid?: string }>(`/datacenter/acme/accounts/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteACMEAccount: (name: string) =>
    request(`/datacenter/acme/accounts/${encodeURIComponent(name)}`, { method: "DELETE" }),
  acmePlugins: () => request<import("@/types").FeatureList>("/datacenter/acme/plugins"),
  createACMEPlugin: (body: Record<string, unknown>) =>
    request("/datacenter/acme/plugins", { method: "POST", body: JSON.stringify(body) }),
  updateACMEPlugin: (id: string, body: Record<string, unknown>) =>
    request(`/datacenter/acme/plugins/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteACMEPlugin: (id: string) =>
    request(`/datacenter/acme/plugins/${encodeURIComponent(id)}`, { method: "DELETE" }),
  notifyEndpoints: (kind: string) =>
    request<import("@/types").FeatureList>(`/datacenter/notifications/endpoints/${encodeURIComponent(kind)}`),
  createNotifyEndpoint: (kind: string, body: Record<string, unknown>) =>
    request(`/datacenter/notifications/endpoints/${encodeURIComponent(kind)}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updateNotifyEndpoint: (kind: string, name: string, body: Record<string, unknown>) =>
    request(`/datacenter/notifications/endpoints/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteNotifyEndpoint: (kind: string, name: string) =>
    request(`/datacenter/notifications/endpoints/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`, {
      method: "DELETE",
    }),
  notifyMatchers: () => request<import("@/types").FeatureList>("/datacenter/notifications/matchers"),
  createNotifyMatcher: (body: Record<string, unknown>) =>
    request("/datacenter/notifications/matchers", { method: "POST", body: JSON.stringify(body) }),
  updateNotifyMatcher: (name: string, body: Record<string, unknown>) =>
    request(`/datacenter/notifications/matchers/${encodeURIComponent(name)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteNotifyMatcher: (name: string) =>
    request(`/datacenter/notifications/matchers/${encodeURIComponent(name)}`, { method: "DELETE" }),
  notifyTargets: () => request<import("@/types").FeatureList>("/datacenter/notifications/targets"),
  testNotifyTarget: (name: string) =>
    request<import("@/types").DiskTaskResult>(`/datacenter/notifications/targets/${encodeURIComponent(name)}/test`, {
      method: "POST",
    }),
  acmeDirectories: () => request<import("@/types").FeatureList>("/datacenter/acme/directories"),
  acmeMeta: (directory?: string) =>
    request<import("@/types").FeatureObject>(
      `/datacenter/acme/meta${directory ? `?directory=${encodeURIComponent(directory)}` : ""}`,
    ),
  acmeTos: (directory?: string) =>
    request<import("@/types").FeatureValue>(
      `/datacenter/acme/tos${directory ? `?directory=${encodeURIComponent(directory)}` : ""}`,
    ),
  acmeChallengeSchema: (plugin?: string) =>
    request<import("@/types").FeatureObject>(
      `/datacenter/acme/challenge-schema${plugin ? `?plugin=${encodeURIComponent(plugin)}` : ""}`,
    ),
  metricServers: () => request<import("@/types").FeatureList>("/datacenter/metrics/servers"),
  createMetricServer: (body: Record<string, unknown>) =>
    request("/datacenter/metrics/servers", { method: "POST", body: JSON.stringify(body) }),
  updateMetricServer: (id: string, body: Record<string, unknown>) =>
    request(`/datacenter/metrics/servers/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteMetricServer: (id: string) =>
    request(`/datacenter/metrics/servers/${encodeURIComponent(id)}`, { method: "DELETE" }),
  clusterLog: (max = 50) => request<import("@/types").FeatureList>(`/datacenter/log?max=${max}`),
  clusterJobs: () => request<import("@/types").FeatureList>("/datacenter/jobs"),
  realmSyncJobs: () => request<import("@/types").FeatureList>("/datacenter/jobs/realm-sync"),
  createRealmSyncJob: (body: Record<string, unknown>) =>
    request("/datacenter/jobs/realm-sync", { method: "POST", body: JSON.stringify(body) }),
  updateRealmSyncJob: (id: string, body: Record<string, unknown>) =>
    request(`/datacenter/jobs/realm-sync/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteRealmSyncJob: (id: string) =>
    request(`/datacenter/jobs/realm-sync/${encodeURIComponent(id)}`, { method: "DELETE" }),
  scheduleAnalyze: (schedule: string) =>
    request<import("@/types").FeatureList>(`/datacenter/jobs/schedule-analyze?schedule=${encodeURIComponent(schedule)}`),
  cpuModels: () => request<import("@/types").FeatureList>("/datacenter/cpu-models"),
  createCPUModel: (body: Record<string, unknown>) =>
    request("/datacenter/cpu-models", { method: "POST", body: JSON.stringify(body) }),
  updateCPUModel: (name: string, body: Record<string, unknown>) =>
    request(`/datacenter/cpu-models/${encodeURIComponent(name)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteCPUModel: (name: string) =>
    request(`/datacenter/cpu-models/${encodeURIComponent(name)}`, { method: "DELETE" }),
  vzdumpDefaults: (node: string, storage?: string) =>
    request<import("@/types").FeatureObject>(
      `/nodes/${encodeURIComponent(node)}/vzdump/defaults${storage ? `?storage=${encodeURIComponent(storage)}` : ""}`,
    ),
  updateVzdumpDefaults: (node: string, body: Record<string, unknown>) =>
    request<import("@/types").DiskTaskResult>(`/nodes/${encodeURIComponent(node)}/vzdump/defaults`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  openid: () => request<import("@/types").FeatureObject>("/access/openid"),
  unlockAccessTfa: (userid: string) =>
    request(`/access/users/${encodeURIComponent(userid)}/unlock-tfa`, { method: "PUT" }),
  clusterConfig: () => request<Record<string, unknown>>("/datacenter/cluster"),
  updateClusterTotem: (body: Record<string, unknown>) =>
    request("/datacenter/cluster/totem", { method: "PUT", body: JSON.stringify(body) }),
  createCluster: (body: Record<string, unknown>) =>
    request("/datacenter/cluster", { method: "POST", body: JSON.stringify(body) }),
  clusterJoinInfo: () => request<Record<string, unknown>>("/datacenter/cluster/join"),
  joinCluster: (body: Record<string, unknown>) =>
    request("/datacenter/cluster/join", { method: "POST", body: JSON.stringify(body) }),
  clusterNodes: () => request<import("@/types").FeatureList>("/datacenter/cluster/nodes"),
  addClusterNode: (body: Record<string, unknown>) =>
    request("/datacenter/cluster/nodes", { method: "POST", body: JSON.stringify(body) }),
  deleteClusterNode: (node: string) =>
    request(`/datacenter/cluster/nodes/${encodeURIComponent(node)}`, { method: "DELETE" }),
  clusterQDevice: () => request<import("@/types").FeatureList>("/datacenter/cluster/qdevice"),
  createClusterQDevice: (body: Record<string, unknown>) =>
    request("/datacenter/cluster/qdevice", { method: "POST", body: JSON.stringify(body) }),
  deleteClusterQDevice: () => request("/datacenter/cluster/qdevice", { method: "DELETE" }),
  mappings: (kind: string) =>
    request<import("@/types").FeatureList>(`/datacenter/mapping/${encodeURIComponent(kind)}`),
  createMapping: (kind: string, body: Record<string, unknown>) =>
    request(`/datacenter/mapping/${encodeURIComponent(kind)}`, { method: "POST", body: JSON.stringify(body) }),
  updateMapping: (kind: string, id: string, body: Record<string, unknown>) =>
    request(`/datacenter/mapping/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteMapping: (kind: string, id: string) =>
    request(`/datacenter/mapping/${encodeURIComponent(kind)}/${encodeURIComponent(id)}`, { method: "DELETE" }),
  sdnStatus: () => request<import("@/types").SDNStatus>("/sdn"),
  applySDN: (body?: Record<string, unknown>) =>
    request<{ upid: string }>("/sdn/apply", { method: "PUT", body: body ? JSON.stringify(body) : undefined }),
  rollbackSDN: (body?: Record<string, unknown>) =>
    request("/sdn/rollback", { method: "DELETE", body: body ? JSON.stringify(body) : undefined }),
  sdnDryRun: (node: string) =>
    request<import("@/types").FeatureObject>(`/sdn/dry-run?node=${encodeURIComponent(node)}`),
  lockSDN: (allowPending?: boolean) => {
    const q = allowPending ? "?allow-pending=1" : "";
    return request<import("@/types").FeatureValue>(`/sdn/lock${q}`, { method: "POST" });
  },
  unlockSDN: (token?: string, force?: boolean) => {
    const q = new URLSearchParams();
    if (token) q.set("lock-token", token);
    if (force) q.set("force", "1");
    const qs = q.toString();
    return request<import("@/types").DiskRescanResult>(`/sdn/lock${qs ? `?${qs}` : ""}`, { method: "DELETE" });
  },
  sdnIPAMStatus: (id: string) =>
    request<import("@/types").FeatureObject>(`/sdn/ipams/${encodeURIComponent(id)}/status`),
  sdnFabricInterfaces: (node: string, id: string) =>
    request<import("@/types").FeatureList>(
      `/sdn/nodes/${encodeURIComponent(node)}/fabrics/${encodeURIComponent(id)}/interfaces`,
    ),
  createSDNVNetIP: (vnet: string, body: Record<string, unknown>) =>
    request<import("@/types").DiskRescanResult>(`/sdn/vnets/${encodeURIComponent(vnet)}/ips`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteSDNVNetIP: (vnet: string, ip: string, mac?: string) => {
    const q = new URLSearchParams({ ip });
    if (mac) q.set("mac", mac);
    return request<import("@/types").DiskRescanResult>(`/sdn/vnets/${encodeURIComponent(vnet)}/ips?${q}`, {
      method: "DELETE",
    });
  },
  sdnZones: () => request<import("@/types").SDNZone[]>("/sdn/zones"),
  createSDNZone: (body: Record<string, unknown>) =>
    request("/sdn/zones", { method: "POST", body: JSON.stringify(body) }),
  updateSDNZone: (zone: string, body: Record<string, unknown>) =>
    request(`/sdn/zones/${encodeURIComponent(zone)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteSDNZone: (zone: string) =>
    request(`/sdn/zones/${encodeURIComponent(zone)}`, { method: "DELETE" }),
  sdnVNets: () => request<import("@/types").SDNVNet[]>("/sdn/vnets"),
  createSDNVNet: (body: Record<string, unknown>) =>
    request("/sdn/vnets", { method: "POST", body: JSON.stringify(body) }),
  updateSDNVNet: (vnet: string, body: Record<string, unknown>) =>
    request(`/sdn/vnets/${encodeURIComponent(vnet)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteSDNVNet: (vnet: string) =>
    request(`/sdn/vnets/${encodeURIComponent(vnet)}`, { method: "DELETE" }),
  sdnSubnets: (vnet: string) =>
    request<import("@/types").SDNSubnet[]>(`/sdn/vnets/${encodeURIComponent(vnet)}/subnets`),
  createSDNSubnet: (vnet: string, body: Record<string, unknown>) =>
    request(`/sdn/vnets/${encodeURIComponent(vnet)}/subnets`, { method: "POST", body: JSON.stringify(body) }),
  updateSDNSubnet: (vnet: string, subnet: string, body: Record<string, unknown>) =>
    request(`/sdn/vnets/${encodeURIComponent(vnet)}/subnets?subnet=${encodeURIComponent(subnet)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteSDNSubnet: (vnet: string, subnet: string) =>
    request(`/sdn/vnets/${encodeURIComponent(vnet)}/subnets?subnet=${encodeURIComponent(subnet)}`, {
      method: "DELETE",
    }),
  sdnVNetIPs: (vnet: string) =>
    request<import("@/types").FeatureList>(`/sdn/vnets/${encodeURIComponent(vnet)}/ips`),
  sdnControllers: () => request<import("@/types").FeatureList>("/sdn/controllers"),
  createSDNController: (body: Record<string, unknown>) =>
    request("/sdn/controllers", { method: "POST", body: JSON.stringify(body) }),
  updateSDNController: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/controllers/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteSDNController: (id: string) =>
    request(`/sdn/controllers/${encodeURIComponent(id)}`, { method: "DELETE" }),
  sdnIPAMs: () => request<import("@/types").FeatureList>("/sdn/ipams"),
  createSDNIPAM: (body: Record<string, unknown>) =>
    request("/sdn/ipams", { method: "POST", body: JSON.stringify(body) }),
  updateSDNIPAM: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/ipams/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteSDNIPAM: (id: string) =>
    request(`/sdn/ipams/${encodeURIComponent(id)}`, { method: "DELETE" }),
  sdnDNS: () => request<import("@/types").FeatureList>("/sdn/dns"),
  createSDNDNS: (body: Record<string, unknown>) =>
    request("/sdn/dns", { method: "POST", body: JSON.stringify(body) }),
  updateSDNDNS: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/dns/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteSDNDNS: (id: string) =>
    request(`/sdn/dns/${encodeURIComponent(id)}`, { method: "DELETE" }),
  sdnFabrics: () => request<import("@/types").FeatureList>("/sdn/fabrics"),
  createSDNFabric: (body: Record<string, unknown>) =>
    request("/sdn/fabrics", { method: "POST", body: JSON.stringify(body) }),
  updateSDNFabric: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteSDNFabric: (id: string) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}`, { method: "DELETE" }),
  sdnFabricNodes: (id: string) =>
    request<import("@/types").FeatureList>(`/sdn/fabrics/${encodeURIComponent(id)}/nodes`),
  createSDNFabricNode: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}/nodes`, { method: "POST", body: JSON.stringify(body) }),
  updateSDNFabricNode: (id: string, node: string, body: Record<string, unknown>) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}/nodes/${encodeURIComponent(node)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteSDNFabricNode: (id: string, node: string) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}/nodes/${encodeURIComponent(node)}`, { method: "DELETE" }),
  sdnFabricRouteMaps: (id: string) =>
    request<import("@/types").FeatureList>(`/sdn/fabrics/${encodeURIComponent(id)}/routemaps`),
  createSDNFabricRouteMap: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}/routemaps`, { method: "POST", body: JSON.stringify(body) }),
  deleteSDNFabricRouteMap: (id: string, name: string) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}/routemaps/${encodeURIComponent(name)}`, { method: "DELETE" }),
  sdnFabricPrefixLists: (id: string) =>
    request<import("@/types").FeatureList>(`/sdn/fabrics/${encodeURIComponent(id)}/prefixlists`),
  createSDNFabricPrefixList: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}/prefixlists`, { method: "POST", body: JSON.stringify(body) }),
  deleteSDNFabricPrefixList: (id: string, name: string) =>
    request(`/sdn/fabrics/${encodeURIComponent(id)}/prefixlists/${encodeURIComponent(name)}`, { method: "DELETE" }),
  sdnRouteMaps: () => request<import("@/types").FeatureList>("/sdn/route-maps"),
  createSDNRouteMap: (body: Record<string, unknown>) =>
    request("/sdn/route-maps", { method: "POST", body: JSON.stringify(body) }),
  deleteSDNRouteMap: (id: string) =>
    request(`/sdn/route-maps/${encodeURIComponent(id)}`, { method: "DELETE" }),
  sdnRouteMapEntries: (id: string) =>
    request<import("@/types").FeatureList>(`/sdn/route-maps/${encodeURIComponent(id)}/entries`),
  createSDNRouteMapEntry: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/route-maps/${encodeURIComponent(id)}/entries`, { method: "POST", body: JSON.stringify(body) }),
  updateSDNRouteMapEntry: (id: string, order: string, body: Record<string, unknown>) =>
    request(`/sdn/route-maps/${encodeURIComponent(id)}/entries/${encodeURIComponent(order)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteSDNRouteMapEntry: (id: string, order: string) =>
    request(`/sdn/route-maps/${encodeURIComponent(id)}/entries/${encodeURIComponent(order)}`, { method: "DELETE" }),
  sdnPrefixLists: () => request<import("@/types").FeatureList>("/sdn/prefix-lists"),
  createSDNPrefixList: (body: Record<string, unknown>) =>
    request("/sdn/prefix-lists", { method: "POST", body: JSON.stringify(body) }),
  updateSDNPrefixList: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/prefix-lists/${encodeURIComponent(id)}`, { method: "PUT", body: JSON.stringify(body) }),
  deleteSDNPrefixList: (id: string) =>
    request(`/sdn/prefix-lists/${encodeURIComponent(id)}`, { method: "DELETE" }),
  sdnPrefixListEntries: (id: string) =>
    request<import("@/types").FeatureList>(`/sdn/prefix-lists/${encodeURIComponent(id)}/entries`),
  createSDNPrefixListEntry: (id: string, body: Record<string, unknown>) =>
    request(`/sdn/prefix-lists/${encodeURIComponent(id)}/entries`, { method: "POST", body: JSON.stringify(body) }),
  updateSDNPrefixListEntry: (id: string, seq: string, body: Record<string, unknown>) =>
    request(`/sdn/prefix-lists/${encodeURIComponent(id)}/entries/${encodeURIComponent(seq)}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteSDNPrefixListEntry: (id: string, seq: string) =>
    request(`/sdn/prefix-lists/${encodeURIComponent(id)}/entries/${encodeURIComponent(seq)}`, { method: "DELETE" }),
  nodeSDN: (node: string) =>
    request<import("@/types").NodeSDNRuntime>(`/sdn/nodes/${encodeURIComponent(node)}`),
  fwVNetRules: (vnet: string) =>
    request<import("@/types").FWRule[]>(`/sdn/vnets/${encodeURIComponent(vnet)}/firewall/rules`),
  createFWVNetRule: (vnet: string, body: Record<string, unknown>) =>
    request(`/sdn/vnets/${encodeURIComponent(vnet)}/firewall/rules`, { method: "POST", body: JSON.stringify(body) }),
  updateFWVNetRule: (vnet: string, pos: number, body: Record<string, unknown>) =>
    request(`/sdn/vnets/${encodeURIComponent(vnet)}/firewall/rules/${pos}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteFWVNetRule: (vnet: string, pos: number, digest?: string) => {
    const q = digest ? `?digest=${encodeURIComponent(digest)}` : "";
    return request(`/sdn/vnets/${encodeURIComponent(vnet)}/firewall/rules/${pos}${q}`, { method: "DELETE" });
  },
  fwVNetOptions: (vnet: string) =>
    request<import("@/types").FWOptions>(`/sdn/vnets/${encodeURIComponent(vnet)}/firewall/options`),
  updateFWVNetOptions: (vnet: string, body: Record<string, unknown>) =>
    request(`/sdn/vnets/${encodeURIComponent(vnet)}/firewall/options`, { method: "PUT", body: JSON.stringify(body) }),
  fwVNetLog: (vnet: string) =>
    request<import("@/types").FWLogLine[]>(`/sdn/vnets/${encodeURIComponent(vnet)}/firewall/log`),
  cephStatus: (node?: string) =>
    request<import("@/types").CephStatus>(`/ceph${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  cephFlags: (node?: string) =>
    request<import("@/types").CephFlags>(`/ceph/flags${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  updateCephFlags: (body: Record<string, unknown>, node?: string) =>
    request(`/ceph/flags${node ? `?node=${encodeURIComponent(node)}` : ""}`, { method: "PUT", body: JSON.stringify(body) }),
  cephConfig: (node?: string) =>
    request<import("@/types").CephStatus>(`/ceph/config${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  cephCrush: (node?: string) =>
    request<import("@/types").CephStatus>(`/ceph/crush${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  updateCephCrush: (body: Record<string, unknown>, node?: string) =>
    request(`/ceph/crush${node ? `?node=${encodeURIComponent(node)}` : ""}`, { method: "PUT", body: JSON.stringify(body) }),
  cephRules: (node?: string) =>
    request<import("@/types").FeatureList>(`/ceph/rules${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  createCephRule: (body: Record<string, unknown>, node?: string) =>
    request(`/ceph/rules${node ? `?node=${encodeURIComponent(node)}` : ""}`, { method: "POST", body: JSON.stringify(body) }),
  cephLog: (node?: string) =>
    request<import("@/types").FWLogLine[]>(`/ceph/log${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  restartCeph: (node?: string) =>
    request<{ upid?: string }>(`/ceph/restart${node ? `?node=${encodeURIComponent(node)}` : ""}`, { method: "POST" }),
  cephReleases: (node?: string) =>
    request<import("@/types").FeatureList>(`/ceph/releases${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  cephRestartBulk: (body: Record<string, unknown>) =>
    request<{ upid?: string }>("/ceph/restart-bulk", { method: "POST", body: JSON.stringify(body) }),
  nodeCephRestartBulk: (node: string, body: Record<string, unknown>) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/ceph/restart-bulk`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  startCeph: (node?: string, service?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (service) q.set("service", service);
    const qs = q.toString();
    return request<{ upid?: string }>(`/ceph/start${qs ? `?${qs}` : ""}`, { method: "POST" });
  },
  stopCeph: (node?: string, service?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (service) q.set("service", service);
    const qs = q.toString();
    return request<{ upid?: string }>(`/ceph/stop${qs ? `?${qs}` : ""}`, { method: "POST" });
  },
  cephCmdSafety: (node: string, service: string, id: string, action: "stop" | "destroy") => {
    const q = new URLSearchParams({ node, service, id, action });
    return request<import("@/types").CephCmdSafety>(`/ceph/cmd-safety?${q}`);
  },
  cephMetadata: (node?: string) =>
    request<import("@/types").CephStatus>(`/ceph/metadata${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  cephRGW: (node?: string) =>
    request<import("@/types").FeatureList>(`/ceph/rgw${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  createCephRGW: (body: Record<string, unknown>, node?: string) =>
    request<{ upid?: string }>(`/ceph/rgw${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteCephRGW: (id: string, node?: string) =>
    request<{ upid?: string }>(`/ceph/rgw/${encodeURIComponent(id)}${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "DELETE",
    }),
  cephRGWUsers: (node?: string) =>
    request<import("@/types").FeatureList>(`/ceph/rgw/users${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  createCephRGWUser: (body: Record<string, unknown>, node?: string) =>
    request(`/ceph/rgw/users${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteCephRGWUser: (uid: string, node?: string) =>
    request(`/ceph/rgw/users/${encodeURIComponent(uid)}${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "DELETE",
    }),
  cephOSDMetadata: (osdid: number, node?: string) =>
    request<import("@/types").FeatureObject>(`/ceph/osds/${osdid}/metadata${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  cephOSDLVInfo: (osdid: number, node?: string) =>
    request<import("@/types").FeatureObject>(`/ceph/osds/${osdid}/lv-info${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  cephPoolStatus: (pool: string, node?: string) =>
    request<import("@/types").FeatureObject>(
      `/ceph/pools/${encodeURIComponent(pool)}/status${node ? `?node=${encodeURIComponent(node)}` : ""}`,
    ),
  cephMons: (node?: string) =>
    request<import("@/types").CephMon[]>(`/ceph/mons${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  createCephMon: (body: Record<string, unknown>, node?: string) =>
    request<{ upid?: string }>(`/ceph/mons${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteCephMon: (id: string, node?: string) =>
    request<{ upid?: string }>(`/ceph/mons/${encodeURIComponent(id)}${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "DELETE",
    }),
  cephMgrs: (node?: string) =>
    request<import("@/types").CephMgr[]>(`/ceph/mgrs${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  createCephMgr: (body: Record<string, unknown>, node?: string) =>
    request<{ upid?: string }>(`/ceph/mgrs${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteCephMgr: (id: string, node?: string) =>
    request<{ upid?: string }>(`/ceph/mgrs/${encodeURIComponent(id)}${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "DELETE",
    }),
  cephMDS: (node?: string) =>
    request<import("@/types").CephMDS[]>(`/ceph/mds${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  createCephMDS: (body: Record<string, unknown>, node?: string) =>
    request<{ upid?: string }>(`/ceph/mds${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  deleteCephMDS: (id: string, node?: string) =>
    request<{ upid?: string }>(`/ceph/mds/${encodeURIComponent(id)}${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "DELETE",
    }),
  cephOSDs: (node?: string) =>
    request<import("@/types").CephOSD[]>(`/ceph/osds${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  cephOSDIn: (osdid: number, node?: string) =>
    request<{ upid?: string }>(`/ceph/osds/${osdid}/in${node ? `?node=${encodeURIComponent(node)}` : ""}`, { method: "POST" }),
  cephOSDOut: (osdid: number, node?: string) =>
    request<{ upid?: string }>(`/ceph/osds/${osdid}/out${node ? `?node=${encodeURIComponent(node)}` : ""}`, { method: "POST" }),
  cephOSDScrub: (osdid: number, deep?: boolean, node?: string) => {
    const q = new URLSearchParams();
    if (node) q.set("node", node);
    if (deep) q.set("deep", "1");
    const qs = q.toString();
    return request<{ upid?: string }>(`/ceph/osds/${osdid}/scrub${qs ? `?${qs}` : ""}`, { method: "POST" });
  },
  destroyCephOSD: (osdid: number, node?: string) =>
    request<{ upid?: string }>(`/ceph/osds/${osdid}/destroy${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "POST",
    }),
  cephPools: (node?: string) =>
    request<import("@/types").CephPool[]>(`/ceph/pools${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  createCephPool: (body: Record<string, unknown>, node?: string) =>
    request<{ upid?: string }>(`/ceph/pools${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  updateCephPool: (pool: string, body: Record<string, unknown>, node?: string) =>
    request(`/ceph/pools/${encodeURIComponent(pool)}${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "PUT",
      body: JSON.stringify(body),
    }),
  deleteCephPool: (pool: string, node?: string) =>
    request<{ upid?: string }>(`/ceph/pools/${encodeURIComponent(pool)}${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "DELETE",
    }),
  cephFS: (node?: string) =>
    request<import("@/types").FeatureList>(`/ceph/fs${node ? `?node=${encodeURIComponent(node)}` : ""}`),
  createCephFS: (body: Record<string, unknown>, node?: string) =>
    request<{ upid?: string }>(`/ceph/fs${node ? `?node=${encodeURIComponent(node)}` : ""}`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  nodeCeph: (node: string) =>
    request<import("@/types").CephStatus>(`/nodes/${encodeURIComponent(node)}/ceph`),
  installCeph: (node: string, body: Record<string, unknown>) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/ceph/install`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  initCeph: (node: string, body: Record<string, unknown>) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/ceph/init`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  createNodeCephOSD: (node: string, body: Record<string, unknown>) =>
    request<{ upid?: string }>(`/nodes/${encodeURIComponent(node)}/ceph/osd`, {
      method: "POST",
      body: JSON.stringify(body),
    }),
  nodeCephLog: (node: string) =>
    request<import("@/types").FWLogLine[]>(`/nodes/${encodeURIComponent(node)}/ceph/log`),
};

function guestQ(node?: string, type?: string) {
  const q = new URLSearchParams();
  if (node) q.set("node", node);
  if (type) q.set("type", type);
  return q.toString();
}

export { consoleUrl } from "./consoleUrl";
