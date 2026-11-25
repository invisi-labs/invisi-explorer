import type { ReactNode } from "react";
import type { Key } from "./i18n";
import { fullTime } from "./public";
type Row = Record<string, any>;
export function RecordTable({
  rows,
  t,
  reference,
  network,
}: {
  rows: Row[];
  t: (key: Key) => string;
  reference: (row: Row) => ReactNode;
  network: (row: Row) => ReactNode;
}) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>{t("hash")}</th>
            <th>{t("network")}</th>
            <th>{t("protocol")}</th>
            <th>{t("operation")}</th>
            <th>{t("time")}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, i) => (
            <tr
              key={[
                row.id,
                row.rootId,
                row.txHash,
                row.operationIndex,
                row.domain,
                i,
              ].join(":")}
            >
              <td data-label={t("hash")}>{reference(row)}</td>
              <td data-label={t("network")}>{network(row)}</td>
              <td data-label={t("protocol")}>
                <span className="mono" title={row.protocolAddress}>
                  {row.protocolAddress
                    ? row.protocolAddress.slice(0, 10) +
                      "…" +
                      row.protocolAddress.slice(-8)
                    : "—"}
                </span>
              </td>
              <td data-label={t("operation")}>
                {row.kind
                  ? t(row.kind as Key)
                  : (row.namespace ?? row.domain ?? "—")}
              </td>
              <td data-label={t("time")}>
                {row.timestamp ? fullTime(row.timestamp) : "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
