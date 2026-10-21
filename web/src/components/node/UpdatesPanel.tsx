import { ConsoleView } from "@/components/console/ConsoleView";
import { attachTask } from "@/components/layout/AppShell";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { BarButton } from "@/components/ui/action-bar";
import { Modal } from "@/components/ui/dialog";
import { Field, Input } from "@/components/ui/input";
import { api } from "@/lib/api";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouteContext } from "@tanstack/react-router";
import { useState } from "react";

export function UpdatesPanel({ node, writable }: { node: string; writable: boolean }) {
  const qc = useQueryClient();
  const updates = useQuery({ queryKey: ["apt-updates", node], queryFn: () => api.aptUpdates(node) });
  const versions = useQuery({ queryKey: ["apt-versions", node], queryFn: () => api.aptVersions(node) });
  const repos = useQuery({ queryKey: ["apt-repos", node], queryFn: () => api.aptRepositories(node) });
  const [log, setLog] = useState("");
  const [installing, setInstalling] = useState(false);
  const count = updates.data?.length ?? 0;

  return (
    <div className="space-y-4">
      {writable && (
        <div className="flex flex-wrap gap-2">
          {count > 0 && (
            <BarButton accent onClick={() => setInstalling(true)}>
              Install {count} update{count === 1 ? "" : "s"}
            </BarButton>
          )}
          <BarButton accent={count === 0} onClick={async () => {
            attachTask((await api.aptUpdate(node)).upid, [["apt-updates", node], ["apt-versions", node]]);
          }}>Refresh updates</BarButton>
        </div>
      )}
      {installing && (
        <UpgradeWindow
          node={node}
          onClose={() => {
            setInstalling(false);
            for (const key of [["apt-updates", node], ["apt-versions", node]]) void qc.invalidateQueries({ queryKey: key });
          }}
        />
      )}
      <Card className="overflow-hidden">
        <h2 className="border-b border-white/15 px-4 py-3 text-xs font-medium text-ink-dim">Available updates</h2>
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-4 py-2">Package</th><th>Version</th><th></th></tr>
          </thead>
          <tbody>
            {(updates.data ?? []).map((u, i) => {
              const name = String(u.Package || u.package || i);
              return (
                <tr key={name} className="border-t border-line">
                  <td className="px-4 py-2">{name}</td>
                  <td className="font-mono text-xs">{String(u.Version || u.version || "")}</td>
                  <td className="px-4 text-right">
                    <BarButton onClick={async () => {
                      setLog((await api.aptChangelog(node, name)).changelog);
                    }}>Changelog</BarButton>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {!updates.data?.length && <p className="p-4 text-sm text-ink-dim">No updates listed.</p>}
      </Card>
      <Card className="overflow-hidden">
        <h2 className="border-b border-white/15 px-4 py-3 text-xs font-medium text-ink-dim">Versions</h2>
        <table className="w-full text-sm">
          <thead className="bg-white/[0.02] text-left text-xs text-ink-dim">
            <tr><th className="px-4 py-2">Package</th><th>Installed</th><th>Candidate</th><th>Repo</th></tr>
          </thead>
          <tbody>
            {(versions.data ?? []).map((v, i) => (
              <tr key={String(v.Package || v.package || i)} className="border-t border-line">
                <td className="px-4 py-2">{String(v.Package || v.package || "—")}</td>
                <td className="font-mono text-xs">{String(v.OldVersion || v.Installed || v.installed || v.CurrentVersion || "—")}</td>
                <td className="font-mono text-xs">{String(v.Version || v.Candidate || v.candidate || "—")}</td>
                <td className="text-ink-dim">{String(v.Origin || v.Repo || v.repo || v.Section || "—")}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {!versions.data?.length && <p className="p-4 text-sm text-ink-dim">No package versions listed.</p>}
      </Card>
      <Card className="space-y-2 p-4">
        <h2 className="border-b border-white/15 pb-2 text-xs font-medium text-ink-dim">Repositories</h2>
        {(repos.data?.files ?? []).flatMap((f) => (f.repositories ?? []).map((repo, idx) => {
          const enabled = repo.enabled === 1 || repo.enabled === true || repo.enabled === "1";
          return (
            <div key={`${f.path}-${idx}`} className="flex items-center justify-between text-sm">
              <span className="font-mono text-xs">{String(repo.suite || repo.file || f.path)} · {enabled ? "on" : "off"}</span>
              {writable && (
                <BarButton onClick={async () => {
                  await api.updateAPTRepository(node, { path: f.path, index: idx, enabled: enabled ? 0 : 1, digest: repos.data?.digest });
                  void qc.invalidateQueries({ queryKey: ["apt-repos", node] });
                }}>{enabled ? "Disable" : "Enable"}</BarButton>
              )}
            </div>
          );
        }))}
      </Card>
      {log && (
        <Modal title="Changelog" onClose={() => setLog("")}>
          <pre className="max-h-80 overflow-auto font-mono text-xs text-ink-dim">{log}</pre>
        </Modal>
      )}
    </div>
  );
}

/**
 * Installs updates the way Proxmox's own Upgrade button does: pveupgrade in a
 * node terminal, so apt's questions can be answered. Proxmox starts that
 * command only for a root@pam login. An API token gets a login shell, so this
 * exchanges the node's root password for a one-time root ticket and opens the
 * upgrade with it. The password is not saved.
 */
function UpgradeWindow({ node, onClose }: { node: string; onClose: () => void }) {
  const ctx = useRouteContext({ from: "/app" });
  const direct = ctx.kind === "pve" && ctx.username === "root@pam";
  const [phase, setPhase] = useState<"password" | "shell" | "upgrade">(direct ? "upgrade" : "password");
  const [grant, setGrant] = useState("");
  const [password, setPassword] = useState("");
  const [otp, setOtp] = useState("");
  const [challenge, setChallenge] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [typeText, setTypeText] = useState<((text: string) => void) | null>(null);
  const [open, setOpen] = useState(false);

  const close = () => {
    if (open && !window.confirm("If the upgrade is still running, closing this window stops it part-way. Close anyway?")) return;
    onClose();
  };

  const start = async () => {
    setBusy(true);
    setError("");
    try {
      const res = await api.aptUpgradeSession(node, {
        password,
        otp: otp || undefined,
        tfaChallenge: challenge || undefined,
      });
      if (res.needTfa) {
        setChallenge(res.tfaChallenge ?? "");
        setOtp("");
        return;
      }
      if (!res.grant) {
        setError("Proxmox did not start an upgrade session.");
        return;
      }
      setPassword("");
      setOtp("");
      setGrant(res.grant);
      setPhase("upgrade");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not start the upgrade");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal title={`Install updates on ${node}`} size={phase === "password" ? "md" : "xl"} onClose={close}>
      {phase === "password" ? (
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            void start();
          }}
        >
          <p className="text-sm leading-6 text-ink-dim">
            Proxmox starts the upgrade on its own only for <span className="font-mono text-xs">root@pam</span>. This session
            {ctx.kind === "pve" ? ` is ${ctx.username}` : " uses the connection's API token"}, so enter the node's root password.
            Hypervane uses it once to open the upgrade, then drops it. You still answer apt's questions in the terminal, the same as
            Proxmox's Upgrade button.
          </p>
          <Field label="Node root password">
            <Input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete="new-password"
              autoFocus={challenge === ""}
            />
          </Field>
          {challenge && (
            <Field label="One-time code">
              <Input
                value={otp}
                onChange={(e) => setOtp(e.target.value)}
                autoComplete="one-time-code"
                placeholder="TOTP or recovery key"
                autoFocus
              />
            </Field>
          )}
          {error && <p className="text-sm text-rose-400">{error}</p>}
          <div className="flex flex-wrap gap-2">
            <Button disabled={busy || password === "" || (challenge !== "" && otp === "")}>
              {busy ? "Starting…" : "Start upgrade"}
            </Button>
            <Button type="button" variant="outline" onClick={() => setPhase("shell")}>
              Log in on the node instead
            </Button>
          </div>
        </form>
      ) : (
        <>
          {phase === "upgrade" ? (
            <p className="text-sm leading-6 text-ink-dim">
              This runs Proxmox's upgrade (<span className="font-mono text-xs">pveupgrade</span>), the same as the Upgrade button in
              the Proxmox interface. Answer any questions it asks below.
            </p>
          ) : (
            <div className="space-y-2 text-sm leading-6 text-ink-dim">
              <p>
                The node asks you to log in first. Log in below as <span className="font-mono text-xs text-ink">root</span> with
                the node's root password, then run the upgrade.
              </p>
              <Button size="sm" variant="outline" disabled={!typeText} onClick={() => typeText?.("pveupgrade\r")}>
                Type pveupgrade
              </Button>
            </div>
          )}
          <p className="text-xs text-warn">Keep this window open until the upgrade finishes. Closing it stops the upgrade part-way.</p>
          <ConsoleView
            node={node}
            kind="term"
            cmd="upgrade"
            grant={grant || undefined}
            onReady={(fn) => {
              setTypeText(() => fn);
              setOpen(true);
            }}
            onClosed={() => {
              setTypeText(null);
              setOpen(false);
            }}
          />
        </>
      )}
    </Modal>
  );
}
