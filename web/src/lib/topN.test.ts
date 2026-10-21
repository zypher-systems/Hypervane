import assert from "node:assert/strict";
import { describe, it } from "node:test";
import type { Resource } from "../types.ts";
import { agentDiskUsed, guestMemScale, integrateRates, iopsTotals, overlayGuests, rankGuests, sumGuestKeys } from "./topN.ts";

function guest(partial: Partial<Resource> & Pick<Resource, "id" | "vmid">): Resource {
  return {
    type: "qemu",
    cpu: 0,
    maxcpu: 1,
    mem: 0,
    maxmem: 0,
    disk: 0,
    maxdisk: 0,
    uptime: 0,
    template: 0,
    netin: 0,
    netout: 0,
    diskread: 0,
    diskwrite: 0,
    ...partial,
  };
}

describe("rankGuests", () => {
  it("returns an empty list", () => {
    assert.deepEqual(rankGuests([], "cpu"), []);
  });

  it("excludes templates and non-guests", () => {
    const rows = rankGuests(
      [
        guest({ id: "node/pve", type: "node", vmid: 0, cpu: 0.9 }),
        guest({ id: "qemu/1", vmid: 1, cpu: 0.2, template: 1 }),
        guest({ id: "qemu/2", vmid: 2, cpu: 0.1 }),
      ],
      "cpu",
    );
    assert.equal(rows.length, 1);
    assert.equal(rows[0].guest.vmid, 2);
  });

  it("breaks ties by VMID then name", () => {
    const rows = rankGuests(
      [
        guest({ id: "qemu/20", vmid: 20, name: "b", cpu: 0.5 }),
        guest({ id: "qemu/10", vmid: 10, name: "a", cpu: 0.5 }),
        guest({ id: "qemu/11", vmid: 11, name: "c", cpu: 0.4 }),
      ],
      "cpu",
    );
    assert.deepEqual(
      rows.map((r) => r.guest.vmid),
      [10, 20, 11],
    );
  });

  it("ranks diskio by read plus write", () => {
    const rows = rankGuests(
      [
        guest({ id: "qemu/1", vmid: 1, diskread: 10, diskwrite: 1 }),
        guest({ id: "qemu/2", vmid: 2, diskread: 3, diskwrite: 20 }),
      ],
      "diskio",
    );
    assert.equal(rows[0].guest.vmid, 2);
    assert.equal(rows[0].value, 23);
  });

  it("omits guests without an IOPS sample", () => {
    const rows = rankGuests(
      [
        guest({ id: "qemu/1", vmid: 1 }),
        guest({ id: "qemu/2", vmid: 2 }),
      ],
      "iops",
      { 2: 40 },
    );
    assert.equal(rows.length, 1);
    assert.equal(rows[0].guest.vmid, 2);
    assert.equal(rows[0].value, 40);
  });
});

describe("overlayGuests", () => {
  it("aligns guests onto one series per id", () => {
    const { points, series } = overlayGuests([
      {
        id: "a",
        label: "A",
        color: "#111",
        points: [{ time: 1, cpu: 0.2 }, { time: 2, cpu: 0.4 }],
        value: (p) => p.cpu * 100,
      },
      {
        id: "b",
        label: "B",
        color: "#222",
        points: [{ time: 2, cpu: 0.1 }],
        value: (p) => p.cpu * 100,
      },
    ]);
    assert.deepEqual(series.map((s) => s.key), ["a", "b"]);
    assert.equal(points[0].a, 20);
    assert.equal(points[1].a, 40);
    assert.equal(points[1].b, 10);
  });
});

describe("sumGuestKeys", () => {
  it("adds matching keys across guests", () => {
    const points = sumGuestKeys(
      [
        [{ time: 1, netin: 2, netout: 3 }],
        [{ time: 1, netin: 4, netout: 1 }],
      ],
      ["netin", "netout"],
    );
    assert.equal(points[0].netin, 6);
    assert.equal(points[0].netout, 4);
  });
});

describe("integrateRates", () => {
  it("integrates average rates over successive intervals", () => {
    const totals = integrateRates(
      [
        { time: 0, netin: 10, netout: 2 },
        { time: 10, netin: 10, netout: 2 },
        { time: 20, netin: 30, netout: 0 },
      ],
      ["netin", "netout"],
    );
    assert.equal(totals.netin, 300);
    assert.equal(totals.netout, 30);
  });

  it("returns zeros without an interval", () => {
    assert.deepEqual(integrateRates([{ time: 1, netin: 5 }], ["netin"]), { netin: 0 });
  });
});

describe("iopsTotals", () => {
  it("sums read and write", () => {
    assert.deepEqual(iopsTotals({ "100": { rd: 3, wr: 2 } }), { 100: 5 });
  });
});

describe("agentDiskUsed", () => {
  it("sums guest filesystems and skips tmpfs", () => {
    assert.equal(
      agentDiskUsed({
        result: [
          { type: "ext4", "used-bytes": 4_000_000_000 },
          { type: "tmpfs", "used-bytes": 9_000_000_000 },
        ],
      }),
      4_000_000_000,
    );
  });
});

describe("guestMemScale", () => {
  it("keeps sub-1GiB samples as bytes when live mem matches", () => {
    const scale = guestMemScale([{ mem: 900e6 }], 910e6, 16e9);
    assert.equal(scale(900e6), 900e6);
  });

  it("scales KiB when that matches live mem", () => {
    const scale = guestMemScale([{ mem: 8e6 }], 8e6 * 1024);
    assert.equal(scale(8e6), 8e6 * 1024);
  });
});
