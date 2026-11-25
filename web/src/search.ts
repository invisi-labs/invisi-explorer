import { request, validateSearch, type ApiError } from "./public";
import type { Key } from "./i18n";
export const searchTypes = [
  "all",
  "transaction",
  "address",
  "note",
  "creditState",
  "nullifier",
  "root",
] as const;
export type SearchType = (typeof searchTypes)[number];
export const searchTypeLabels: Record<SearchType, Key> = {
  all: "searchAllTypes",
  transaction: "searchTransactions",
  address: "searchAddresses",
  note: "searchNotes",
  creditState: "searchCreditStates",
  nullifier: "searchNullifiers",
  root: "searchRoots",
};
export const searchPlaceholders: Record<SearchType, Key> = {
  all: "searchHint",
  transaction: "searchTransactionHint",
  address: "searchAddressHint",
  note: "searchNoteHint",
  creditState: "searchCreditHint",
  nullifier: "searchNullifierHint",
  root: "searchRootHint",
};
export function validateTypedSearch(
  query: string,
  type: SearchType,
): "secret" | "invalid" | null {
  const invalid = validateSearch(query);
  if (invalid) return invalid;
  const value = query.trim();
  return (type === "all"
    ? /^(?:0x[\da-f]{40}|0x[\da-f]{64})$/i
    : type === "address"
      ? /^0x[\da-f]{40}$/i
      : /^0x[\da-f]{64}$/i
  ).test(value)
    ? null
    : "invalid";
}
type SearchResponse = { results: Record<string, any>[]; truncated?: boolean };
export async function searchPublic(
  query: string,
  type: SearchType,
  signal?: AbortSignal,
): Promise<SearchResponse> {
  const invalid = validateTypedSearch(query, type);
  if (invalid)
    throw {
      code: invalid === "secret" ? "SECRET_MATERIAL" : "INVALID_PARAMS",
      status: 400,
    };
  let data: SearchResponse;
  try {
    data = await request<SearchResponse>("/search", signal, {
      query: query.trim(),
      ...(type === "all" ? {} : { type }),
    });
  } catch (error) {
    const e = error as ApiError;
    if (
      type === "all" ||
      e.status !== 400 ||
      e.code !== "INVALID_PARAMS" ||
      signal?.aborted
    )
      throw error;
    data = await request<SearchResponse>("/search", signal, {
      query: query.trim(),
    });
  }
  return {
    ...data,
    results:
      type === "all"
        ? data.results
        : data.results.filter((row) => row.type === type),
  };
}
