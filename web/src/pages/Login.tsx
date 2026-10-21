import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Field, Input, Select } from "@/components/ui/input";
import { api } from "@/lib/api";
import { setActiveConnection } from "@/lib/connection";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { useState } from "react";

export function LoginPage() {
  const nav = useNavigate();
  const me = useQuery({ queryKey: ["me"], queryFn: api.me });
  const connections = (me.data?.connections ?? []).filter((c) => c.enabled);
  const [connection, setConnection] = useState("");
  const realms = useQuery({
    queryKey: ["auth-realms", connection],
    queryFn: () => api.authRealms(connection || undefined),
  });
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [realm, setRealm] = useState("local");
  const [otp, setOtp] = useState("");
  const [challenge, setChallenge] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  return (
    <div className="mx-auto flex min-h-full max-w-md flex-col items-center justify-center px-4">
      <img src="/hypervane.svg" alt="Hypervane" className="mb-6 h-16 w-16" />
      <Card className="w-full p-8">
        <p className="font-mono text-xs uppercase tracking-[0.25em] text-accent">Hypervane</p>
        <h1 className="mt-2 text-3xl font-semibold">Sign in</h1>
        <form
          className="mt-6 grid gap-4"
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            setError("");
            try {
              const res = await api.login(
                username,
                password,
                realm,
                otp || undefined,
                challenge || undefined,
                realm !== "local" ? connection || undefined : undefined,
              );
              if (res.needTfa) {
                setChallenge(res.tfaChallenge ?? "");
                return;
              }
              if (connection) setActiveConnection(connection);
              nav({ to: "/" });
            } catch (err) {
              setError(err instanceof Error ? err.message : "Login failed");
            } finally {
              setBusy(false);
            }
          }}
        >
          {connections.length > 1 && realm !== "local" && (
            <Field label="Connection">
              <Select
                value={connection}
                onChange={(e) => {
                  setConnection(e.target.value);
                  setChallenge("");
                  setOtp("");
                }}
              >
                <option value="">Default</option>
                {connections.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </Select>
            </Field>
          )}
          <Field label="Realm">
            <Select value={realm} onChange={(e) => { setRealm(e.target.value); setChallenge(""); setOtp(""); }}>
              {(realms.data ?? [{ realm: "local", type: "local" }]).map((r) => (
                <option key={r.realm} value={r.realm}>
                  {r.realm}{r.type && r.type !== r.realm ? ` (${r.type})` : ""}
                </option>
              ))}
            </Select>
          </Field>
          <Field label="Username">
            <Input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
          </Field>
          <Field label="Password">
            <Input type="password" value={password} onChange={(e) => setPassword(e.target.value)} autoComplete="current-password" />
          </Field>
          {challenge && (
            <Field label="One-time code">
              <Input value={otp} onChange={(e) => setOtp(e.target.value)} autoComplete="one-time-code" placeholder="TOTP or recovery key" />
            </Field>
          )}
          {error && <p className="text-sm text-rose-400">{error}</p>}
          <Button disabled={busy}>{busy ? "Signing in…" : challenge ? "Verify" : "Continue"}</Button>
        </form>
      </Card>
    </div>
  );
}
