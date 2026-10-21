import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { defaultStorage, storagesForContent } from "./storage.ts";

const stores = [
  { storage: "zs-pbs-01", type: "pbs", content: "backup" },
  { storage: "local-zfs-array", type: "zfspool", content: "images,rootdir" },
  { storage: "local", type: "dir", content: "iso,vztmpl,backup,snippets,import" },
];

describe("storagesForContent", () => {
  it("keeps PBS off disk-image pickers", () => {
    assert.deepEqual(storagesForContent(stores, "images").map((s) => s.storage), ["local-zfs-array"]);
  });
  it("prefers local for imports", () => {
    assert.equal(defaultStorage(stores, "import"), "local");
  });
});
