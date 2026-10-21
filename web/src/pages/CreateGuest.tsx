import { attachTask } from "@/components/layout/AppShell";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useCan } from "@/lib/perms";
import { guestKeys } from "@/lib/tasks";
import { defaultStorage, storagesForContent } from "@/lib/storage";
import { useQuery } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useEffect, useMemo, useState } from "react";

const steps = ["Identity", "OS", "System", "Resources", "Network"] as const;

export function CreateGuestPage() {
  const search = useSearch({ from: "/app/create" });
  const [mode, setMode] = useState<"create" | "import">(search.import === "1" ? "import" : "create");
  const nav = useNavigate();
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const next = useQuery({ queryKey: ["nextid"], queryFn: api.nextid });
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const nodes = (summary.data?.resources ?? []).filter((r) => r.type === "node");
  const [step, setStep] = useState(0);
  const can = useCan();
  const canApl = can("/", "Sys.Audit") || can("/", "Datastore.Allocate") || can("/storage", "Datastore.Allocate");
  const [error, setError] = useState("");
  const [aplMsg, setAplMsg] = useState("");
  const [form, setForm] = useState({
    type: "qemu",
    node: "",
    vmid: 0,
    name: "",
    memory: 2048,
    cores: 2,
    diskGb: 32,
    storage: "",
    bridge: "vmbr0",
    iso: "",
    template: "",
    ostype: "l26",
    password: "",
    bios: "seabios",
    machine: "",
    scsihw: "virtio-scsi-pci",
    cpu: "",
    sockets: 1,
    vga: "std",
    tablet: true,
    args: "",
    agent: true,
    unprivileged: true,
    ciuser: "",
    sshkeys: "",
    ipconfig0: "ip=dhcp",
  });

  const media = useQuery({
    queryKey: ["media", form.type === "lxc" ? "vztmpl" : "iso", form.node],
    queryFn: () => api.media(form.type === "lxc" ? "vztmpl" : "iso", form.node || undefined),
    enabled: !!form.node,
  });
  const caps = useQuery({
    queryKey: ["caps", form.node],
    queryFn: () => api.capabilities(form.node),
    enabled: !!form.node && form.type === "qemu",
  });
  const apl = useQuery({
    queryKey: ["aplinfo", form.node],
    queryFn: () => api.nodeAPLInfo(form.node),
    enabled: !!form.node && canApl,
  });

  useEffect(() => {
    if (!form.node && nodes[0]?.node) setForm((f) => ({ ...f, node: nodes[0].node ?? "" }));
  }, [nodes, form.node]);
  useEffect(() => {
    if (!form.vmid && next.data?.vmid) setForm((f) => ({ ...f, vmid: next.data!.vmid }));
  }, [next.data, form.vmid]);
  const diskKind = form.type === "lxc" ? "rootdir" : "images";
  const diskStores = useMemo(
    () => storagesForContent(storages.data ?? [], diskKind, form.node),
    [storages.data, diskKind, form.node],
  );
  useEffect(() => {
    if (!diskStores.some((s) => s.storage === form.storage)) {
      setForm((f) => ({ ...f, storage: defaultStorage(storages.data ?? [], diskKind, form.node) }));
    }
  }, [diskStores, diskKind, form.node, form.storage, storages.data]);

  const machines = (caps.data?.machines ?? []).map((m) => String(m.id ?? m.name ?? "")).filter(Boolean);
  const cpus = (caps.data?.cpus ?? []).map((c) => String(c.name ?? c.id ?? "")).filter(Boolean);

  if (mode === "import") {
    return (
      <ImportWizard
        nodes={nodes.map((n) => n.node ?? "").filter(Boolean)}
        storages={storagesForContent(storages.data ?? [], "images").map((s) => s.storage)}
        initial={{ node: search.node || "", storage: search.storage || "", volume: search.volume || "" }}
        onBack={() => setMode("create")}
      />
    );
  }

  return (
    <div className="mx-auto max-w-2xl">
      <h1 className="text-2xl font-semibold">Create guest</h1>
      <p className="mt-1 text-sm text-ink-dim">Step {step + 1} of {steps.length}: {steps[step]}</p>
      <div className="mt-3 flex gap-2">
        <Button size="sm" variant="primary">Create</Button>
        <Button size="sm" variant="outline" onClick={() => setMode("import")}>Import ESXi / OVF</Button>
      </div>
      <div className="mt-4 flex gap-2">
        {steps.map((label, i) => (
          <button
            key={label}
            onClick={() => setStep(i)}
            className={`rounded-full px-3 py-1 text-xs ${i === step ? "bg-raised text-ink ring-1 ring-white/15" : "text-ink-dim"}`}
          >
            {label}
          </button>
        ))}
      </div>
      <Card className="mt-5 grid gap-4 p-6">
        {step === 0 && (
          <div className="grid gap-4 md:grid-cols-2">
            <Field label="Type">
              <Select value={form.type} onChange={(e) => setForm({ ...form, type: e.target.value })}>
                <option value="qemu">Virtual machine</option>
                <option value="lxc">Container</option>
              </Select>
            </Field>
            <Field label="Node">
              <Select value={form.node} onChange={(e) => setForm({ ...form, node: e.target.value })}>
                {nodes.map((n) => <option key={n.id} value={n.node}>{n.node}</option>)}
              </Select>
            </Field>
            <Field label="VMID">
              <Input type="number" value={form.vmid || ""} onChange={(e) => setForm({ ...form, vmid: Number(e.target.value) })} />
            </Field>
            <Field label="Name">
              <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
            </Field>
          </div>
        )}
        {step === 1 && (
          <div className="grid gap-4">
            {form.type === "qemu" ? (
              <>
                <Field label="OS type">
                  <Select value={form.ostype} onChange={(e) => setForm({ ...form, ostype: e.target.value })}>
                    <option value="l26">Linux 2.6+</option>
                    <option value="win11">Windows 11</option>
                    <option value="win10">Windows 10</option>
                    <option value="other">Other</option>
                  </Select>
                </Field>
                <Field label="ISO">
                  <Select value={form.iso} onChange={(e) => setForm({ ...form, iso: e.target.value })}>
                    <option value="">None</option>
                    {(media.data ?? []).map((m) => <option key={m.volid} value={m.volid}>{m.volid}</option>)}
                  </Select>
                </Field>
                {apl.data?.supported && <AppliancePicker type={form.type} node={form.node} storages={storages.data ?? []} items={apl.data.items} onMsg={setAplMsg} />}
              </>
            ) : (
              <>
                <Field label="OS template">
                  <Select value={form.template} onChange={(e) => setForm({ ...form, template: e.target.value })}>
                    <option value="">Select a template</option>
                    {(media.data ?? []).map((m) => <option key={m.volid} value={m.volid}>{m.volid}</option>)}
                  </Select>
                </Field>
                {apl.data?.supported && <AppliancePicker type={form.type} node={form.node} storages={storages.data ?? []} items={apl.data.items} onMsg={setAplMsg} />}
                <Field label="Root password">
                  <Input type="password" value={form.password} onChange={(e) => setForm({ ...form, password: e.target.value })} />
                </Field>
              </>
            )}
          </div>
        )}
        {step === 2 && (
          <div className="grid gap-4 md:grid-cols-2">
            {form.type === "qemu" ? (
              <>
                <Field label="BIOS">
                  <Select value={form.bios} onChange={(e) => setForm({ ...form, bios: e.target.value })}>
                    <option value="seabios">SeaBIOS</option>
                    <option value="ovmf">UEFI (OVMF)</option>
                  </Select>
                </Field>
                <Field label="Machine">
                  <Select value={form.machine} onChange={(e) => setForm({ ...form, machine: e.target.value })}>
                    <option value="">Default</option>
                    {machines.map((m) => <option key={m}>{m}</option>)}
                  </Select>
                </Field>
                <Field label="SCSI controller">
                  <Select value={form.scsihw} onChange={(e) => setForm({ ...form, scsihw: e.target.value })}>
                    <option value="virtio-scsi-pci">virtio-scsi-pci</option>
                    <option value="virtio-scsi-single">virtio-scsi-single</option>
                    <option value="lsi">lsi</option>
                  </Select>
                </Field>
                <Field label="CPU model">
                  <Select value={form.cpu} onChange={(e) => setForm({ ...form, cpu: e.target.value })}>
                    <option value="">Default</option>
                    {cpus.map((c) => <option key={c} value={c}>{c}</option>)}
                  </Select>
                </Field>
                <Field label="Sockets">
                  <Input type="number" value={form.sockets} onChange={(e) => setForm({ ...form, sockets: Number(e.target.value) })} />
                </Field>
                <Field label="Display">
                  <Select value={form.vga} onChange={(e) => setForm({ ...form, vga: e.target.value })}>
                    <option value="std">std</option>
                    <option value="qxl">qxl (SPICE)</option>
                    <option value="virtio">virtio</option>
                    <option value="virtio-gl">virtio-gl</option>
                  </Select>
                </Field>
                <label className="flex items-center gap-2 self-end text-sm">
                  <input type="checkbox" checked={form.tablet} onChange={(e) => setForm({ ...form, tablet: e.target.checked })} />
                  USB tablet
                </label>
                <label className="flex items-center gap-2 self-end text-sm">
                  <input type="checkbox" checked={form.agent} onChange={(e) => setForm({ ...form, agent: e.target.checked })} />
                  QEMU guest agent
                </label>
                <Field label="Extra QEMU args (advanced)">
                  <Input value={form.args} onChange={(e) => setForm({ ...form, args: e.target.value })} />
                </Field>
              </>
            ) : (
              <label className="flex items-center gap-2 text-sm">
                <input type="checkbox" checked={form.unprivileged} onChange={(e) => setForm({ ...form, unprivileged: e.target.checked })} />
                Unprivileged container
              </label>
            )}
          </div>
        )}
        {step === 3 && (
          <div className="grid gap-4 md:grid-cols-2">
            <Field label="Memory (MiB)">
              <Input type="number" value={form.memory} onChange={(e) => setForm({ ...form, memory: Number(e.target.value) })} />
            </Field>
            <Field label="Cores">
              <Input type="number" value={form.cores} onChange={(e) => setForm({ ...form, cores: Number(e.target.value) })} />
            </Field>
            <Field label="Disk (GiB)">
              <Input type="number" value={form.diskGb} onChange={(e) => setForm({ ...form, diskGb: Number(e.target.value) })} />
            </Field>
            <Field label="Storage">
              <Select value={form.storage} onChange={(e) => setForm({ ...form, storage: e.target.value })}>
                {diskStores.map((s) => <option key={s.storage} value={s.storage}>{s.storage}</option>)}
              </Select>
            </Field>
          </div>
        )}
        {step === 4 && (
          <div className="grid gap-4">
            <Field label="Bridge">
              <Input value={form.bridge} onChange={(e) => setForm({ ...form, bridge: e.target.value })} />
            </Field>
            <Field label="Cloud-init user">
              <Input value={form.ciuser} onChange={(e) => setForm({ ...form, ciuser: e.target.value })} />
            </Field>
            <Field label="ipconfig0">
              <Input value={form.ipconfig0} onChange={(e) => setForm({ ...form, ipconfig0: e.target.value })} />
            </Field>
            <Field label="SSH keys">
              <Textarea rows={3} value={form.sshkeys} onChange={(e) => setForm({ ...form, sshkeys: e.target.value })} />
            </Field>
          </div>
        )}
        {aplMsg && <p className="text-sm text-ink-dim">{aplMsg}</p>}
        {error && <p className="text-sm text-rose-400">{error}</p>}
        <div className="flex justify-between">
          <Button variant="ghost" disabled={step === 0} onClick={() => setStep((s) => s - 1)}>Back</Button>
          {step < steps.length - 1 ? (
            <Button onClick={() => setStep((s) => s + 1)}>Next</Button>
          ) : (
            <Button
              onClick={async () => {
                setError("");
                try {
                  const res = await api.createGuest(form);
                  attachTask(res.upid, [...guestKeys(res.vmid), ["nextid"]]);
                  nav({ to: "/guests/$vmid", params: { vmid: String(res.vmid) }, search: { node: form.node, type: form.type } });
                } catch (e) {
                  setError(e instanceof Error ? e.message : "Create failed");
                }
              }}
            >
              Create
            </Button>
          )}
        </div>
      </Card>
    </div>
  );
}

