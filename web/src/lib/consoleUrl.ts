export function consoleUrl(opts: {
  vmid?: number;
  node: string;
  type?: string;
  kind: "vnc" | "term" | "spice";
  connection?: string;
  /** Node terminals only: run Proxmox's upgrade instead of a login shell. */
  cmd?: "upgrade";
  /** One-time root@pam grant that makes Proxmox start the upgrade. */
  grant?: string;
}) {
  const connection = opts.connection ?? "";
  if (opts.vmid) {
    const q = new URLSearchParams({ node: opts.node, kind: opts.kind });
    if (opts.type) q.set("type", opts.type);
    if (connection) q.set("connection", connection);
    return `/api/guests/${opts.vmid}/console?${q}`;
  }
  const q = new URLSearchParams({ kind: opts.kind });
  if (opts.cmd) q.set("cmd", opts.cmd);
  if (opts.grant) q.set("grant", opts.grant);
  if (connection) q.set("connection", connection);
  return `/api/nodes/${encodeURIComponent(opts.node)}/console?${q}`;
}
