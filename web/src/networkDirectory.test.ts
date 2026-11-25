import { expect, it } from "vitest";
import {
  networkGroups,
  transactionPath,
  transactionFilterValues,
} from "./networkDirectory";
const a = "0x" + "a".repeat(40),
  b = "0x" + "b".repeat(40);
it("counts unique networks while preserving deployments and their own observations", () => {
  const groups = networkGroups(
    [
      { chainId: "97", protocolAddress: a, name: "BSC" },
      { chainId: "97", protocolAddress: b, name: "BSC" },
      { chainId: "97", protocolAddress: a.toUpperCase(), name: "duplicate" },
      { chainId: "1", protocolAddress: a, name: "Ethereum" },
    ],
    [
      { chainId: "97", protocolAddress: a, status: "ready" },
      { chainId: "97", protocolAddress: b, status: "unavailable" },
    ],
  );
  expect(groups).toHaveLength(2);
  const bsc = groups.find((g) => g.chainId === "97")!;
  expect(bsc.deployments).toHaveLength(2);
  expect(bsc.deployments[0].observation?.status).toBe("ready");
  expect(bsc.deployments[1].observation?.status).toBe("unavailable");
  expect(
    groups.find((g) => g.chainId === "1")!.deployments[0].observation,
  ).toBeUndefined();
});
it("never assigns a chain-only observation to several deployments", () => {
  const groups = networkGroups(
    [
      { chainId: "97", protocolAddress: a },
      { chainId: "97", protocolAddress: b },
    ],
    [{ chainId: "97", status: "ready" }],
  );
  expect(groups[0].deployments.every((d) => !d.observation)).toBe(true);
});
it("keeps only supported transaction filters and never carries a cursor between filter changes", () => {
  const filters = transactionFilterValues(
    "?chainId=97&protocolAddress=" +
      a +
      "&kind=shield&tokenAddress=" +
      b +
      "&cursor=old&limit=1&private=value",
  );
  expect(transactionPath(filters)).toBe(
    "/transactions?chainId=97&protocolAddress=" +
      a +
      "&kind=shield&tokenAddress=" +
      b,
  );
  expect(
    transactionPath({ ...filters, chainId: "1", protocolAddress: "" }),
  ).not.toContain("cursor");
});

it("normalizes checksum address parameters without losing active filters", () => {
  const filters = transactionFilterValues(
    "?chainId=97&protocolAddress=" +
      a.toUpperCase() +
      "&tokenAddress=" +
      b.toUpperCase(),
  );
  expect(filters.protocolAddress).toBe(a);
  expect(filters.tokenAddress).toBe(b);
});