function AppliancePicker({
  type, node, storages, items, onMsg,
}: {
  type: string;
  node: string;
  storages: { storage: string; content?: string }[];
  items: Record<string, unknown>[];
  onMsg: (msg: string) => void;
}) {
  const [picked, setPicked] = useState("");
  const content = type === "lxc" ? "vztmpl" : "iso";
  const store = storages.find((s) => (s.content || "").includes(content))?.storage || storages[0]?.storage || "";
  const item = items[Number(picked)];
  const source = item ? String(item.location ?? item.source ?? item.url ?? "") : "";
  const filename = item ? String(item.template ?? item.package ?? source.split("/").pop() ?? "appliance") : "";
  return (
    <div className="grid gap-2">
      <Field label="Appliance (optional)">
        <Select value={picked} onChange={(e) => setPicked(e.target.value)}>
          <option value="">None</option>
          {items.map((a, i) => {
            const label = String(a.headline ?? a.package ?? a.template ?? i);
            return <option key={label + i} value={i}>{label}</option>;
          })}
        </Select>
      </Field>
      {source && store && (
        <Button size="sm" variant="outline" onClick={async () => {
          onMsg("");
          try {
            const res = await api.downloadUrl(store, node, { url: source, filename, content });
            attachTask(res.upid, [["content", store, node], ["storages"]]);
            onMsg(`Downloading ${filename} onto ${store}. After it finishes, pick it from the template/ISO list.`);
          } catch (e) {
            onMsg(e instanceof Error ? e.message : "Download failed");
          }
        }}>Download appliance</Button>
      )}
    </div>
  );
}

