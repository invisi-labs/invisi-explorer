import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { GrantStateEffect } from "./GrantStateEffect";
import { fullTime } from "./public";
import type { Key } from "./i18n";
type RecordData = Record<string, any>;
type Props = {
  data: RecordData; network?: ReactNode; t: (key: Key) => string;
  address: (value: string, kind?: "address" | "token") => ReactNode;
  transaction: (row: RecordData) => ReactNode;
  amount: (value: string, row: RecordData) => ReactNode;
  href: (type: string, row: RecordData, id: string) => string;
};
export function GrantHistory({data,network,t,address,transaction,amount,href}: Props) {
  const items: RecordData[] = data.activities?.items ?? [];
  const status = data.chainState?.status ?? data.status;
  return <>
    <section className="card">
      <h2>{t("publicInfo")}</h2>
      <dl className="public-fields">
        <dt>{t("network")}</dt><dd>{network ?? data.chainId}</dd>
        <dt>{t("protocol")}</dt><dd>{address(data.protocolAddress)}</dd>
        <dt>{t("token")}</dt><dd>{address(data.tokenAddress,"token")}</dd>
        <dt>{t("grantOwner")}</dt><dd>{address(data.owner ?? data.payer)}</dd>
        <dt>{t("grantIdLabel")}</dt><dd><code>{data.grantId}</code></dd>
        <dt>{t("creditGrantStatus")}</dt><dd>{t(status === "1" ? "creditGrantActive" : status === "2" ? "creditGrantRevoked" : status === "0" ? "creditGrantUnused" : "fundingUnknown")}</dd>
      </dl>
    </section>
    <section className="card">
      <h2>{t("creditHistory")}</h2>
      <p className="muted">{t("creditHistoryBoundary")}</p>
      {data.historyStatus === "backfilling" && <p className="pagination-message" role="status">{t("creditHistoryPending")}</p>}
      {!items.length && <p>{t("creditHistoryEmpty")}</p>}
      <ol className="credit-history">{items.map((row,i)=>{
        const actions: string[] = row.actions ?? [row.kind];
        const payment = actions.includes("creditDraw");
        return <li className="credit-history-item" key={`${row.txHash}:${row.operationIndex}:${i}`}>
          <div className="section-head"><h3>{actions.map(action=>t(action as Key)).join(" · ")}</h3><time>{fullTime(row.timestamp)}</time></div>
          <dl className="public-fields">
            <dt>{t("transactionHashLabel")}</dt><dd>{transaction(row)}</dd>
            <dt>{t("creditStateChange")}</dt><dd><GrantStateEffect effect={row.stateEffect} t={t}/></dd>
            {payment && <><dt>{t("creditSlotAmount")}</dt><dd>{row.slotFundingAmount !== undefined ? amount(row.slotFundingAmount,row) : t("creditSharedAmount")}</dd></>}
          </dl>
          {(row.creditOutputs?.length > 0 || row.noteOutputs?.length > 0) && <details>
            <summary>{t("creditOperationOutputs")}</summary>
            {row.creditOutputs?.map((state: RecordData,j: number)=><p key={`credit:${j}`}>
              {t("creditCommitmentLabel")}{" "}<Link to={href("credit-states",row,state.creditCommitment)}><code>{state.creditCommitment}</code></Link>
            </p>)}
            {row.noteOutputs?.map((note: RecordData,j: number)=><p key={`note:${j}`}>
              {t("noteCommitmentLabel")}{" "}<Link to={href("notes",row,note.noteCommitment)}><code>{note.noteCommitment}</code></Link>
            </p>)}
          </details>}
        </li>;
      })}</ol>
    </section>
  </>;
}
