import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Field, Input, Select, Textarea } from "@/components/ui/input";
import { Tabs } from "@/components/ui/tabs";
import { api, ApiError } from "@/lib/api";
import { defaultStorage, storagesForContent } from "@/lib/storage";
import { useReadOnly } from "@/lib/perms";
import { fleetKeys, openTasks } from "@/lib/tasks";
import { cn } from "@/lib/utils";
import type { Resource, ScriptField, ScriptJob, ScriptJobStep, ScriptRecipe } from "@/types";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Check, ChevronRight, Circle, Loader2, Minus, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState, type ReactNode } from "react";

const CATEGORIES = ["templates", "guests", "nodes", "protection"] as const;
type Category = (typeof CATEGORIES)[number];

const CATEGORY_LABEL: Record<Category, string> = {
  templates: "Templates",
  guests: "Guests",
  nodes: "Nodes",
  protection: "Backups & snapshots",
};

const CATEGORY_INTRO: Record<Category, string> = {
  templates: "Turn a distribution's cloud image into a template with cloud-init, ready to clone.",
  guests: "Create guests from templates and tidy up the ones you have.",
  nodes: "Maintenance across a node or the whole cluster.",
  protection: "Close backup gaps and keep snapshots under control.",
};

// Field types whose value is a VMID.
const VMID_FIELDS = new Set(["guest", "qemu-guest", "template"]);

type FormValue = string | number | boolean;
type Form = Record<string, FormValue>;

/**
 * Recipes: curated jobs that run as ordinary Proxmox calls with your own
 * permissions. Every recipe shows the exact steps before it runs, and each run
 * reports its progress step by step.
 */
export function RecipesPage() {
  const recipes = useQuery({ queryKey: ["scripts"], queryFn: api.scripts });
  const [category, setCategory] = useState<Category>("templates");
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [jobId, setJobId] = useState<string | null>(null);
  const all = recipes.data?.recipes ?? [];
  const selected = all.find((r) => r.id === selectedId) ?? null;
  const inCategory = all.filter((r) => r.category === category);

  const groups = useMemo(() => {
    const out: { family: string; items: ScriptRecipe[] }[] = [];
    for (const r of inCategory) {
      const family = r.family ?? "";
      let g = out.find((x) => x.family === family);
      if (!g) out.push((g = { family, items: [] }));
      g.items.push(r);
    }
    return out;
  }, [inCategory]);

  return (
    <div className="space-y-5">
      <header>
        <h1 className="text-2xl font-semibold tracking-tight">Recipes</h1>
        <p className="mt-1 text-sm text-ink-dim">
          Common jobs, done the careful way. Each one checks your choices and shows exactly what it will do before it runs.
        </p>
      </header>
      <Tabs
        items={CATEGORIES}
        value={category}
        label={(c) => CATEGORY_LABEL[c]}
        onChange={(c) => {
          setCategory(c);
          setSelectedId(null);
          setJobId(null);
        }}
      />
      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,30rem)]">
        <div className="min-w-0 space-y-5">
          <p className="text-sm text-ink-dim">{CATEGORY_INTRO[category]}</p>
          {recipes.isLoading && <p className="text-sm text-ink-dim">Loading recipes…</p>}
          {recipes.error && <p className="text-sm text-danger">Couldn't load recipes: {(recipes.error as Error).message}</p>}
          {groups.map((g) => (
            <section key={g.family || "all"} className="space-y-2">
              {g.family && <h2 className="text-xs font-medium text-ink-dim">{g.family}</h2>}
              <div className="grid gap-2 sm:grid-cols-2">
                {g.items.map((r) => (
                  <RecipeCard
                    key={r.id}
                    recipe={r}
                    active={r.id === selectedId}
                    onSelect={() => {
                      setSelectedId(r.id);
                      setJobId(null);
                    }}
                  />
                ))}
              </div>
            </section>
          ))}
        </div>
        <div className="min-w-0 lg:sticky lg:top-4 lg:self-start">
          {jobId ? (
            <JobView id={jobId} onClose={() => setJobId(null)} />
          ) : selected ? (
            <RecipeForm key={selected.id} recipe={selected} sources={all.filter((r) => r.kind === "cloud-template")} onStarted={setJobId} />
          ) : (
            <RecentRuns onOpen={setJobId} />
          )}
        </div>
      </div>
    </div>
  );
}