function asRecord(v: unknown): Record<string, unknown> {
  return v && typeof v === "object" && !Array.isArray(v) ? v as Record<string, unknown> : {};
}

function warningLines(raw: unknown): string[] {
  if (!Array.isArray(raw)) return [];
  return raw.map((item) => {
    const rec = asRecord(item);
    const kind = String(rec.type ?? rec.key ?? "warning");
    const key = rec.key ? ` (${String(rec.key)})` : "";
    return `${kind}${key}`;
  });
}

function netLine(raw: unknown, bridge: string): string {
  if (typeof raw === "string") {
    if (/bridge=/.test(raw)) return raw.replace(/bridge=[^,]+/, `bridge=${bridge}`);
    return raw ? `${raw},bridge=${bridge}` : `virtio,bridge=${bridge}`;
  }
  const rec = asRecord(raw);
  const model = String(rec.model ?? "virtio") || "virtio";
  const mac = rec.macaddr || rec.mac;
  return [model, `bridge=${bridge}`, mac ? `macaddr=${mac}` : ""].filter(Boolean).join(",");
}

function ImportWizard({
  nodes, storages, initial, onBack,
}: {
  nodes: string[];
  storages: string[];
  initial: { node: string; storage: string; volume: string };
  onBack: () => void;
}) {
  const nav = useNavigate();
  const next = useQuery({ queryKey: ["nextid"], queryFn: api.nextid });
  const [form, setForm] = useState({
    node: initial.node || nodes[0] || "",
    storage: initial.storage || storages[0] || "",
    volume: initial.volume,
    dest: storages[0] || "",
    working: "",
    vmid: 0,
    name: "",
    memory: 0,
    cores: 0,
    bridge: "vmbr0",
  });
  const [meta, setMeta] = useState<Record<string, unknown> | null>(null);
  const [createArgs, setCreateArgs] = useState<Record<string, unknown>>({});
  const [disks, setDisks] = useState<{ slot: string; src: string; storage: string; include: boolean }[]>([]);
  const [nets, setNets] = useState<Record<string, unknown>>({});
  const [bridges, setBridges] = useState<Record<string, string>>({});
  const [warnings, setWarnings] = useState<string[]>([]);
  const [dump, setDump] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const volumes = useQuery({
    queryKey: ["content", form.storage, form.node, "import-wizard"],
    queryFn: () => api.storageContent(form.storage, form.node).catch(() => []),
    enabled: !!form.storage && !!form.node,
  });

  useEffect(() => {
    if (!form.node && nodes[0]) setForm((f) => ({ ...f, node: nodes[0] }));
  }, [nodes, form.node]);
  useEffect(() => {
    if (!form.vmid && next.data?.vmid) setForm((f) => ({ ...f, vmid: next.data!.vmid }));
  }, [next.data, form.vmid]);
  useEffect(() => {
    if (!form.storage && storages[0]) setForm((f) => ({ ...f, storage: storages[0], dest: storages[0] }));
  }, [storages, form.storage]);

  async function loadMeta() {
    setError("");
    setLoading(true);
    try {
      const res = await api.importMetadata(form.storage, form.node, form.volume);
      if (!res.supported) {
        setMeta(null);
        setError("Import metadata is not available on this node.");
        return;
      }
      const data = res.data ?? {};
      setMeta(data);
      setDump(JSON.stringify(data, null, 2));
      const args = asRecord(data["create-args"]);
      setCreateArgs(args);
      const dest = form.dest || storages[0] || "";
      setDisks(Object.entries(asRecord(data.disks)).map(([slot, src]) => ({
        slot,
        src: String(src ?? ""),
        storage: dest,
        include: true,
      })));
      const netMap = asRecord(data.net);
      setNets(netMap);
      setBridges(Object.fromEntries(Object.keys(netMap).map((k) => [k, form.bridge])));
      setWarnings(warningLines(data.warnings));
      setForm((f) => ({
        ...f,
        name: String(args.name ?? f.name),
        memory: Number(args.memory ?? f.memory) || f.memory,
        cores: Number(args.cores ?? f.cores) || f.cores,
      }));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Metadata failed");
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="mx-auto max-w-2xl">
      <h1 className="text-2xl font-semibold">Import guest</h1>
      <p className="mt-1 text-sm text-ink-dim">ESXi VM or OVA/OVF on storage. Loads import-metadata, then POST qemu with import-from.</p>
      <div className="mt-3 flex gap-2">
        <Button size="sm" variant="outline" onClick={onBack}>Create</Button>
        <Button size="sm" variant="primary">Import ESXi / OVF</Button>
      </div>
      <Card className="mt-5 grid gap-4 p-6">
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="Node">
            <Select value={form.node} onChange={(e) => setForm({ ...form, node: e.target.value })}>
              {nodes.map((n) => <option key={n}>{n}</option>)}
            </Select>
          </Field>
          <Field label="Source storage">
            <Select value={form.storage} onChange={(e) => setForm({ ...form, storage: e.target.value })}>
              {storages.map((s) => <option key={s}>{s}</option>)}
            </Select>
          </Field>
          <Field label="Volume">
            <Select value={form.volume} onChange={(e) => setForm({ ...form, volume: e.target.value })}>
              <option value="">Select…</option>
              {(volumes.data ?? []).map((v) => (
                <option key={v.volid} value={v.volid}>{v.volid}</option>
              ))}
            </Select>
          </Field>
          <Field label="Or type volume">
            <Input value={form.volume} onChange={(e) => setForm({ ...form, volume: e.target.value })} placeholder="store:import/guest.ova" />
          </Field>
        </div>
        <Button size="sm" variant="outline" disabled={!form.node || !form.storage || !form.volume || loading} onClick={() => void loadMeta()}>
          {loading ? "Loading…" : "Load metadata"}
        </Button>
        {warnings.length > 0 && (
          <ul className="list-disc space-y-1 pl-5 text-sm text-amber-300">
            {warnings.map((w) => <li key={w}>{w}</li>)}
          </ul>
        )}
        {meta && (
          <>
            <div className="grid gap-4 md:grid-cols-2">
              <Field label="VMID">
                <Input type="number" value={form.vmid || ""} onChange={(e) => setForm({ ...form, vmid: Number(e.target.value) })} />
              </Field>
              <Field label="Name">
                <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
              </Field>
              <Field label="Memory (MiB)">
                <Input type="number" value={form.memory || ""} onChange={(e) => setForm({ ...form, memory: Number(e.target.value) })} />
              </Field>
              <Field label="Cores">
                <Input type="number" value={form.cores || ""} onChange={(e) => setForm({ ...form, cores: Number(e.target.value) })} />
              </Field>
              <Field label="Destination storage">
                <Select
                  value={form.dest}
                  onChange={(e) => {
                    const dest = e.target.value;
                    setForm({ ...form, dest });
                    setDisks((rows) => rows.map((d) => !d.storage || d.storage === form.dest ? { ...d, storage: dest } : d));
                  }}
                >
                  {storages.map((s) => <option key={s}>{s}</option>)}
                </Select>
              </Field>
              <Field label="Working storage">
                <Select value={form.working} onChange={(e) => setForm({ ...form, working: e.target.value })}>
                  <option value="">Source storage</option>
                  {storages.map((s) => <option key={s}>{s}</option>)}
                </Select>
              </Field>
            </div>
            {disks.length > 0 && (
              <div className="space-y-2">
                <h2 className="text-sm font-medium">Disks</h2>
                <p className="text-xs text-ink-dim">Remap bus slot or destination storage, or skip a disk.</p>
                <div className="space-y-3">
                  {disks.map((row, i) => (
                    <div key={`${row.src}-${i}`} className="grid gap-3 rounded-md border border-white/10 p-3 md:grid-cols-[7rem_1fr_10rem_auto]">
                      <Field label="Slot">
                        <Input
                          value={row.slot}
                          onChange={(e) => setDisks(disks.map((d, j) => j === i ? { ...d, slot: e.target.value } : d))}
                        />
                      </Field>
                      <Field label="Source">
                        <Input value={row.src} readOnly className="font-mono text-xs" />
                      </Field>
                      <Field label="Storage">
                        <Select
                          value={row.storage}
                          onChange={(e) => setDisks(disks.map((d, j) => j === i ? { ...d, storage: e.target.value } : d))}
                        >
                          {storages.map((s) => <option key={s}>{s}</option>)}
                        </Select>
                      </Field>
                      <label className="flex items-end gap-2 pb-2 text-sm">
                        <input
                          type="checkbox"
                          checked={row.include}
                          onChange={(e) => setDisks(disks.map((d, j) => j === i ? { ...d, include: e.target.checked } : d))}
                        />
                        Import
                      </label>
                    </div>
                  ))}
                </div>
              </div>
            )}
            {Object.keys(nets).length > 0 && (
              <div className="grid gap-3">
                <h2 className="text-sm font-medium">Network</h2>
                {Object.keys(nets).map((slot) => (
                  <Field key={slot} label={`${slot} bridge`}>
                    <Input
                      value={bridges[slot] ?? form.bridge}
                      onChange={(e) => setBridges({ ...bridges, [slot]: e.target.value })}
                    />
                  </Field>
                ))}
              </div>
            )}
            {Object.keys(nets).length === 0 && (
              <Field label="Bridge">
                <Input value={form.bridge} onChange={(e) => setForm({ ...form, bridge: e.target.value })} />
              </Field>
            )}
            {dump && (
              <details>
                <summary className="cursor-pointer text-xs text-ink-dim">Raw import-metadata</summary>
                <pre className="mt-2 max-h-48 overflow-auto font-mono text-xs text-ink-dim">{dump}</pre>
              </details>
            )}
          </>
        )}
        {meta && !disks.length && (
          <p className="text-sm text-amber-300">Metadata has no disks. This volume may not be an importable guest.</p>
        )}
        {error && <p className="text-sm text-rose-400">{error}</p>}
        <div className="flex justify-end">
          <Button
            disabled={!meta || !form.vmid || !disks.some((d) => d.include && d.slot && d.src)}
            onClick={async () => {
              setError("");
              try {
                const selected = disks.filter((d) => d.include && d.slot && d.src);
                const importFrom = Object.fromEntries(selected.map((d) => [d.slot, d.src]));
                const importDest = Object.fromEntries(
                  selected.filter((d) => d.storage && d.storage !== form.dest).map((d) => [d.slot, d.storage]),
                );
                const netBody = Object.keys(nets).length
                  ? Object.fromEntries(Object.entries(nets).map(([slot, raw]) => [slot, netLine(raw, bridges[slot] || form.bridge)]))
                  : { net0: `virtio,bridge=${form.bridge}` };
                const res = await api.createGuest({
                  type: "qemu",
                  node: form.node,
                  vmid: form.vmid,
                  name: form.name,
                  memory: form.memory || undefined,
                  cores: form.cores || undefined,
                  storage: form.dest,
                  createArgs,
                  importFrom,
                  ...(Object.keys(importDest).length ? { importDest } : {}),
                  importWorkingStorage: form.working || undefined,
                  nets: netBody,
                });
                attachTask(res.upid, [...guestKeys(res.vmid), ["nextid"]]);
                nav({ to: "/guests/$vmid", params: { vmid: String(res.vmid) }, search: { node: form.node, type: "qemu" } });
              } catch (e) {
                setError(e instanceof Error ? e.message : "Import failed");
              }
            }}
          >
            Import
          </Button>
        </div>
      </Card>
    </div>
  );
}
