import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import { NetworkIcon, networkIconPath } from "./NetworkIcon";

describe("network icons", () => {
  it("uses the BNB logo for both mainnet and the live testnet", () => {
    expect(networkIconPath(56)).toBe("/assets/networks/smartchain.png");
    expect(networkIconPath("97")).toBe(networkIconPath(56));
    expect(renderToStaticMarkup(<NetworkIcon chainId="97" />)).toContain('src="/assets/networks/smartchain.png"');
  });

  it("shares logos between mainnets and their testnets", () => {
    for (const [mainnet, testnet] of [[1, 11155111], [8453, 84532], [42161, 421614], [10, 11155420], [137, 80002], [43114, 43113]]) {
      expect(networkIconPath(mainnet)).toBeDefined();
      expect(networkIconPath(testnet)).toBe(networkIconPath(mainnet));
    }
  });

  it("keeps unknown and missing networks on the default globe", () => {
    for (const chainId of [undefined, "", "unknown", 999999999]) {
      expect(networkIconPath(chainId)).toBeUndefined();
      const html = renderToStaticMarkup(<NetworkIcon chainId={chainId} />);
      expect(html).toContain("<svg");
      expect(html).not.toContain("<img");
    }
  });
});
