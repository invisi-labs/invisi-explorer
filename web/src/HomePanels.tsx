import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import type { Key } from "./i18n";
export function OverviewStats({
  totals,
  networkCount,
  t,
}: {
  totals?: Record<string, any>;
  networkCount?: number;
  t: (key: Key) => string;
}) {
  return (
    <div className="stats home-stats">
      {[
        {
          key: "transactions",
          href: "/transactions",
          label: "transactions",
          value: totals?.transactions,
        },
        { key: "notes", href: "/notes", label: "notes", value: totals?.notes },
        {
          key: "grants",
          href: "/grants",
          label: "grants",
          value: totals?.grants,
        },
        {
          key: "networks",
          href: "/networks",
          label: "supportedNetworks",
          value: networkCount,
        },
      ].map((item) => (
        <Link className="card" to={item.href} key={item.key}>
          <span>{t(item.label as Key)}</span>
          <strong>{item.value ?? "—"}</strong>
        </Link>
      ))}
    </div>
  );
}
export function RecentTransactions({
  rows,
  t,
  record,
  state,
}: {
  rows: Record<string, any>[];
  t: (key: Key) => string;
  record: (row: Record<string, any>) => ReactNode;
  state?: ReactNode;
}) {
  return (
    <section className="card home-recent">
      <div className="section-head">
        <h2>{t("latestTransactions")}</h2>
        <Link to="/transactions">{t("viewAll")}</Link>
      </div>
      {state}
      {rows.length > 0 && (
        <div className="home-transaction-list">
          {rows.map((row, i) => (
            <article
              key={[
                row.chainId,
                row.protocolAddress,
                row.txHash ?? row.transactionHash,
                i,
              ].join(":")}
              className="home-transaction-row"
            >
              {record(row)}
            </article>
          ))}
        </div>
      )}
      <Link className="home-panel-footer" to="/transactions">
        {t("viewAllTransactions")}
      </Link>
    </section>
  );
}
