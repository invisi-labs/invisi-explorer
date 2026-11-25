import { expect, it } from "vitest";
import { broadcasterScopes, broadcasterDetailPath } from "./broadcasterScopes";
const operator = "0x" + "1".repeat(40);
const row = (
  chainId: string,
  protocolAddress: string,
  host = "relay.example",
) => ({
  operator,
  chainId,
  protocolAddress,
  endpoint: `https://${host}/v1/chains/${chainId}/${protocolAddress}`,
  observation: {
    status: chainId === "97" ? "available" : "halted",
    tokens: [{ tokenAddress: chainId }],
  },
});
const a = row("97", "0xAAA"),
  b = row("1", "0xBBB"),
  c = row("97", "0xCCC");
it("uses URL scope as a selection without discarding sibling networks or deployments", () => {
  const view = broadcasterScopes(
    { occurrences: [a, b, c] },
    { chainId: "97", protocolAddress: "0xccc" },
  );
  expect(view.selected).toBe(c);
  expect(view.networks.map((x) => x.chainId)).toEqual(["97", "1"]);
  expect(view.deployments).toEqual([a, c]);
  expect(view.rows).toHaveLength(3);
  expect(view.selected.observation.tokens[0].tokenAddress).toBe("97");
  expect(
    broadcasterScopes({ occurrences: [a, b, c] }, { chainId: "1" }).selected,
  ).toBe(b);
});
it("does not pool another endpoint origin or operator into one service", () => {
  const other = row("56", "0xDDD", "other.example"),
    wrongOperator = { ...row("10", "0xEEE"), operator: "0x" + "2".repeat(40) };
  expect(
    broadcasterScopes(
      { occurrences: [a, b, other, wrongOperator] },
      { chainId: "97" },
    ).rows,
  ).toEqual([a, b]);
});
it("handles legacy single records and unavailable default selections explicitly", () => {
  expect(broadcasterScopes(a, {}).selected).toBe(a);
  const view = broadcasterScopes(
    { occurrences: [a, b] },
    { chainId: "8453", protocolAddress: "0xmissing" },
  );
  expect(view.selected).toBe(a);
  expect(view.selectionMissing).toBe(true);
  expect(broadcasterScopes({ occurrences: [] }, {}).selected).toBeUndefined();
});
it("fetches all registrations for the operator instead of sending UI scope filters", () => {
  expect(broadcasterDetailPath(operator)).toBe("/broadcasters/" + operator);
});
