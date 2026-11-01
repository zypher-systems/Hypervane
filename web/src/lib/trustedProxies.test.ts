import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { after, describe, it } from "node:test";

const root = join(import.meta.dirname, "..", "..");
const script = join(root, "docker", "trusted-proxies.sh");
const dir = mkdtempSync(join(tmpdir(), "hypervane-proxies-"));
const out = join(dir, "trusted-proxies.conf");

function run(value?: string) {
  const env: Record<string, string> = { PATH: process.env.PATH ?? "", TRUSTED_PROXIES_CONF: out };
  if (value !== undefined) env.HYPERVANE_TRUSTED_PROXIES = value;
  return spawnSync("sh", [script], { env, encoding: "utf8" });
}

describe("trusted proxies", () => {
  after(() => rmSync(dir, { recursive: true, force: true }));

  it("trusts nobody by default", () => {
    assert.equal(run().status, 0);
    assert.equal(readFileSync(out, "utf8"), "");
    assert.equal(run("").status, 0);
    assert.equal(readFileSync(out, "utf8"), "");
  });

  it("lists exactly what it was given", () => {
    assert.equal(run("192.0.2.10, 198.51.100.0/24 2001:db8::1").status, 0);
    assert.equal(
      readFileSync(out, "utf8"),
      "set_real_ip_from 192.0.2.10;\nset_real_ip_from 198.51.100.0/24;\nset_real_ip_from 2001:db8::1;\n",
    );
  });

  it("refuses anything that could change the nginx config", () => {
    for (const bad of ["10.0.0.1;include/etc/passwd", "10.0.0.1;", "proxy.example", "$host"]) {
      const res = run(bad);
      assert.notEqual(res.status, 0, bad);
      assert.match(res.stderr, /not an address/);
    }
  });

  it("refuses a range that trusts every client", () => {
    for (const bad of ["0.0.0.0/0", "::/0", "192.0.2.1, 0.0.0.0/0"]) {
      assert.notEqual(run(bad).status, 0, bad);
    }
  });

  it("keeps the trust list out of the fixed nginx config", () => {
    const conf = readFileSync(join(root, "nginx.conf"), "utf8");
    assert.equal(/^\s*set_real_ip_from/m.test(conf), false);
    assert.match(conf, /include \/etc\/nginx\/trusted-proxies\.conf;/);
  });
});
