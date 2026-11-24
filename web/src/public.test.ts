import { describe, it, expect, vi } from "vitest";
import {
  exactAmount,
  age,
  validateSearch,
  request,
  objectIdentity,
  statusKey,
  showsSourceRecognition,
  collectionFor,
  finalityKey,
} from "./public";
describe("public privacy boundary", () => {
  it("blocks known secret encodings locally", () => {
    for (const secret of [
      "0iabcdef",
      "ipn:abc",
      "private_key:abc",
      "view key:abc",
      "mnemonic: word",
    ])
      expect(validateSearch(secret)).toBe("secret");
    expect(validateSearch("0x" + "a".repeat(64))).toBeNull();
  });
  it("does not claim arbitrary 32 byte secrets are identifiable", () => {
    expect(validateSearch("0x" + "1".repeat(64))).toBeNull();
  });
  it("posts search payload without placing terms in URL", async () => {
    const fetch = vi
      .fn()
      .mockResolvedValue({ ok: true, json: async () => ({ results: [] }) });
    vi.stubGlobal("fetch", fetch);
    await request("/search", undefined, { query: "0x" + "a".repeat(64) });
    expect(fetch.mock.calls[0][0]).toBe("/api/v1/search");
    expect(fetch.mock.calls[0][1].method).toBe("POST");
    vi.unstubAllGlobals();
  });
  it("preserves API errors and does not infer recognition", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: false,
        status: 503,
        json: async () => ({ error: { code: "PROVIDER_UNAVAILABLE" } }),
      }),
    );
    await expect(request("/notes")).rejects.toEqual({
      code: "PROVIDER_UNAVAILABLE",
      status: 503,
    });
    vi.unstubAllGlobals();
  });
});
describe("exact public rendering", () => {
  it("never rounds values beyond floating point precision", () => {
    expect(exactAmount("9007199254740993123456", 6)).toBe(
      "9,007,199,254,740,993.123456",
    );
    expect(exactAmount("1", 18)).toBe("0.000000000000000001");
    expect(exactAmount("123")).toBe("123 (raw)");
  });
  it("uses full timestamp at 24 hours and future timestamps", () => {
    const now = Date.parse("2026-11-09T12:00:00Z");
    const t = (k: string) => k;
    expect(age(now - 59999, now, t)).toBe("justNow");
    expect(age(now - 3600000, now, t)).toBe("1 hours");
    expect(age(now - 86400000, now, t)).toMatch(/^2026-/);
    expect(age(now + 1000, now, t)).toMatch(/^2026-/);
  });
});

it("formats string metadata decimals without floating point amounts", () => {
  expect(exactAmount("1234567", "6")).toBe("1.234567");
  expect(exactAmount("123", "256")).toBe("123 (raw)");
});

it("keeps entity links in their own public namespace", () => {
  const row = {
    txHash: "transaction",
    noteCommitment: "note",
    creditCommitment: "credit",
    nullifier: "nullifier",
    treeId: "1",
    id: "protocol:1",
  };
  expect(objectIdentity("transactions", row)).toBe("transaction");
  expect(objectIdentity("notes", row)).toBe("note");
  expect(objectIdentity("credit-states", row)).toBe("credit");
  expect(objectIdentity("nullifiers", row)).toBe("nullifier");
  expect(objectIdentity("trees", row)).toBe("protocol:1");
  expect(objectIdentity("grants", { ...row, id: "grant-ref" })).toBe(
    "grant-ref",
  );
});
it("labels current recognition states without inventing aliases", () => {
  expect(statusKey("pending_proof")).toBe("awaitingProof");
  expect(statusKey("pending_dependencies")).toBe("recognitionPending");
  expect(statusKey("chain_reorged")).toBe("reorg");
  expect(statusKey("not_applicable")).toBe("notApplicable");
  expect(statusKey("RECOGNIZED")).toBe("unknown");
});

it("maps public search kinds to their typed collection", () => {
  expect(collectionFor("note")).toBe("notes");
  expect(collectionFor("creditState")).toBe("credit-states");
  expect(collectionFor("address")).toBe("addresses");
  expect(collectionFor("transaction")).toBe("transactions");
});

it("keeps finality availability separate from execution and recognition", () => {
  expect(statusKey("pending_confirmation")).toBe("awaitingFinality");
  expect(finalityKey("available")).toBe("finalized");
  expect(finalityKey("unsupported")).toBe("finalityUnsupported");
  expect(finalityKey("unavailable")).toBe("finalityUnavailable");
  expect(finalityKey("inconsistent")).toBe("finalityConflict");
  expect(finalityKey(undefined)).toBe("unknown");
});

it("hides inapplicable source recognition without hiding pending or mixed transactions", () => {
  expect(showsSourceRecognition("credit-states", {})).toBe(false);
  for (const type of ["nullifiers", "trees", "roots", "grants", "broadcasters", "diagnostics"]) {
    expect(showsSourceRecognition(type, {})).toBe(false);
  }
  expect(showsSourceRecognition("nullifiers", {kind:"transfer",recognitionStatus:"recognized"})).toBe(false);
  expect(showsSourceRecognition("transactions", {recognition: [{status: "not_applicable"}]})).toBe(false);
  expect(showsSourceRecognition("transactions", {recognition: {recognitionState: "not_applicable"}})).toBe(false);
  expect(showsSourceRecognition("transactions", {recognitionStatus: "not_applicable"})).toBe(false);
  expect(showsSourceRecognition("transactions", {recognition: [{status: "not_applicable"}, {status: "pending_source"}]})).toBe(true);
  expect(showsSourceRecognition("transactions", {})).toBe(true);
  expect(showsSourceRecognition("notes", {recognition: [{status: "recognized"}]})).toBe(true);
});
