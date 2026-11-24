import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import { CopyButton } from "./CopyButton";
import { compactIdentifier, grantStatusKey } from "./credit";
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
  return <>
    <section className="card">
      <h2>{t("publicInfo")}</h2>
      <dl className="public-fields">
        <dt>{t("network")}</dt><dd>{network ?? data.chainId}</dd>
        <dt>{t("protocol")}</dt><dd>{address(data.protocolAddress)}</dd>
        <dt>{t("token")}</dt><dd>{data.tokenAddress?.toLowerCase() === "0x0000000000000000000000000000000000000000" ? <span className="address" title={data.tokenAddress}>{"-"}<CopyButton value={data.tokenAddress} t={t}/></span> : address(data.tokenAddress,"token")}</dd>
        <dt>{t("grantOwner")}</dt><dd>{address(data.owner ?? data.payer)}</dd>
        <dt>{t("grantIdLabel")}</dt><dd><code>{data.grantId}</code></dd>
        <dt>{t("creditGrantStatus")}</dt><dd>{t(grantStatusKey(data))}</dd>
      </dl>
    </section>
    <section className="card">
      <h2>{t("creditHistory")}</h2>
      <p className="muted">{t("creditHistoryBoundary")}</p>
      {data.lifecycleStatus === "pre_revoked" && <p className="muted">{t("grantPreRevokedNotice")}</p>}
      {data.lifecycleStatus === "revoked_after_creation" && <p className="muted">{t("grantRevokedNotice")}</p>}
      {data.creationTransaction && <p>{t("grantCreationRecord")}{": "}{transaction(data.creationTransaction)}</p>}

      {data.historyStatus === "backfilling" && <p className="pagination-message" role="status">{t("creditHistoryPending")}</p>}
      {!items.length && <p>{t("creditHistoryEmpty")}</p>}
      <ol className="credit-history">{items.map((row,i)=>{
        const actions: string[] = row.actions ?? [row.kind];
        const payment = actions.includes("creditDraw");
        return <li className="credit-history-item" key={`${row.txHash}:${row.operationIndex}:${i}`}>
          <div className="section-head"><h3>{row.kind === "createCredit" ? t("createCredit") : actions.map(action=>t(action as Key)).join(" · ")}</h3><time>{fullTime(row.timestamp)}</time></div>
          <dl className="public-fields">
            <dt>{t("transactionHashLabel")}</dt><dd>{transaction(row)}</dd>
            <dt>{t("creditStateChange")}</dt><dd><GrantStateEffect effect={row.stateEffect} t={t}/></dd>
            {payment && <><dt>{t("creditSlotAmount")}</dt><dd>{row.slotFundingAmount !== undefined ? amount(row.slotFundingAmount,row) : t("creditSharedAmount")}</dd></>}
          </dl>
          {(row.creditOutputs?.length > 0 || row.noteOutputs?.length > 0) && <details className="grant-outputs">
            <summary><span className="grant-outputs-title">{t("grantOutputsTitle")}</span><span className="grant-outputs-count">{[
              row.creditOutputs?.length ? t("grantCreditOutputCount").replace("{count}",String(row.creditOutputs.length)) : "",
              row.noteOutputs?.length ? t("grantNoteOutputCount").replace("{count}",String(row.noteOutputs.length)) : "",
            ].filter(Boolean).join(" · ")}</span><svg className="grant-outputs-chevron" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><path d="m9 5 7 7-7 7"/></svg></summary>
            {(row.grantRefs?.length > 1) && <p className="muted">{t(row.creditOutputScope === "grant" ? "grantScopedOutputs" : "grantOutputsScope")}</p>}
            <div className="table-wrap"><table className="funding-table">
              <thead><tr><th>{t("grantOutputType")}</th><th>{t("commitments")}</th><th>{t("outputAmount")}</th><th>{t("treeIdLabel")}</th><th>{t("leafIndexLabel")}</th></tr></thead>
              <tbody>{[
                ...(row.creditOutputs ?? []).map((output: RecordData)=>({...output,type:"credit-states",commitment:output.creditCommitment})),
                ...(row.noteOutputs ?? []).map((output: RecordData)=>({...output,type:"notes",commitment:output.noteCommitment})),
              ].map((output: RecordData,j: number)=><tr key={`${output.type}:${j}`}>
                <td data-label={t("grantOutputType")}>{t(output.type === "notes" ? "noteCommitmentLabel" : "creditCommitmentLabel")}</td>
                <td data-label={t("commitments")}><span className="address"><Link to={href(output.type,row,output.commitment)}><code title={output.commitment}>{compactIdentifier(output.commitment)}</code></Link><CopyButton value={output.commitment} t={t}/></span></td>
                <td data-label={t("outputAmount")}>{output.type === "notes" && output.amount !== undefined ? amount(output.amount,row) : "—"}</td>
                <td data-label={t("treeIdLabel")}>{output.treeId ?? "—"}</td>
                <td data-label={t("leafIndexLabel")}>{output.leafIndex ?? "—"}</td>
              </tr>)}</tbody>
            </table></div>
          </details>}

        </li>;
      })}</ol>
    </section>
  </>;
}
