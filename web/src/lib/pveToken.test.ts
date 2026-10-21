import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { composePVEToken, pveMigrateToken } from "./pveToken.ts";

describe("composePVEToken", () => {
  it("joins user and secret", () => {
    assert.equal(composePVEToken("root@pam!hv", "abc-123"), "root@pam!hv=abc-123");
  });
  it("accepts a full token pasted in the secret field", () => {
    assert.equal(composePVEToken("root@pam!hv", "root@pam!hv=abc-123"), "root@pam!hv=abc-123");
  });
  it("strips the PVEAPIToken prefix", () => {
    assert.equal(composePVEToken("", "PVEAPIToken=root@pam!hv=abc"), "root@pam!hv=abc");
  });
  it("needs both parts", () => {
    assert.equal(composePVEToken("root@pam!hv", ""), "");
    assert.equal(composePVEToken("", "abc-123"), "");
  });
});

describe("pveMigrateToken", () => {
  it("prefixes PVEAPIToken", () => {
    assert.equal(pveMigrateToken("root@pam!mig", "secret"), "PVEAPIToken=root@pam!mig=secret");
  });
});
