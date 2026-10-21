import { Button } from "@/components/ui/button";
import { Modal } from "@/components/ui/dialog";
import { Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { configDigest } from "@/lib/guestHardware";
import { useEffect, useState } from "react";

const skip = new Set(["digest"]);

export function RawConfigEditor({
  vmid, node, type, config, onSaved, onClose,
}: {
  vmid: number;
  node: string;
  type: string;
  config: Record<string, unknown>;
  onSaved: () => void;
  onClose: () => void;
}) {
  const [text, setText] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    setText(serializeConfig(config));
  }, [config]);

  return (
    <Modal
      title="Raw config"
      onClose={onClose}
      size="lg"
      footer={
        <Button
          disabled={busy}
          onClick={async () => {
            setError("");
            setBusy(true);
            try {
              const { fields, delete: del } = parseConfig(text, config);
              await api.updateGuest(vmid, {
                node,
                type,
                digest: configDigest(config),
                fields,
                delete: del,
              }, node, type);
              onSaved();
            } catch (e) {
              setError(e instanceof Error ? e.message : "Save failed");
            } finally {
              setBusy(false);
            }
          }}
        >
          Save raw config
        </Button>
      }
    >
      <p className="text-xs text-ink-dim">
        Full qemu/lxc config. Saving uses the current digest; a conflict means someone else changed the guest.
      </p>
      <Textarea className="min-h-56 font-mono text-xs" value={text} onChange={(e) => setText(e.target.value)} />
      {error && <p className="text-sm text-rose-400">{error}</p>}
    </Modal>
  );
}

function serializeConfig(config: Record<string, unknown>) {
  return Object.entries(config)
    .filter(([key]) => !skip.has(key))
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => `${key}: ${value == null ? "" : typeof value === "object" ? JSON.stringify(value) : String(value)}`)
    .join("\n");
}

function parseConfig(text: string, current: Record<string, unknown>) {
  const fields: Record<string, unknown> = {};
  const seen = new Set<string>();
  for (const line of text.split("\n")) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const idx = trimmed.indexOf(":");
    if (idx <= 0) continue;
    const key = trimmed.slice(0, idx).trim();
    if (!key || skip.has(key)) continue;
    fields[key] = trimmed.slice(idx + 1).trim();
    seen.add(key);
  }
  const del = Object.keys(current).filter((key) => !skip.has(key) && !seen.has(key));
  return { fields, delete: del };
}
