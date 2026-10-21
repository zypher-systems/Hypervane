import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Textarea } from "@/components/ui/input";
import { copyText } from "@/lib/clipboard";
import { buildJoinInfo, decodeJoinInfo, encodeJoinInfo, type DecodedJoin } from "@/lib/clusterJoin";
import { useState } from "react";

function CopyButton({ text, label = "Copy" }: { text: string; label?: string }) {
  const [state, setState] = useState<"" | "copied" | "failed">("");
  return (
    <Button
      size="sm"
      variant="outline"
      disabled={!text}
      onClick={async () => {
        setState((await copyText(text)) ? "copied" : "failed");
        window.setTimeout(() => setState(""), 2000);
      }}
    >
      {state === "copied" ? "Copied" : state === "failed" ? "Select and copy it" : label}
    </Button>
  );
}

/**
 * Join information for adding a node to this cluster, as Proxmox's own "Join
 * Information" window shows it: the address and fingerprint of the node
 * Hypervane is connected to, and the base64 blob Proxmox's "Join Cluster"
 * dialog (or Hypervane's) accepts.
 */
export function JoinInfoCard({ data }: { data: unknown }) {
  const info = buildJoinInfo(data);
  if (!info) return null;
  const blob = encodeJoinInfo(info);
  const rows: [string, string][] = [
    ["Cluster", String(info.totem.cluster_name ?? "")],
    ["IP address", info.ipAddress],
    ["Fingerprint", info.fingerprint],
  ];
  return (
    <Card className="space-y-3 p-4">
      <div>
        <h2 className="font-medium">Join information</h2>
        <p className="mt-1 text-sm text-ink-dim">
          To add a node, copy this and paste it into the new node's Join cluster form, in Hypervane or in Proxmox (Datacenter,
          Cluster, Join Cluster). You'll also need this node's root password.
        </p>
      </div>
      <dl className="grid gap-x-4 gap-y-2 text-sm sm:grid-cols-[8rem_minmax(0,1fr)_auto] sm:items-center">
        {rows.map(([label, value]) => (
          <div key={label} className="contents">
            <dt className="text-ink-dim">{label}</dt>
            <dd className="min-w-0 break-all font-mono text-xs">{value || "—"}</dd>
            <dd>{label !== "Cluster" && <CopyButton text={value} />}</dd>
          </div>
        ))}
      </dl>
      <Field label="Join information">
        <Textarea readOnly rows={4} className="font-mono text-xs" value={blob} onFocus={(e) => e.currentTarget.select()} />
      </Field>
      <CopyButton text={blob} label="Copy join information" />
    </Card>
  );
}

/**
 * Paste box for joining this node to a cluster: reads a join information blob
 * and hands back the address, fingerprint and the cluster's links.
 */
export function JoinPaste({ onFill }: { onFill: (join: DecodedJoin) => void }) {
  const [value, setValue] = useState("");
  const [decoded, setDecoded] = useState<DecodedJoin | null>(null);
  const bad = value.trim() !== "" && !decoded;
  return (
    <div className="space-y-2">
      <Field label="Join information (optional)">
        <Textarea
          rows={3}
          className="font-mono text-xs"
          placeholder="Paste the join information copied from a node of the cluster"
          value={value}
          onChange={(e) => {
            setValue(e.target.value);
            const d = e.target.value.trim() ? decodeJoinInfo(e.target.value) : null;
            setDecoded(d);
            if (d) onFill(d);
          }}
        />
      </Field>
      {bad && <p className="text-sm text-warn">That isn't join information. Copy it again from a node that's already in the cluster.</p>}
      {decoded && (
        <p className="text-sm text-ink-dim">
          Joining <span className="text-ink">{decoded.clusterName}</span> through {decoded.ip}. Enter that node's root password
          {decoded.links.length > 1 ? ", and this node's address on each link" : ""} below.
        </p>
      )}
    </div>
  );
}
