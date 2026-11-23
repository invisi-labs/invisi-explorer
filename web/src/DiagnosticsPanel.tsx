import type { ReactNode } from "react";
import type { Key } from "./i18n";
import { finalityKey, fullTime } from "./public";

type Anchor = { blockNumber?: string; blockHash?: string };
export type DiagnosticRecord = {
  chainId: string;
  status?: string;
  reason?: string;
  indexedHead?: Anchor;
  chainHead?: Anchor;
  finality?: { finalized?: { status?: string; block?: Anchor | null } };
  updatedAt?: string;
};
export type DiagnosticsResponse = { items: DiagnosticRecord[] };

export function DiagnosticsPanel({
  items,
  t,
  network,
}: {
  items: DiagnosticRecord[];
  t: (key: Key) => string;
  network: (record: DiagnosticRecord) => ReactNode;
}) {
  const statusLabels = new Map<string, Key>([
    ["ready", "syncReady"],
    ["unavailable", "syncUnavailable"],
    ["not_checked", "syncNotChecked"],
  ]);
  return (
    <section className="card">
      <h2>{t("diagnostics")}</h2>
      {!items.length ? (
        <p>{t("empty")}</p>
      ) : (
        <div className="table-wrap">
          <table className="diagnostics-table">
            <thead>
              <tr>
                {(
                  [
                    "network",
                    "sync",
                    "indexed",
                    "chainHead",
                    "confirmed",
                    "backendUpdated",
                  ] as const
                ).map((key) => (
                  <th scope="col" key={key}>
                    {t(key)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map((record) => {
                const finalized = record.finality?.finalized;
                const statusLabel = statusLabels.get(record.status ?? "");
                const finalityLabel =
                  finalized?.status === "available"
                    ? finalized.block?.blockNumber || t("unknown")
                    : t(finalityKey(finalized?.status));
                return (
                  <tr key={record.chainId}>
                    <td data-label={t("network")}>{network(record)}</td>
                    <td data-label={t("sync")}>
                      <span
                        className={`badge${record.status === "ready" ? " good" : record.status === "unavailable" ? " bad" : ""}`}
                      >
                        {t(statusLabel ?? "unknown")}
                      </span>
                      {!statusLabel && record.status && (
                        <>
                          {" "}
                          <code>{record.status}</code>
                        </>
                      )}
                      {record.reason && (
                        <div>
                          <code>{record.reason}</code>
                        </div>
                      )}
                    </td>
                    <td data-label={t("indexed")}>
                      {record.indexedHead?.blockNumber || "—"}
                    </td>
                    <td data-label={t("chainHead")}>
                      {record.chainHead?.blockNumber || "—"}
                    </td>
                    <td data-label={t("confirmed")}>{finalityLabel}</td>
                    <td data-label={t("backendUpdated")}>
                      {record.updatedAt && record.updatedAt !== "0"
                        ? fullTime(record.updatedAt)
                        : "—"}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
