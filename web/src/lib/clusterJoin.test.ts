import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { buildJoinInfo, decodeJoinInfo, encodeJoinInfo } from "./clusterJoin.ts";

// GET /cluster/config/join, as a two-node cluster answers it.
const joinApi = {
  preferred_node: "zs-hv1",
  config_digest: "abc",
  nodelist: [
    { name: "zs-hv1", nodeid: "1", pve_addr: "10.0.0.11", pve_fp: "AA:BB:CC", quorum_votes: "1", ring0_addr: "10.0.0.11", ring1_addr: "10.1.0.11" },
    { name: "zs-hv2", nodeid: "2", pve_addr: "10.0.0.12", pve_fp: "DD:EE:FF", quorum_votes: "1", ring0_addr: "10.0.0.12" },
  ],
  totem: {
    cluster_name: "Zypher-Systems",
    config_version: "2",
    interface: { "0": { linknumber: "0" }, "1": { linknumber: "1" } },
    ip_version: "ipv4-6",
    secauth: "on",
    version: "2",
  },
};

describe("buildJoinInfo", () => {
  it("builds Proxmox's join information for the node answering", () => {
    const info = buildJoinInfo(joinApi);
    assert.deepEqual(info, {
      ipAddress: "10.0.0.11",
      fingerprint: "AA:BB:CC",
      peerLinks: { 0: "10.0.0.11", 1: "10.1.0.11" },
      ring_addr: ["10.0.0.11", "10.1.0.11"],
      totem: joinApi.totem,
    });
  });
  it("is null outside a cluster", () => {
    assert.equal(buildJoinInfo({ nodelist: [], totem: {} }), null);
    assert.equal(buildJoinInfo(undefined), null);
  });
});

describe("encode and decode", () => {
  it("encodes the same way Proxmox does (base64 of the JSON)", () => {
    const info = buildJoinInfo(joinApi)!;
    const blob = encodeJoinInfo(info);
    assert.deepEqual(JSON.parse(Buffer.from(blob, "base64").toString("utf8")), JSON.parse(JSON.stringify(info)));
  });
  it("reads a blob back into what the join form needs", () => {
    const blob = encodeJoinInfo(buildJoinInfo(joinApi)!);
    assert.deepEqual(decodeJoinInfo(`\n${blob.slice(0, 20)}\n${blob.slice(20)}  `), {
      ip: "10.0.0.11",
      fingerprint: "AA:BB:CC",
      clusterName: "Zypher-Systems",
      links: [
        { number: 0, peer: "10.0.0.11" },
        { number: 1, peer: "10.1.0.11" },
      ],
    });
  });
  it("rejects anything that isn't join information", () => {
    assert.equal(decodeJoinInfo("not base64!"), null);
    assert.equal(decodeJoinInfo(Buffer.from('{"hello":1}').toString("base64")), null);
  });
});
