import { attachTask } from "@/components/layout/AppShell";
import { Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { ConfirmDialog } from "@/components/ui/dialog";
import { Field, Textarea } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

export function CertsPanel({ node, writable }: { node: string; writable: boolean }) {
  const qc = useQueryClient();
  const certs = useQuery({ queryKey: ["node-certs", node], queryFn: () => api.nodeCertificates(node) });
  const [pem, setPem] = useState("");
  const [key, setKey] = useState("");
  const [del, setDel] = useState(false);
  const [forceACME, setForceACME] = useState(false);

  return (
    <div className="space-y-4">
      <Card className="overflow-hidden">
        {certs.error ? (
          <p className="p-4 text-sm text-danger">Couldn't read certificates: {certs.error.message}</p>
        ) : (
          <table className="w-full text-sm">
            <thead className="border-b border-line text-left text-xs text-ink-dim">
              <tr><th className="px-4 py-2 font-medium">File</th><th className="font-medium">Subject</th><th className="font-medium">Expires</th></tr>
            </thead>
            <tbody>
              {(certs.data ?? []).map((c) => (
                <tr key={c.filename || c.fingerprint} className="border-t border-line/60">
                  <td className="px-4 py-2 font-mono text-xs">{c.filename || "—"}</td>
                  <td className="text-ink-dim">{c.subject || "—"}</td>
                  <td className="pr-4"><CertExpiry notafter={c.notafter} /></td>
                </tr>
              ))}
              {certs.isSuccess && !certs.data?.length && (
                <tr><td colSpan={3} className="px-4 py-3 text-ink-dim">No certificates reported.</td></tr>
              )}
            </tbody>
          </table>
        )}
      </Card>
      {writable && (
        <Card className="space-y-3 p-4">
          <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Custom certificate</h2>
          <Field label="Certificate PEM"><Textarea className="min-h-24 font-mono text-xs" value={pem} onChange={(e) => setPem(e.target.value)} /></Field>
          <Field label="Private key"><Textarea className="min-h-24 font-mono text-xs" value={key} onChange={(e) => setKey(e.target.value)} /></Field>
          <div className="flex flex-wrap gap-2">
            <BarButton accent onClick={async () => {
              await api.uploadNodeCertificate(node, { certificates: pem, key, restart: 1 });
              setPem("");
              setKey("");
              void qc.invalidateQueries({ queryKey: ["node-certs", node] });
            }}>Upload</BarButton>
            <BarButton onClick={() => setDel(true)}>Delete custom</BarButton>
            <label className="flex items-center gap-2 text-sm text-ink-dim">
              <input type="checkbox" checked={forceACME} onChange={(e) => setForceACME(e.target.checked)} />
              Force order
            </label>
            <BarButton onClick={async () => attachTask((await api.orderACMECertificate(node, forceACME)).upid, [["node-certs", node]])}>Order ACME</BarButton>
            <BarButton onClick={async () => attachTask((await api.renewACMECertificate(node)).upid, [["node-certs", node]])}>Renew ACME</BarButton>
          </div>
        </Card>
      )}
      {del && (
        <ConfirmDialog
          title="Delete custom certificate?"
          body="Reverts to the default node certificate."
          danger
          confirmLabel="Delete"
          onClose={() => setDel(false)}
          onConfirm={async () => {
            await api.deleteNodeCertificate(node);
            setDel(false);
            void qc.invalidateQueries({ queryKey: ["node-certs", node] });
          }}
        />
      )}
    </div>
  );
}

/** Expiry date plus days left; amber inside 30 days, red inside 7 or expired. */
function CertExpiry({ notafter }: { notafter?: number }) {
  if (!notafter) return <span className="text-ink-dim">—</span>;
  const days = Math.floor((notafter * 1000 - Date.now()) / 86_400_000);
  const date = new Date(notafter * 1000).toLocaleDateString(undefined, { year: "numeric", month: "short", day: "numeric" });
  const tone = days < 7 ? "text-danger" : days < 30 ? "text-warn" : "text-ink-dim";
  const left = days < 0 ? `expired ${-days} d ago` : days === 0 ? "expires today" : `${days} d left`;
  return (
    <span className="whitespace-nowrap">
      {date} <span className={`text-xs ${tone}`}>· {left}</span>
    </span>
  );
}
