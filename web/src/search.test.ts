import { afterEach, expect, it, vi } from "vitest";
import { searchPublic, validateTypedSearch, searchTypes } from "./search";
afterEach(() => vi.unstubAllGlobals());
const hash = "0x" + "a".repeat(64),
  address = "0x" + "b".repeat(40);
it("validates identifier shape for the selected type and keeps private materials local", () => {
  expect(searchTypes).toHaveLength(7);
  expect(validateTypedSearch(address, "address")).toBeNull();
  expect(validateTypedSearch(hash, "transaction")).toBeNull();
  expect(validateTypedSearch(address, "note")).toBe("invalid");
  expect(validateTypedSearch("seed: secret words", "all")).toBe("secret");
});
it("posts the selected type and retains deployment scope in filtered results", async () => {
  const fetcher = vi.fn(async (_url: unknown, _init: any) => ({
    ok: true,
    json: async () => ({
      results: [
        { type: "note", hash, chainId: "97", protocolAddress: address },
        { type: "transaction", hash, chainId: "1", protocolAddress: address },
      ],
    }),
  }));
  vi.stubGlobal("fetch", fetcher);
  const data = await searchPublic(hash, "note");
  expect(data.results).toEqual([
    { type: "note", hash, chainId: "97", protocolAddress: address },
  ]);
  expect(JSON.parse(fetcher.mock.calls[0][1].body)).toEqual({
    query: hash,
    type: "note",
  });
});
it("supports older servers without hiding truncated search coverage", async () => {
  const fetcher = vi
    .fn()
    .mockResolvedValueOnce({
      ok: false,
      status: 400,
      json: async () => ({ error: { code: "INVALID_PARAMS" } }),
    })
    .mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        results: [{ type: "transaction", hash }],
        truncated: true,
      }),
    });
  vi.stubGlobal("fetch", fetcher);
  const data = await searchPublic(hash, "note");
  expect(data.results).toEqual([]);
  expect(data.truncated).toBe(true);
  expect(JSON.parse(fetcher.mock.calls[1][1].body)).toEqual({ query: hash });
});
it("does not send known private material or retry unavailable services", async () => {
  const fetcher = vi.fn();
  vi.stubGlobal("fetch", fetcher);
  await expect(searchPublic("ipn:private", "note")).rejects.toMatchObject({
    code: "SECRET_MATERIAL",
  });
  expect(fetcher).not.toHaveBeenCalled();
  fetcher.mockResolvedValue({
    ok: false,
    status: 503,
    json: async () => ({ error: { code: "UNAVAILABLE" } }),
  });
  await expect(searchPublic(hash, "note")).rejects.toMatchObject({
    code: "UNAVAILABLE",
  });
  expect(fetcher).toHaveBeenCalledTimes(1);
});
