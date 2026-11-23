import { readFileSync, existsSync } from "node:fs";
import { describe, it, expect } from "vitest";
import { networkIconPath } from "./NetworkIcon";

describe("Explorer production assets", () => {
  it("ships a local PNG for each supported network", () => {
    for (const id of [1, 56, 8453, 42161, 10, 137, 43114, 100, 324, 59144, 534352, 5000, 81457, 42220, 204]) {
      const path = networkIconPath(id);
      expect(path).toBeDefined();
      const asset = new URL(`../public${path}`, import.meta.url);
      expect(existsSync(asset)).toBe(true);
      expect(readFileSync(asset).subarray(0, 8).toString("hex")).toBe("89504e470d0a1a0a");
    }
  });
  it("serves the brand image from the backend's public assets directory", () => {
    const entry = readFileSync(new URL("./main.tsx", import.meta.url), "utf8");
    const source = entry.match(/className="brand-mark"\s+src="([^"]+)"/)?.[1];
    expect(source).toMatch(/^\/assets\/[^/]+\.svg$/);
    const asset = new URL(`../public${source}`, import.meta.url);
    expect(existsSync(asset)).toBe(true);
    expect(readFileSync(asset, "utf8")).toContain('<svg xmlns="http://www.w3.org/2000/svg"');
  });
});
