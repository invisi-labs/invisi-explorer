import { useId } from "react";
import { amountView } from "./amounts";
import { CopyButton } from "./CopyButton";
import type { Key } from "./i18n";

export function AmountDisplay({ value, decimals, symbol, address, compact = false, t }: {
  value: unknown;
  decimals?: number | string;
  symbol: string;
  address?: string;
  compact?: boolean;
  t: (key: Key) => string;
}) {
  const tooltipId = useId();
  const view = amountView(value, decimals, compact);
  const full = `${view.copyValue}${symbol ? " " + symbol : ""}${view.raw ? " (" + t("rawUnits") + ")" : ""}`;
  return <span className="amount">
    <span className="amount-line">
      <span className="amount-number" title={view.valid ? full : undefined}
        aria-label={view.valid ? full : undefined} aria-describedby={view.valid ? tooltipId : undefined}
        tabIndex={view.valid ? 0 : undefined}>
        <span className="amount-value">{view.display}</span>
        {view.valid && <span id={tooltipId} role="tooltip" className="amount-tooltip">{full}</span>}
      </span>{" "}
      <span className="amount-token" title={address}>{symbol}</span>
      {view.valid && <CopyButton value={view.copyValue} t={t} className="icon amount-copy"/>}
    </span>
    {view.raw && <small className="raw-unit">{t("rawUnits")}</small>}
  </span>;
}
