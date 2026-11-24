import { Link } from "react-router-dom";
import { CopyButton } from "./CopyButton";
import type { Key } from "./i18n";

export function AddressReference({ value, kind, explorerUrl, internalHref, compact = false, displayLabel, t }: {
  value: string;
  compact?: boolean;
  displayLabel?: string;
  kind?: "tx" | "address" | "token";
  explorerUrl?: string;
  internalHref?: string;
  t: (key: Key) => string;
}) {
  let externalHref: string | undefined;
  const length = kind === "tx" ? 64 : 40;
  try {
    if (explorerUrl && kind && new RegExp(`^0x[0-9a-fA-F]{${length}}$`).test(value)) {
      const base = new URL(explorerUrl);
      if (base.protocol === "https:" && !base.username && !base.password) {
        externalHref = `${base.origin}${base.pathname.replace(/\/$/, "")}/${kind}/${value}`;
      }
    }
  } catch { /* Keep identifiers copyable when network metadata is unavailable. */ }
  const label = displayLabel ?? (compact && value.length > 18 ? value.slice(0, 8) + "…" + value.slice(-6) : value);
  return <span className="address">
    {internalHref
      ? <Link className="address-value" to={internalHref}><code title={value}>{label}</code></Link>
      : <code className="address-value" title={value}>{label}</code>}
    <span className="address-actions">
      <CopyButton value={value} t={t}/>
      {externalHref && <a className="icon" href={externalHref} target="_blank" rel="noopener noreferrer" aria-label={t("external")} title={t("external")}>
        <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true"><path d="M7 17 17 7M7 7h10v10"/></svg>
      </a>}
    </span>
  </span>;
}
