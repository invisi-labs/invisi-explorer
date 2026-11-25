import { expect, it } from "vitest";
import { pagePath, normalizeRecordPage, recordRows } from "./recordPages";
it("retains network/deployment/namespace parameters when adding a page cursor", () => {
  const path = pagePath(
    "/nullifiers/hash?chainId=97&protocolAddress=0xabc&namespace=note",
    "next/+",
  );
  const url = new URL(path, "http://localhost");
  expect(url.searchParams.get("chainId")).toBe("97");
  expect(url.searchParams.get("namespace")).toBe("note");
  expect(url.searchParams.get("limit")).toBe("20");
  expect(url.searchParams.get("cursor")).toBe("next/+");
  expect(
    [...url.searchParams.keys()].filter((k) => k === "limit"),
  ).toHaveLength(1);
});
it("promotes nested address activity pagination without dropping rows", () => {
  const page = normalizeRecordPage({
    address: "a",
    activities: {
      items: [{ txHash: "first" }],
      total: "205",
      nextCursor: "next",
    },
  });
  expect(page).toMatchObject({ nextCursor: "next", total: "205" });
  expect(recordRows(page)).toEqual([{ txHash: "first" }]);
});
it("retains occurrence and flat pages including empty final pages", () => {
  expect(recordRows({ occurrences: [{ id: "occurrence" }] })).toEqual([
    { id: "occurrence" },
  ]);
  expect(recordRows({ items: [] })).toEqual([]);
  expect(recordRows({ txHash: "single" })).toBeUndefined();
});