function RecipeCard({ recipe, active, onSelect }: { recipe: ScriptRecipe; active: boolean; onSelect: () => void }) {
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={active}
      className={cn(
        "group flex items-start gap-3 rounded-lg border px-4 py-3 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/40",
        active ? "border-accent/60 bg-raised" : "border-line hover:border-white/20 hover:bg-white/[0.02]",
      )}
    >
      <div className="min-w-0 flex-1">
        <span className="text-sm font-medium">{recipe.title}</span>
        <p className="mt-1 line-clamp-2 text-[13px] leading-5 text-ink-dim">{recipe.summary}</p>
      </div>
      <ChevronRight className="mt-0.5 h-4 w-4 shrink-0 text-ink-faint group-hover:text-ink-dim" aria-hidden />
    </button>
  );
}

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return v;
}

function RecipeForm({ recipe, sources, onStarted }: { recipe: ScriptRecipe; sources: ScriptRecipe[]; onStarted: (id: string) => void }) {
  const readOnly = useReadOnly();
  const summary = useQuery({ queryKey: ["summary"], queryFn: api.summary });
  const storages = useQuery({ queryKey: ["storages"], queryFn: api.storages });
  const resources = summary.data?.resources ?? [];
  const nodes = useMemo(
    () =>
      resources
        .filter((r) => r.type === "node" && r.status === "online")
        .map((r) => r.node ?? "")
        .sort((a, b) => a.localeCompare(b, undefined, { numeric: true })),
    [resources],
  );
  const [form, setForm] = useState<Form>({});
  const [confirming, setConfirming] = useState(false);
  const [busy, setBusy] = useState(false);
  const [runError, setRunError] = useState("");

  // What the form shows: the user's values over the recipe's defaults.
  const values = useMemo(() => {
    const out: Form = {};
    for (const f of recipe.fields ?? []) {
      if (f.default !== undefined) out[f.name] = f.default as FormValue;
      if (f.type === "node" && !f.optional) out[f.name] = nodes[0] ?? "";
      if (f.type === "node" && f.optional) out[f.name] = "";
    }
    Object.assign(out, form);
    const node = String(out.node ?? "") || undefined;
    for (const f of recipe.fields ?? []) {
      if (f.type === "target-node" && !out[f.name]) out[f.name] = nodes.find((n) => n !== node) ?? "";
      if ((f.type === "storage" || f.type === "backup-storage") && !f.optional && !out[f.name]) {
        out[f.name] = defaultStorage(storages.data ?? [], f.type === "storage" ? "images" : "backup", node);
      }
    }
    return out;
  }, [recipe, form, nodes, storages.data]);

  const params = useMemo(() => toParams(recipe.fields ?? [], values), [recipe.fields, values]);
  const debounced = useDebounced(params, 350);
  const plan = useQuery({
    queryKey: ["script-plan", recipe.id, debounced],
    queryFn: () => api.scriptPlan(recipe.id, debounced),
    enabled: summary.isSuccess && storages.isSuccess,
    retry: false,
    placeholderData: (prev) => prev,
  });
  const stale = plan.isFetching || debounced !== params;
  const planError = plan.error instanceof ApiError ? plan.error.message : plan.error ? "Couldn't check this recipe" : "";
  // "Choose a template" is the next thing to do, not a problem.
  const planHint = /^(choose|enter) /i.test(planError);

  const set = (name: string, v: FormValue) => setForm((cur) => ({ ...cur, [name]: v }));

  async function run() {
    setBusy(true);
    setRunError("");
    try {
      const job = await api.scriptRun(recipe.id, params);
      onStarted(job.id);
    } catch (e) {
      setRunError(e instanceof Error ? e.message : "Couldn't start the recipe");
    } finally {
      setBusy(false);
    }
  }

  const basic = (recipe.fields ?? []).filter((f) => !f.advanced);
  const advanced = (recipe.fields ?? []).filter((f) => f.advanced);
  const fieldProps = { values, set, resources, nodes, storages: storages.data ?? [], sources, recipe };

  return (
    <Card className="p-5">
      <h2 className="text-base font-semibold">{recipe.title}</h2>
      <p className="mt-1 text-sm leading-6 text-ink-dim">{recipe.summary}</p>

      {(recipe.fields ?? []).length > 0 && (
        <div className="mt-4 space-y-3">
          {basic.map((f) => (
            <RecipeField key={f.name} field={f} {...fieldProps} />
          ))}
          {advanced.length > 0 && (
            <details className="group rounded-md border border-line px-3 py-2">
              <summary className="cursor-pointer select-none text-sm text-ink-dim hover:text-ink">More options</summary>
              <div className="mt-3 space-y-3 pb-1">
                {advanced.map((f) => (
                  <RecipeField key={f.name} field={f} {...fieldProps} />
                ))}
              </div>
            </details>
          )}
        </div>
      )}

      <div className="mt-5 border-t border-line pt-4">
        <p className="flex items-center gap-2 text-xs font-medium text-ink-dim">
          What will happen
          {stale && <Loader2 className="h-3 w-3 animate-spin" aria-label="Checking" />}
        </p>
        {planError ? (
          <p className={cn("mt-2 text-sm", planHint ? "text-ink-dim" : "text-warn")}>{capitalize(planError)}</p>
        ) : plan.data ? (
          <>
            <ol className="mt-2 space-y-1.5 text-sm">
              {plan.data.steps.map((s, i) => (
                <li key={s.id} className="flex gap-2.5">
                  <span className="w-4 shrink-0 text-right font-mono text-xs leading-5 text-ink-faint">{i + 1}</span>
                  <span className="leading-5">{s.label}</span>
                </li>
              ))}
            </ol>
            {plan.data.notes?.map((n) => (
              <p key={n} className="mt-2 text-xs leading-5 text-ink-dim">
                {n}
              </p>
            ))}
          </>
        ) : (
          <p className="mt-2 text-sm text-ink-dim">Checking…</p>
        )}
      </div>

      {runError && <p className="mt-3 text-sm text-danger">{runError}</p>}
      <div className="mt-4 flex items-center gap-3">
        <Button
          disabled={readOnly || busy || stale || !plan.data || !!planError}
          onClick={() => (plan.data?.confirm ? setConfirming(true) : void run())}
        >
          {busy ? "Starting…" : "Run"}
        </Button>
        {readOnly ? (
          <span className="text-xs text-ink-faint">This connection's API token can only read, so recipes can't run.</span>
        ) : plan.data && !planError && (
          <span className="text-xs text-ink-faint">
            {plan.data.steps.length} step{plan.data.steps.length === 1 ? "" : "s"}, as your Proxmox user
          </span>
        )}
      </div>

      {confirming && plan.data?.confirm && (
        <ConfirmDialog
          title={`Run ${recipe.title}?`}
          body={plan.data.confirm}
          confirmLabel="Run"
          danger
          onClose={() => setConfirming(false)}
          onConfirm={() => {
            setConfirming(false);
            void run();
          }}
        />
      )}
    </Card>
  );
}

