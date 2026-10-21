export type Resource = {
  id: string;
  type: string;
  node?: string;
  name?: string;
  storage?: string;
  vmid?: number;
  status?: string;
  cpu: number;
  maxcpu: number;
  mem: number;
  maxmem: number;
  disk: number;
  maxdisk: number;
  uptime: number;
  template: number;
  pool?: string;
  tags?: string;
  hastate?: string;
  netin: number;
  netout: number;
  diskread: number;
  diskwrite: number;
  content?: string;
  pending?: boolean;
  lock?: string;
};

export type ClusterStatus = {
  id: string;
  name?: string;
  type: string;
  ip?: string;
  online: number;
  local: number;
  quorate: number;
  nodes: number;
};

export type GuestStatus = {
  status: string;
  vmid: number;
  name?: string;
  cpu: number;
  cpus: number;
  mem: number;
  maxmem: number;
  disk: number;
  maxdisk: number;
  uptime: number;
  netin: number;
  netout: number;
  agent?: number;
  ha?: Record<string, unknown>;
};

export type GuestRef = {
  vmid: number;
  node: string;
  type: string;
  name?: string;
};

export type MediaItem = {
  volid: string;
  content: string;
  storage: string;
  node: string;
  size: number;
};

export type NodeCapabilities = {
  machines: Record<string, unknown>[];
  cpus: Record<string, unknown>[];
};

export type PendingChange = {
  key: string;
  value?: unknown;
  pending?: unknown;
  delete?: number;
};

export type Snapshot = {
  name: string;
  description?: string;
  snaptime?: number;
  vmstate?: number;
  parent?: string;
};

export type Task = {
  upid: string;
  node: string;
  type: string;
  id: string;
  user: string;
  status?: string;
  starttime: number;
  endtime?: number;
};

export type TaskLogLine = { n: number; t: string };

export type Storage = {
  storage: string;
  type: string;
  content: string;
  shared?: number;
  disable?: number;
  nodes?: string;
};

export type StorageContent = {
  volid: string;
  content: string;
  format?: string;
  size: number;
  ctime?: number;
  vmid?: number;
  notes?: string;
};

export type Connection = {
  id: string;
  name: string;
  pveUrl: string;
  tlsMode: string;
  enabled: boolean;
  online?: boolean;
  error?: string;
  nodes?: string[];
  clusterName?: string;
};

export type InventoryGroup = {
  id: string;
  name: string;
  status?: ClusterStatus[];
  resources: Resource[];
};

export type ScriptField = {
  name: string;
  label: string;
  type: string;
  default?: unknown;
  help?: string;
  optional?: boolean;
  advanced?: boolean;
};

export type ScriptRecipe = {
  id: string;
  title: string;
  summary: string;
  category: "templates" | "guests" | "nodes" | "protection";
  kind: string;
  family?: string;
  fields: ScriptField[];
};

export type ScriptPlan = {
  steps: { id: string; label: string }[];
  notes?: string[];
  confirm?: string;
  vmid?: number;
};

export type ScriptJobStep = {
  id: string;
  label: string;
  status: "pending" | "running" | "done" | "failed" | "skipped";
  upids?: string[];
  note?: string;
  error?: string;
};

export type ScriptJob = {
  id: string;
  recipe: string;
  title: string;
  status: "running" | "done" | "partial" | "failed";
  steps: ScriptJobStep[];
  started: number;
  finished?: number;
  message?: string;
  vmid?: number;
};

export type AuditEvent = {
  id: number;
  at: string;
  username: string;
  userKind: string;
  connectionId: string;
  method: string;
  path: string;
  status: number;
  upid?: string;
};

export type HistoryPoint = {
  time: string;
  cpu: number;
  mem: number;
  maxmem: number;
  disk: number;
  maxdisk: number;
  netin: number;
  netout: number;
};

