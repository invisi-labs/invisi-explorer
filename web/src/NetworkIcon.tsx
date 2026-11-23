import { useState } from "react";

// Match chain IDs rather than display names so custom network names stay safe.
const networks: { asset: string; chainIds: number[] }[] = [
  { asset: "ethereum", chainIds: [1, 11155111, 17000, 560048] },
  { asset: "smartchain", chainIds: [56, 97, 204, 5611] },
  { asset: "base", chainIds: [8453, 84532] },
  { asset: "arbitrum", chainIds: [42161, 42170, 421614] },
  { asset: "optimism", chainIds: [10, 11155420] },
  { asset: "polygon", chainIds: [137, 80002] },
  { asset: "avalanchec", chainIds: [43114, 43113] },
  { asset: "xdai", chainIds: [100, 10200] },
  { asset: "zksync", chainIds: [324, 300] },
  { asset: "linea", chainIds: [59144, 59141] },
  { asset: "scroll", chainIds: [534352, 534351] },
  { asset: "mantle", chainIds: [5000, 5003] },
  { asset: "blast", chainIds: [81457, 168587773] },
  { asset: "celo", chainIds: [42220, 11142220] },
];
const icons = new Map(networks.flatMap(({ asset, chainIds }) =>
  chainIds.map((id) => [String(id), `/assets/networks/${asset}.png`] as const),
));

export function networkIconPath(chainId?: string | number): string | undefined {
  return icons.get(String(chainId));
}

export function NetworkIcon({ chainId }: { chainId?: string | number }) {
  const src = networkIconPath(chainId);
  const [failedSrc, setFailedSrc] = useState<string>();
  if (src && failedSrc !== src) {
    return <img className="network-icon" src={src} alt="" aria-hidden="true"
      width="18" height="18" onError={() => setFailedSrc(src)} />;
  }
  return (
    <svg className="network-icon network-symbol" aria-hidden="true" width="18" height="18"
      viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
      <circle cx="12" cy="12" r="9" />
      <path d="M3 12h18M12 3c-6 6-6 12 0 18 6-6 6-12 0-18Z" />
    </svg>
  );
}