function toParams(fields: ScriptField[], values: Form): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  for (const f of fields) {
    const v = values[f.name];
    if (v === undefined || v === "") continue;
    if (f.type === "number" || VMID_FIELDS.has(f.type)) {
      const n = Number(v);
      if (Number.isFinite(n) && n !== 0) out[f.name] = n;
    } else if (f.type === "bool") {
      out[f.name] = Boolean(v);
    } else {
      out[f.name] = String(v).trim();
    }
  }
  return out;
}

function capitalize(s: string) {
  return s ? s[0].toUpperCase() + s.slice(1) : s;
}

type FieldProps = {
  field: ScriptField;
  values: Form;
  set: (name: string, v: FormValue) => void;
  resources: Resource[];
  nodes: string[];
  storages: { storage: string; content?: string; nodes?: string; disable?: number; shared?: number }[];
  sources: ScriptRecipe[];
  recipe: ScriptRecipe;
};

function guestLabel(g: Resource) {
  return `${g.vmid}${g.name ? ` · ${g.name}` : ""} (${g.type === "lxc" ? "container" : "VM"} on ${g.node})`;
}

function RecipeField({ field: f, values, set, resources, nodes, storages, sources, recipe }: FieldProps) {
  const v = values[f.name];
  const node = String(values.node ?? "") || undefined;
  const help = f.help && <p className="mt-1 text-xs leading-5 text-ink-faint">{f.help}</p>;

  if (f.type === "bool") {
    return (
      <div>
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" className="h-4 w-4 accent-accent" checked={Boolean(v)} onChange={(e) => set(f.name, e.target.checked)} />
          {f.label}
        </label>
        {help}
      </div>
    );
  }

  let control;
  switch (f.type) {
    case "node":
      control = (
        <Select value={String(v ?? "")} onChange={(e) => set(f.name, e.target.value)}>
          {f.optional && <option value="">All nodes</option>}
          {nodes.map((n) => (
            <option key={n} value={n}>
              {n}
            </option>
          ))}
        </Select>
      );
      break;
    case "target-node":
      control = (
        <Select value={String(v ?? "")} onChange={(e) => set(f.name, e.target.value)}>
          {nodes
            .filter((n) => n !== node)
            .map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
        </Select>
      );
      break;
    case "storage":
    case "backup-storage": {
      const list = storagesForContent(storages, f.type === "storage" ? "images" : "backup", node);
      control = (
        <Select value={String(v ?? "")} onChange={(e) => set(f.name, e.target.value)}>
          {f.optional && <option value="">{f.type === "storage" ? "Same as the template" : "Choose…"}</option>}
          {list.map((s) => (
            <option key={s.storage} value={s.storage}>
              {s.storage}
              {s.shared ? " (shared)" : ""}
            </option>
          ))}
        </Select>
      );
      break;
    }
    case "guest":
    case "qemu-guest":
    case "template": {
      const list = resources
        .filter((r) => r.type === "qemu" || r.type === "lxc")
        .filter((r) => (f.type === "template" ? r.template === 1 : r.template !== 1))
        .filter((r) => f.type !== "qemu-guest" || r.type === "qemu")
        .filter((r) => !(f.type === "template" && recipe.kind === "refresh-template" && r.type !== "qemu"))
        .sort((a, b) => (a.vmid ?? 0) - (b.vmid ?? 0));
      control = (
        <Select value={String(v ?? "")} onChange={(e) => set(f.name, Number(e.target.value) || "")}>
          <option value="">{f.type === "template" ? "Choose a template…" : "Choose a guest…"}</option>
          {list.map((g) => (
            <option key={g.id} value={g.vmid}>
              {guestLabel(g)}
            </option>
          ))}
        </Select>
      );
      if (!list.length) {
        return (
          <Field label={f.label}>
            <p className="text-sm text-ink-dim">
              {f.type === "template" ? "No templates yet. Build one from the Templates tab first." : "No guests to choose from."}
            </p>
          </Field>
        );
      }
      break;
    }
    case "source":
      control = (
        <Select value={String(v ?? "")} onChange={(e) => set(f.name, e.target.value)}>
          <option value="">The image it was built from</option>
          {sources.map((s) => (
            <option key={s.id} value={s.id}>
              {s.title}
            </option>
          ))}
        </Select>
      );
      break;
    case "mode":
      control = (
        <Select value={String(v ?? "snapshot")} onChange={(e) => set(f.name, e.target.value)}>
          <option value="snapshot">Snapshot (guests keep running)</option>
          <option value="suspend">Suspend (brief pause)</option>
          <option value="stop">Stop (shut down during the backup)</option>
        </Select>
      );
      break;
    case "keys":
      control = (
        <Textarea
          rows={3}
          className="font-mono text-xs"
          placeholder="ssh-ed25519 AAAA… you@laptop"
          value={String(v ?? "")}
          onChange={(e) => set(f.name, e.target.value)}
        />
      );
      break;
    case "number":
      control = (
        <Input
          type="number"
          inputMode="numeric"
          placeholder={f.optional ? "Automatic" : undefined}
          value={v === undefined ? "" : String(v)}
          onChange={(e) => set(f.name, e.target.value === "" ? "" : Number(e.target.value))}
        />
      );
      break;
    default:
      control = <Input value={String(v ?? "")} onChange={(e) => set(f.name, e.target.value)} />;
  }
  return (
    <Field label={f.optional && f.type !== "node" ? `${f.label} (optional)` : f.label}>
      {control}
      {help}
    </Field>
  );
}

const STEP_ICON: Record<ScriptJobStep["status"], ReactNode> = {
  pending: <Circle className="h-3.5 w-3.5 text-ink-faint" aria-label="Waiting" />,
  running: <Loader2 className="h-3.5 w-3.5 animate-spin text-accent" aria-label="Running" />,
  done: <Check className="h-3.5 w-3.5 text-ok" aria-label="Done" />,
  failed: <X className="h-3.5 w-3.5 text-danger" aria-label="Failed" />,
  skipped: <Minus className="h-3.5 w-3.5 text-ink-faint" aria-label="Skipped" />,
};

const JOB_STATUS: Record<ScriptJob["status"], { text: string; cls: string }> = {
  running: { text: "Running", cls: "text-ink-dim" },
  done: { text: "Finished", cls: "text-ok" },
  partial: { text: "Finished with some failures", cls: "text-warn" },
  failed: { text: "Failed", cls: "text-danger" },
};

function JobView({ id, onClose }: { id: string; onClose: () => void }) {
  const qc = useQueryClient();
  const q = useQuery({
    queryKey: ["script-job", id],
    queryFn: () => api.scriptJob(id),
    refetchInterval: (query) => (query.state.data?.status === "running" || !query.state.data ? 1500 : false),
  });
  const job = q.data;
  const finished = job && job.status !== "running";
  const refreshed = useRef(false);
  useEffect(() => {
    if (!finished || refreshed.current) return;
    refreshed.current = true;
    for (const key of [...fleetKeys(), ["summary"], ["nextid"], ["script-jobs"]]) void qc.invalidateQueries({ queryKey: key });
  }, [finished, qc]);

  if (q.error) {
    return (
      <Card className="p-5">
        <p className="text-sm text-danger">This run is no longer available.</p>
        <Button variant="outline" size="sm" className="mt-3" onClick={onClose}>
          Back
        </Button>
      </Card>
    );
  }
  if (!job) return <Card className="p-5 text-sm text-ink-dim">Starting…</Card>;
  const status = JOB_STATUS[job.status];
  return (
    <Card className="p-5">
      <div className="flex items-center gap-3">
        <h2 className="text-base font-semibold">{job.title}</h2>
        <span className={cn("ml-auto text-xs", status.cls)}>{status.text}</span>
      </div>
      <ol className="mt-4 space-y-3">
        {job.steps.map((s) => (
          <li key={s.id} className="flex gap-2.5">
            <span className="mt-[3px]">{STEP_ICON[s.status]}</span>
            <div className="min-w-0 flex-1 text-sm">
              <p className={cn("leading-5", s.status === "skipped" || s.status === "pending" ? "text-ink-dim" : "text-ink")}>{s.label}</p>
              {s.note && <p className="mt-0.5 text-xs text-ink-dim">{s.note}</p>}
              {s.error && <p className="mt-0.5 break-words text-xs text-danger">{s.error}</p>}
              {!!s.upids?.length && (
                <button type="button" className="mt-0.5 text-xs text-ink-faint underline-offset-2 hover:text-ink hover:underline" onClick={() => openTasks(s.upids![s.upids!.length - 1])}>
                  View task log
                </button>
              )}
            </div>
          </li>
        ))}
      </ol>
      {job.message && <p className="mt-4 rounded-md border border-line px-3 py-2 text-sm text-ink-dim">{job.message}</p>}
      <div className="mt-5 flex gap-2">
        <Button variant="outline" size="sm" onClick={onClose}>
          {finished ? "Done" : "Hide"}
        </Button>
        {!finished && <span className="self-center text-xs text-ink-faint">Keeps running if you leave this page.</span>}
      </div>
    </Card>
  );
}

function RecentRuns({ onOpen }: { onOpen: (id: string) => void }) {
  const q = useQuery({
    queryKey: ["script-jobs"],
    queryFn: api.scriptJobs,
    refetchInterval: (query) => (query.state.data?.jobs.some((j) => j.status === "running") ? 3000 : false),
  });
  const jobs = q.data?.jobs ?? [];
  return (
    <Card className="p-5">
      <h2 className="text-sm font-semibold">Recent runs</h2>
      {jobs.length === 0 ? (
        <p className="mt-2 text-sm leading-6 text-ink-dim">Pick a recipe to see what it does. Runs you start show up here.</p>
      ) : (
        <ul className="mt-3 divide-y divide-line/60">
          {jobs.slice(0, 10).map((j) => (
            <li key={j.id}>
              <button type="button" onClick={() => onOpen(j.id)} className="flex w-full items-center gap-3 py-2 text-left text-sm hover:text-ink">
                <span className="min-w-0 flex-1 truncate">{j.title}</span>
                <span className={cn("text-xs", JOB_STATUS[j.status].cls)}>{JOB_STATUS[j.status].text}</span>
                <span className="w-16 text-right text-xs text-ink-faint">{new Date(j.started * 1000).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
              </button>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