export type Me = {
  setupRequired: boolean;
  clusterConfigured: boolean;
  user: { id: string; username: string; kind?: "local" | "pve"; pveUserid?: string } | null;
  permissions?: Record<string, Record<string, number>>;
  /** "token": a local admin's view, limited to what the connection's API token may do. */
  permissionsFrom?: "token";
  connections?: Connection[];
  needTfa?: boolean;
  tfaChallenge?: string;
};

export type AuthRealm = {
  realm: string;
  type: string;
  comment?: string;
};

export type AccessUser = {
  userid: string;
  comment?: string;
  email?: string;
  enable: number;
  expire?: number;
  firstname?: string;
  lastname?: string;
  groups?: string;
};

export type AccessToken = {
  tokenid: string;
  comment?: string;
  expire?: number;
  privsep?: number;
  value?: string;
};

export type TFAEntry = {
  id: string;
  type: string;
  enable?: number;
  created?: number;
  description?: string;
};

export type TFAEnroll = {
  id?: string;
  type?: string;
  uri?: string;
  qrcode?: string;
  recovery?: string[];
};

export type AccessGroup = {
  group: string;
  comment?: string;
  users?: string;
};

export type AccessRole = {
  roleid: string;
  privs?: string;
};

export type ACLEntry = {
  path: string;
  ugid: string;
  type: string;
  roleid: string;
  propagate?: number;
};

export type RingSample = {
  ts: number;
  cpu?: number;
  mem?: number;
  maxmem?: number;
  disk?: number;
  maxdisk?: number;
  netin?: number;
  netout?: number;
};

export type BackupJob = {
  id: string;
  enabled: number;
  schedule?: string;
  storage?: string;
  mode?: string;
  compress?: string;
  vmid?: string;
  all?: number;
  exclude?: string;
  node?: string;
  "notes-template"?: string;
  mailto?: string;
  mailnotification?: string;
  "notification-mode"?: string;
  "prune-backups"?: string;
  comment?: string;
  fleecing?: string;
  bwlimit?: string | number;
  "pbs-change-detection-mode"?: string;
};

export type BackupJobRequest = {
  id?: string;
  enabled?: boolean;
  schedule?: string;
  storage?: string;
  mode?: string;
  compress?: string;
  vmid?: string;
  all?: boolean;
  exclude?: string;
  node?: string;
  notes?: string;
  mailto?: string;
  mailnotification?: string;
  "notification-mode"?: string;
  pruneBackups?: string;
  comment?: string;
  fleecing?: string;
  bwlimit?: string;
  pbsChangeDetection?: string;
};

export type IncludedVolume = {
  id?: unknown;
  type?: string;
  name?: string;
  included?: number;
  leaf?: number;
  children?: IncludedVolume[];
};

export type CoverageGuest = {
  vmid: number;
  name?: string;
  type: string;
};

export type GuestBackupInfo = {
  vmid: number;
  name?: string;
  type: string;
  node?: string;
  lastBackup?: number;
  lastVolid?: string;
  inJob: boolean;
};

export type CoverageReport = {
  notBackedUp: CoverageGuest[];
  guests: GuestBackupInfo[];
};

export type FileRestoreEntry = {
  filepath: string;
  text?: string;
  type?: string;
  leaf?: boolean | number;
  size?: number;
  mtime?: number;
};

export type PrunePreview = {
  volid: string;
  ctime?: number;
  mark?: string;
  type?: string;
  vmid?: number;
  size?: number;
};

export type ReplicationJob = {
  id: string;
  guest: number;
  target: string;
  schedule?: string;
  type?: string;
  disable?: number;
  comment?: string;
  rate?: number;
  source?: string;
};

export type ReplicationStatus = {
  id?: string;
  guest?: number;
  last_sync?: number;
  duration?: number;
  fail_count?: number;
  error?: string;
  pid?: number;
};

export type HACurrent = {
  type: string;
  sid?: string;
  state?: string;
  status?: string;
  node?: string;
  request?: string;
  quorate?: number;
  crm_state?: string;
  id?: string;
};

export type HAStatus = {
  current: HACurrent[];
  manager?: unknown;
};

