import {fullTimestamp} from "./displayPreferences";
export const fullTime = fullTimestamp;
export function age(
  value: unknown,
  now: number,
  t: (k: "justNow" | "minutes" | "hours") => string,
): string {
  const n =
    typeof value === "number" || /^\d+$/.test(String(value))
      ? Number(value) * (Number(value) < 1e12 ? 1000 : 1)
      : Date.parse(String(value));
  const diff = now - n;
  if (!Number.isFinite(diff) || diff < 0 || diff >= 86400000)
    return fullTime(value);
  if (diff < 60000) return t("justNow");
  if (diff < 3600000) return `${Math.floor(diff / 60000)} ${t("minutes")}`;
  return `${Math.floor(diff / 3600000)} ${t("hours")}`;
}
export function exactAmount(
  value: unknown,
  decimals?: number | string,
): string {
  const raw = String(value ?? "");
  if (!/^-?\d+$/.test(raw)) return "—";
  const negative = raw.startsWith("-");
  let v = negative ? raw.slice(1) : raw;
  const parsed =
    typeof decimals === "string" && /^(?:0|[1-9][0-9]{0,2})$/.test(decimals)
      ? Number(decimals)
      : decimals;
  const known =
    typeof parsed === "number" &&
    Number.isInteger(parsed) &&
    parsed >= 0 &&
    parsed <= 255;
  const d = known ? (parsed as number) : 0;
  v = v.padStart(d + 1, "0");
  const whole = (d ? v.slice(0, -d) : v)
    .replace(/^0+(?=\d)/, "")
    .replace(/\B(?=(\d{3})+(?!\d))/g, ",");
  const fraction = d ? v.slice(-d).replace(/0+$/, "") : "";
  return `${negative ? "-" : ""}${whole}${fraction ? "." + fraction : ""}${known ? "" : " (raw)"}`;
}
export function validateSearch(query: string): "secret" | "invalid" | null {
  if (
    /^(?:0i|ipn[:1_-]|(?:invisi[ _-]?)?(?:view|private|secret)[ _-]?key|(?:seed|mnemonic)[:\s])/i.test(
      query.trim(),
    )
  )
    return "secret";
  return /^(?:0x[\da-fA-F]{40}|0x[\da-fA-F]{64}|[A-Za-z0-9][A-Za-z0-9_.:-]{0,127})$/.test(
    query.trim(),
  )
    ? null
    : "invalid";
}
export type ApiError = { code: string; status: number };
export async function request<T>(
  path: string,
  signal?: AbortSignal,
  body?: unknown,
): Promise<T> {
  const response = await fetch("/api/v1" + path, {
    signal,
    credentials: "same-origin",
    cache: "no-store",
    headers: body ? { "Content-Type": "application/json" } : undefined,
    method: body ? "POST" : "GET",
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!response.ok) {
    let code = "UNAVAILABLE";
    try {
      const data = await response.json();
      code = data.error?.code ?? data.code ?? code;
    } catch {}
    throw { code, status: response.status } satisfies ApiError;
  }
  return response.json();
}

export function objectIdentity(
  type: string,
  row: Record<string, unknown>,
): string {
  const fields: Record<string, string[]> = {
    transactions: ["txHash", "transactionHash"],
    notes: ["noteCommitment"],
    "credit-states": ["creditCommitment"],
    nullifiers: ["nullifier"],
    roots: ["rootId"],
    trees: ["id"],
    grants: ["id"],
    broadcasters: ["operator", "id"],
  };
  for (const field of fields[type] ?? ["id", "hash"]) {
    if (typeof row[field] === "string" && row[field])
      return row[field] as string;
  }
  return "";
}
// Query availability is distinct from the operation's recognition decision.
export function recognitionQueryStatus(data: Record<string, unknown>): unknown {
  const status = data.status ?? data.recognitionState ?? "unknown";
  if (status !== "provider_unavailable") return status;
  const reason = data.reason ?? data.reasonCode;
  return reason === "INDEX_PENDING" ? "recognition_index_pending" : reason === "NOT_FOUND" ? "recognition_not_found" : status;
}
export function statusKey(value: unknown): import("./i18n").Key {
  const labels: Record<string, import("./i18n").Key> = {
    pending_source: "pending",
    pending_confirmation: "awaitingFinality",
    pending_proof: "awaitingProof",
    pending_dependencies: "recognitionPending",
    recognition_pending: "recognitionPending",
    recognized: "recognized",
    source_rejected: "rejected",
    provider_unavailable: "providerUnavailable",
    recognition_index_pending: "recognitionSyncing",
    recognition_not_found: "recognitionMissing",
    chain_reorged: "reorg",
    not_applicable: "notApplicable",
    verifying: "verifying",
    unsupported: "unsupported",
    true: "success",
    false: "failed",
    partial_success: "partialSuccess",
  };
  return labels[String(value)] ?? "unknown";
}

export function collectionFor(kind: string): string {
  return (
    (
      {
        transaction: "transactions",
        note: "notes",
        creditState: "credit-states",
        credit: "credit-states",
        nullifier: "nullifiers",
        grant: "grants",
        tree: "trees",
        root: "roots",
        address: "addresses",
        broadcaster: "broadcasters",
      } as Record<string, string>
    )[kind] ?? kind
  );
}

export function finalityKey(value: unknown): import("./i18n").Key {
  const labels: Record<string, import("./i18n").Key> = {
    available: "finalized",
    unsupported: "finalityUnsupported",
    unavailable: "finalityUnavailable",
    inconsistent: "finalityConflict",
  };
  return labels[String(value)] ?? "unknown";
}

// Unknown or missing evidence remains visible; only explicitly inapplicable records are hidden.
export function showsSourceRecognition(type: string, data: Record<string, unknown> | undefined): boolean {
  if (type && type !== "transactions" && type !== "notes") return false;
  const rows = Array.isArray(data?.recognition)
    ? data.recognition
    : data?.recognition && typeof data.recognition === "object" ? [data.recognition] : [];
  if (rows.length) return rows.some((row) =>
    !row || (row.status ?? row.recognitionState) !== "not_applicable");
  return (data?.recognitionStatus ?? data?.status) !== "not_applicable";
}
