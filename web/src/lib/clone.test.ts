import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { cloneCanLeaveNode, guestStorages } from "./clone.ts";

const stores = [
  { storage: "local", type: "dir", content: "iso,vztmpl,import" },
  { storage: "local-zfs", type: "zfspool", content: "images,rootdir" },
  { storage: "ceph", type: "rbd", content: "images,rootdir", shared: 1 },
  { storage: "nfs", type: "nfs", content: "images,iso", shared: 1 },
];

describe("guestStorages", () => {
  it("counts disks, cloud-init and ISOs but not empty drives", () => {
    const cfg = {
      scsi0: "local-zfs:base-9000-disk-0,discard=on,size=3G",
      ide2: "local-zfs:vm-9000-cloudinit,media=cdrom",
      ide0: "none,media=cdrom",
      sata1: "nfs:iso/debian.iso,media=cdrom",
      net0: "virtio=BC:24:11:00:00:01,bridge=vmbr0",
      unused0: "ceph:vm-9000-disk-9",
    };
    assert.deepEqual(guestStorages("qemu", cfg).sort(), ["local-zfs", "nfs"]);
  });
  it("counts a container's rootfs and mount points, not bind mounts", () => {
    const cfg = { rootfs: "local-zfs:subvol-200-disk-0,size=8G", mp0: "ceph:vm-200-disk-1,mp=/data", mp1: "/srv/share,mp=/share" };
    assert.deepEqual(guestStorages("lxc", cfg).sort(), ["ceph", "local-zfs"]);
  });
});

describe("cloneCanLeaveNode", () => {
  it("keeps a template on local storage on its node", () => {
    const got = cloneCanLeaveNode("qemu", { scsi0: "local-zfs:base-9000-disk-0,size=3G" }, stores, true, "ceph");
    assert.equal(got.allowed, false);
    assert.deepEqual(got.local, ["local-zfs"]);
  });
  it("counts the cloud-init drive too", () => {
    const cfg = { scsi0: "ceph:base-9000-disk-0", ide2: "local-zfs:vm-9000-cloudinit,media=cdrom" };
    assert.equal(cloneCanLeaveNode("qemu", cfg, stores, true).allowed, false);
  });
  it("lets a guest on shared storage go anywhere", () => {
    const cfg = { scsi0: "ceph:base-9000-disk-0", ide2: "ceph:vm-9000-cloudinit,media=cdrom", ide0: "none,media=cdrom" };
    assert.equal(cloneCanLeaveNode("qemu", cfg, stores, false).allowed, true);
  });
  it("lets a container's full clone leave when it lands on shared storage", () => {
    const cfg = { rootfs: "local-zfs:subvol-200-disk-0,size=8G" };
    assert.equal(cloneCanLeaveNode("lxc", cfg, stores, true, "ceph").allowed, true);
    assert.equal(cloneCanLeaveNode("lxc", cfg, stores, true, "local-zfs").allowed, false);
    assert.equal(cloneCanLeaveNode("lxc", cfg, stores, false, "ceph").allowed, false);
  });
  it("treats an unknown storage as local", () => {
    assert.equal(cloneCanLeaveNode("qemu", { scsi0: "gone:vm-1-disk-0" }, stores, true).allowed, false);
  });
});