export type HAResource = {
  sid: string;
  type?: string;
  state?: string;
  group?: string;
  max_restart?: number;
  max_relocate?: number;
  comment?: string;
};

export type HAGroup = {
  group: string;
  nodes?: string;
  restricted?: number;
  nofailback?: number;
  comment?: string;
};

export type HARule = {
  rule: string;
  type?: string;
  resources?: string;
  nodes?: string;
  affinity?: string;
  strict?: number;
  disable?: number;
  comment?: string;
};

export type HARulesResponse = {
  supported: boolean;
  rules: HARule[];
};

export type FWRule = {
  pos: number;
  action?: string;
  type?: string;
  enable: number;
  source?: string;
  dest?: string;
  proto?: string;
  dport?: string;
  sport?: string;
  macro?: string;
  iface?: string;
  comment?: string;
  log?: string;
  digest?: string;
};

export type FWOptions = {
  enable: number;
  policy_in?: string;
  policy_out?: string;
  log_level_in?: string;
  log_level_out?: string;
  ebtables?: number;
  ndp?: number;
  dhcp?: number;
  radv?: number;
  macfilter?: number;
  ipfilter?: number;
  digest?: string;
};

export type FWAlias = {
  name: string;
  cidr?: string;
  comment?: string;
  digest?: string;
};

export type FWIPSet = {
  name: string;
  comment?: string;
  digest?: string;
};

export type FWIPSetCIDR = {
  cidr: string;
  nomatch?: number;
  comment?: string;
  digest?: string;
};

export type FWGroup = {
  group: string;
  comment?: string;
  digest?: string;
};

export type FWMacro = {
  macro: string;
  descr?: string;
};

export type FWRef = {
  ref: string;
  type?: string;
  name?: string;
  comment?: string;
};

export type FWRefsResponse = {
  supported: boolean;
  refs: FWRef[];
};

export type FWLogLine = {
  n: number;
  t: string;
};

export type FeatureList = {
  supported: boolean;
  items: Record<string, unknown>[];
};

export type FeatureObject = {
  supported: boolean;
  item?: Record<string, unknown>;
};

export type FeatureValue = {
  supported: boolean;
  value?: string;
};

export type DiskRescanResult = {
  supported: boolean;
  upid?: string;
};

export type NetIface = {
  iface: string;
  type?: string;
  active: number;
  address?: string;
  cidr?: string;
  gateway?: string;
  bridge_ports?: string;
  comments?: string;
  method?: string;
  autostart?: number;
  exists?: number;
};

export type DiskInfo = {
  devpath: string;
  size: number;
  used?: string;
  gpt: number;
  osdid: number;
  model?: string;
  serial?: string;
  vendor?: string;
  type?: string;
  health?: string;
};

export type NodeReport = {
  supported: boolean;
  report: string;
};

export type WakeOnLANResult = {
  supported: boolean;
  mac?: string;
};

export type ZFSPoolStatus = {
  supported: boolean;
  pool?: Record<string, unknown>;
};

export type DiskTaskResult = {
  supported: boolean;
  upid?: string;
};

export type NodeService = {
  service: string;
  name?: string;
  state?: string;
  desc?: string;
  "unit-state"?: string;
};

export type NodeDNS = {
  search?: string;
  dns1?: string;
  dns2?: string;
  dns3?: string;
};

export type NodeTime = {
  timezone?: string;
  time?: number;
  localtime?: number;
};

export type NodeHosts = {
  data?: string;
  digest?: string;
};

export type JournalLine = {
  n: number;
  t: string;
};

export type NodeSubscription = {
  status?: string;
  level?: string;
  key?: string;
  message?: string;
  productname?: string;
  serverid?: string;
  sockets?: number;
  nextduedate?: string;
};

export type CertInfo = {
  filename?: string;
  fingerprint?: string;
  issuer?: string;
  subject?: string;
  /** Unix seconds. */
  notbefore?: number;
  /** Unix seconds. */
  notafter?: number;
};

export type Pool = {
  poolid: string;
  comment?: string;
  members?: Record<string, unknown>[];
};

