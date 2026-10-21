import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { attachTask, finishTask, hasPendingTask, isTaskFinished } from "./tasks.ts";

describe("task completion", () => {
  it("is finished when exitstatus or endtime is set", () => {
    assert.equal(isTaskFinished({ status: "running" }), false);
    assert.equal(isTaskFinished({}), false);
    assert.equal(isTaskFinished({ status: "stopped", exitstatus: "OK" }), true);
    assert.equal(isTaskFinished({ endtime: 1 }), true);
    assert.equal(isTaskFinished({ status: "OK" }), true);
  });

  it("registers query keys and yields them once on finish", () => {
    attachTask("UPID:pve1:1:1:1:resize:127:root@pam:", [["guest", 127]]);
    assert.equal(hasPendingTask("UPID:pve1:1:1:1:resize:127:root@pam:"), true);
    const keys = finishTask("UPID:pve1:1:1:1:resize:127:root@pam:");
    assert.deepEqual(keys, [["guest", 127]]);
    assert.equal(finishTask("UPID:pve1:1:1:1:resize:127:root@pam:"), null);
  });

  it("ignores tasks with no invalidation keys", () => {
    attachTask("UPID:pve1:1:1:1:qmstart:127:root@pam:");
    assert.equal(hasPendingTask("UPID:pve1:1:1:1:qmstart:127:root@pam:"), false);
    assert.equal(finishTask("UPID:pve1:1:1:1:qmstart:127:root@pam:"), null);
  });

  it("merges keys when the same UPID is attached twice", () => {
    attachTask("UPID:pve1:1:1:1:qmsnapshot:127:root@pam:", [["snaps", 127]]);
    attachTask("UPID:pve1:1:1:1:qmsnapshot:127:root@pam:", [["guest", 127]]);
    assert.deepEqual(finishTask("UPID:pve1:1:1:1:qmsnapshot:127:root@pam:"), [["snaps", 127], ["guest", 127]]);
  });
});
