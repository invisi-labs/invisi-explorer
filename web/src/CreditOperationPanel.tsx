import { type ReactNode } from "react";
import { Link } from "react-router-dom";
import type { Key } from "./i18n";
import { creditGrantRefs, creditSlotId } from "./credit";
import { fundingView } from "./funding";
import { CopyButton } from "./CopyButton";

type RecordData = Record<string, any>;
type Props = {
  operation: RecordData;
  hideTransaction?: boolean;
  t: (key: Key) => string;
  address: (value: string, kind?: "address" | "token") => ReactNode;
  amount: (value: string, token: string) => ReactNode;
  href: (type: string, id: string) => string;
};
const descriptions: Record<string, Key> = {
  createCredit: "creditCreateDescription", updateCredit: "creditUpdateDescription",
  revokeCredit: "creditRevokeDescription", creditDraw: "creditDrawDescription",
};
export function CreditOperationPanel({operation: op, t, address, amount, href, hideTransaction = false}: Props) {
  const view = fundingView(op);
  const settled = op.success === true && op.canonical === true && !op.unsupportedReason;
  const refs = creditGrantRefs(op);
  const initial = op.kind === "createCredit" && (op.noteOutputs?.length ?? 0) > 0;
  const payment = op.kind === "creditDraw" || initial;
  const outputs: RecordData[] = settled && Array.isArray(op.creditOutputs) ? op.creditOutputs : [];
  const nullifiers: string[] = settled && Array.isArray(op.creditNullifiers) ? op.creditNullifiers : [];
  return <section className="credit-operation" aria-label={t(op.kind as Key)}>
    <h3>{t(op.kind as Key)}</h3>
    <p className="muted">{t(descriptions[op.kind])}</p>
    {!settled && <p className="muted">{t("creditUnsettled")}</p>}
    <h4>{t("creditSlots")}</h4>
    {refs.length ? <div className="credit-slots">{refs.map((ref, i) => {
      const id = creditSlotId(op, ref);
      return <div className="credit-slot" key={id ?? i}>
        <dl className="public-fields">
          <dt>{t("grantOwner")}</dt><dd>{ref.payer ? address(ref.payer) : t("fundingUnknown")}</dd>
          <dt>{t("grantIdLabel")}</dt>
          <dd className="credit-slot-id">
            <code>{ref.grantId}</code>
            {id && <Link className="credit-slot-link" to={href("grants", id)}>
              {t("creditViewGrant")}
              <svg aria-hidden="true" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round"><path d="m9 5 7 7-7 7" /></svg>
            </Link>}
          </dd>
        </dl>
      </div>;
    })}</div> : <p className="muted">{t(settled ? "creditIndexPending" : "fundingUnknown")}</p>}
    {["createCredit", "updateCredit"].includes(op.kind) && <p className="muted">{t("creditPrivatePolicy")}</p>}
    {settled && payment && <>
      <h4>{t(initial ? "creditInitialPayment" : "creditDrawFunding")}</h4>
      <p className="muted">{t("creditPayerAmountCaveat")}</p>
      {view.coreFunding.map((payer, i) => <dl className="public-fields credit-payer" key={i}>
        <dt>{t("fundingPaymentAddress")}</dt><dd>{address(payer.payer)}</dd>
        <dt>{t("fundingPaymentToken")}</dt><dd>{address(payer.tokenAddress,"token")}</dd>
        <dt>{t("creditPayerTotal")}</dt><dd>{amount(payer.amount,payer.tokenAddress)}</dd>
      </dl>)}
    </>}
    {settled && typeof op.feeAmount === "string" && <dl className="public-fields">
      <dt>{t("fee")}</dt><dd>{amount(op.feeAmount,op.tokenAddress)}</dd>
      {op.feeRecipient && !/^0x0{40}$/.test(op.feeRecipient) && <><dt>{t("fundingFeeRecipient")}</dt><dd>{address(op.feeRecipient)}</dd></>}
    </dl>}
    {!hideTransaction && view.rolesConfirmed && <dl className="public-fields">
      {view.submitter && <><dt>{t("fundingSubmitter")}</dt><dd>{address(view.submitter)}</dd></>}
      {view.transactionTo && <><dt>{t("fundingTransactionTo")}</dt><dd>{address(view.transactionTo)}</dd></>}
    </dl>}
    {nullifiers.length > 0 && <>
      <h4>{t("creditConsumedStates")}</h4>
      {nullifiers.map(n => <p key={n}><Link to={href("nullifiers", n) + "&namespace=credit"}><code>{n}</code></Link></p>)}
    </>}
    {outputs.length > 0 && <>
      <h4>{t("creditNewStates")}</h4>
      <div className="table-wrap"><table className="credit-outputs-table"><thead><tr>
        <th>{t("creditCommitmentLabel")}</th><th>{t("treeIdLabel")}</th><th>{t("leafIndexLabel")}</th>
      </tr></thead><tbody>{outputs.map((output,i)=><tr key={output.creditCommitment ?? i}>
        <td data-label={t("creditCommitmentLabel")}><span className="credit-state-reference"><Link to={href("credit-states",output.creditCommitment)}><code title={output.creditCommitment}>{output.creditCommitment}</code></Link><CopyButton value={output.creditCommitment} t={t}/></span></td>
        <td data-label={t("treeIdLabel")}>{output.treeId}</td><td data-label={t("leafIndexLabel")}>{output.leafIndex}</td>
      </tr>)}</tbody></table></div>
    </>}
  </section>;
}
