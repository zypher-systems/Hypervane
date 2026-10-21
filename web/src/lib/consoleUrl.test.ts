import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { consoleUrl } from "./consoleUrl.ts";

describe("consoleUrl", () => {
  it("puts connection on guest console so the websocket can select the cluster", () => {
    const url = consoleUrl({ vmid: 127, node: "zs-hv1", type: "qemu", kind: "vnc", connection: "lab-a" });
    assert.equal(url, "/api/guests/127/console?node=zs-hv1&kind=vnc&type=qemu&connection=lab-a");
  });

  it("puts connection on node console", () => {
    const url = consoleUrl({ node: "zs-hv2", kind: "term", connection: "lab-b" });
    assert.equal(url, "/api/nodes/zs-hv2/console?kind=term&connection=lab-b");
  });

  it("asks a node terminal for Proxmox's upgrade", () => {
    const url = consoleUrl({ node: "zs-hv1", kind: "term", cmd: "upgrade", connection: "lab-b" });
    assert.equal(url, "/api/nodes/zs-hv1/console?kind=term&cmd=upgrade&connection=lab-b");
  });

  it("carries a one-time root grant so the upgrade starts", () => {
    const url = consoleUrl({ node: "zs-hv1", kind: "term", cmd: "upgrade", grant: "abc", connection: "lab-b" });
    assert.equal(url, "/api/nodes/zs-hv1/console?kind=term&cmd=upgrade&grant=abc&connection=lab-b");
  });

  it("omits connection when none is active", () => {
    const url = consoleUrl({ vmid: 127, node: "zs-hv1", type: "qemu", kind: "vnc", connection: "" });
    assert.equal(url, "/api/guests/127/console?node=zs-hv1&kind=vnc&type=qemu");
  });
});
