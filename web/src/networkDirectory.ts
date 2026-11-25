export type PublicNetwork = {
  chainId: string;
  protocolAddress: string;
  name?: string;
  environment?: string;
  explorerUrl?: string;
  codeHash?: string;
  startBlock?: string;
};
export type NetworkObservation = {
  chainId: string;
  protocolAddress?: string;
  status?: string;
  updatedAt?: string;
  indexedHead?: { blockNumber?: string };
  chainHead?: { blockNumber?: string };
  finality?: {
    finalized?: { status?: string; block?: { blockNumber?: string } | null };
  };
};
export type NetworkGroup = {
  chainId: string;
  name?: string;
  environment?: string;
  deployments: (PublicNetwork & { observation?: NetworkObservation })[];
};
export function networkGroups(
  networks: PublicNetwork[],
  observations: NetworkObservation[] = [],
): NetworkGroup[] {
  const groups = new Map<string, NetworkGroup>();
  for (const row of networks) {
    if (!row.chainId || !row.protocolAddress) continue;
    const chainId = String(row.chainId);
    let group = groups.get(chainId);
    if (!group) {
      group = {
        chainId,
        name: row.name,
        environment: row.environment,
        deployments: [],
      };
      groups.set(chainId, group);
    }
    if (
      !group.deployments.some(
        (d) =>
          d.protocolAddress.toLowerCase() === row.protocolAddress.toLowerCase(),
      )
    )
      group.deployments.push({ ...row, chainId });
  }
  for (const group of groups.values())
    for (const deployment of group.deployments) {
      deployment.observation =
        observations.find(
          (o) =>
            String(o.chainId) === group.chainId &&
            o.protocolAddress?.toLowerCase() ===
              deployment.protocolAddress.toLowerCase(),
        ) ??
        (group.deployments.length === 1
          ? observations.find(
              (o) => String(o.chainId) === group.chainId && !o.protocolAddress,
            )
          : undefined);
    }
  return [...groups.values()];
}
export type TransactionFilters = {
  chainId: string;
  protocolAddress: string;
  kind: string;
  tokenAddress: string;
};
export function transactionFilterValues(search: string): TransactionFilters {
  const params = new URLSearchParams(search);
  return {
    chainId: params.get("chainId") ?? "",
    protocolAddress: (params.get("protocolAddress") ?? "").toLowerCase(),
    kind: params.get("kind") ?? "",
    tokenAddress: (params.get("tokenAddress") ?? "").toLowerCase(),
  };
}
export function transactionPath(filters: Partial<TransactionFilters>) {
  const params = new URLSearchParams();
  for (const key of [
    "chainId",
    "protocolAddress",
    "kind",
    "tokenAddress",
  ] as const)
    if (filters[key]) params.set(key, filters[key]!);
  return "/transactions" + (params.size ? "?" + params.toString() : "");
}
