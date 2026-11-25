type Row = Record<string, any>;
export type BroadcasterSelection = {
  chainId?: string;
  protocolAddress?: string;
};
const same = (a: unknown, b: unknown) =>
  typeof a === "string" &&
  typeof b === "string" &&
  a.toLowerCase() === b.toLowerCase();
function serviceOrigin(endpoint: unknown): string {
  try {
    const url = new URL(String(endpoint));
    if (url.protocol === "https:" && !url.username && !url.password)
      return url.origin;
  } catch {}
  return String(endpoint ?? "");
}
export function broadcasterDetailPath(operator: string): string {
  return "/broadcasters/" + encodeURIComponent(operator);
}
export function broadcasterScopes(data: Row, selection: BroadcasterSelection) {
  const entries: Row[] = Array.isArray(data.occurrences)
    ? data.occurrences
    : data.chainId
      ? [data]
      : [];
  const anchor =
    entries.find(
      (row) =>
        (!selection.chainId || String(row.chainId) === selection.chainId) &&
        (!selection.protocolAddress ||
          same(row.protocolAddress, selection.protocolAddress)),
    ) ??
    entries.find(
      (row) => selection.chainId && String(row.chainId) === selection.chainId,
    ) ??
    entries[0];
  const rows = anchor
    ? entries.filter(
        (row) =>
          same(row.operator, anchor.operator) &&
          serviceOrigin(row.endpoint) === serviceOrigin(anchor.endpoint),
      )
    : [];
  const networks = rows.filter(
    (row, index) =>
      rows.findIndex(
        (other) => String(other.chainId) === String(row.chainId),
      ) === index,
  );
  const chain =
    rows.find((row) => String(row.chainId) === selection.chainId)?.chainId ??
    anchor?.chainId;
  const deployments = rows.filter(
    (row) => String(row.chainId) === String(chain),
  );
  const selected =
    deployments.find((row) =>
      same(row.protocolAddress, selection.protocolAddress),
    ) ?? deployments[0];
  const selectionMissing =
    !!selected &&
    !!(
      (selection.chainId && String(selected.chainId) !== selection.chainId) ||
      (selection.protocolAddress &&
        !same(selected.protocolAddress, selection.protocolAddress))
    );
  return { rows, networks, deployments, selected, selectionMissing };
}