export type SDNStatus = {
  supported: boolean;
  data?: Record<string, unknown>;
};

export type SDNZone = {
  zone: string;
  type?: string;
  bridge?: string;
  mtu?: number;
  nodes?: string;
  ipam?: string;
  dns?: string;
  pending?: unknown;
  digest?: string;
};

export type SDNVNet = {
  vnet: string;
  zone?: string;
  tag?: number;
  alias?: string;
  vlanaware?: number | boolean;
  pending?: unknown;
  digest?: string;
};

export type SDNSubnet = {
  subnet: string;
  gateway?: string;
  snat?: number;
  vnet?: string;
  "dhcp-range"?: string;
  "dhcp-dns-server"?: string;
  pending?: unknown;
  digest?: string;
};

export type CephCmdSafety = {
  supported: boolean;
  safe: boolean;
  status?: string;
};

export type NodeSDNRuntime = {
  supported: boolean;
  zones?: Record<string, unknown>[];
  neighbors?: Record<string, unknown>[];
  routes?: Record<string, unknown>[];
};

export type CephStatus = {
  supported: boolean;
  data?: Record<string, unknown>;
};

export type CephFlags = {
  supported: boolean;
  flags?: Record<string, unknown>;
};

export type CephMon = {
  name: string;
  host?: string;
  addr?: string;
  state?: string;
};

export type CephMgr = {
  name: string;
  host?: string;
  state?: string;
  addr?: string;
};

export type CephMDS = {
  name: string;
  host?: string;
  state?: string;
  addr?: string;
};

export type CephOSD = {
  osdid: number;
  id?: number;
  name?: string;
  host?: string;
  status?: string;
  in: number;
  weight?: number;
  crush_device_class?: string;
};

export type CephPool = {
  pool?: number;
  pool_name: string;
  size?: number;
  min_size?: number;
  pg_num?: number;
  crush_rule?: unknown;
  crush_rule_name?: string;
  application?: string;
};

export type APTRepositories = {
  digest?: string;
  files?: { path?: string; repositories?: Record<string, unknown>[] }[];
};

export function haSid(type: string, vmid: number) {
  return type === "lxc" || type === "ct" ? `ct:${vmid}` : `vm:${vmid}`;
}

export function haTone(state?: string): "green" | "amber" | "rose" | "zinc" | "teal" {
  const s = (state || "").toLowerCase();
  if (s === "started" || s === "online" || s === "active") return "green";
  if (s === "error" || s === "recovery" || s === "fence") return "rose";
  if (s === "migrate" || s === "relocate" || s === "queued") return "amber";
  if (s) return "teal";
  return "zinc";
}

export type InsightSeverity = "action" | "notice";

/** Where an insight's action leads. */
export type InsightTarget = {
  kind: "guest" | "node" | "storage" | "backups" | "ha" | "task" | "datacenter";
  node?: string;
  vmid?: number;
  type?: string;
  tab?: string;
  storage?: string;
  upid?: string;
};

export type InsightSubject = {
  kind: "guest" | "node" | "storage";
  node?: string;
  vmid?: number;
  type?: string;
  name?: string;
};

export type Insight = {
  id: string;
  severity: InsightSeverity;
  title: string;
  detail: string;
  action?: string;
  target?: InsightTarget;
  subjects?: InsightSubject[];
};

export type FailoverNode = {
  node: string;
  /** Configured memory of the node's running guests, bytes. */
  required: number;
  haRequired: number;
  /** Free memory on the other online nodes, bytes. */
  available: number;
  fits: boolean;
  haFits: boolean;
};

export type InsightsReport = {
  generatedAt: number;
  insights: Insight[];
  failover?: { nodes: FailoverNode[] };
  backups?: { lastRun: number; ok: number; failed: number; running: boolean };
  /** VMID → start of the latest recent run that backed the guest up. */
  lastBackups?: Record<string, number>;
  /** Checks that could not run, e.g. for lack of permission. */
  unavailable?: string[];
};
