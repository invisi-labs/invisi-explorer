import { TermHelp } from "./TermHelp";
import type { ReactNode } from "react";
import { fundingView, type ExecutionAuthorization } from "./funding";
import type { Key } from "./i18n";

type RecordData = Record<string, any>;
const addressValue = (value: unknown): string | null =>
  typeof value === "string" && /^0x[0-9a-f]{40}$/i.test(value) ? value.toLowerCase() : null;
const same = (a: unknown, b: unknown) => typeof a === "string" && typeof b === "string" && a.toLowerCase() === b.toLowerCase();

// Read only observations bound to the exact canonical transaction. Members may
// lack optional observations; conflicting known observations cannot be merged.
function executionSummary(operations: RecordData[]) {
  const first = operations[0];
  const invalid = { conflict: true as const, core: null, sender: null, to: null, type: null, authorizations: [] as ExecutionAuthorization[] };
  if (!first || !addressValue(first.protocolAddress)) return invalid;
  const keys = ["chainId", "protocolAddress", "txHash", "blockHash", "blockNumber"];
  if (operations.some(op => op.canonical !== true || keys.some(key => !same(op[key], first[key])))) return invalid;
  const known = { sender: new Set<string>(), to: new Set<string>(), type: new Set<string>() };
  const authorizations = new Map<string, ExecutionAuthorization>();
  for (const op of operations) {
    const observation = op.transactionObservation;
    if (observation && keys.some(key => !same(observation[key], op[key]))) continue;
    const view = fundingView(op);
    if (view.warnings.includes("context-conflict")) return invalid;
    // Indexed execution contexts and funding traces are already bound to this
    // canonical operation by the backend; retain them if the optional read fails.
    if (!observation && !view.rolesConfirmed) continue;
    const sender = addressValue(observation ? observation.transactionSender : view.submitter);
    const to = addressValue(observation ? observation.transactionTo : view.transactionTo);
    const type = observation ? observation.transactionType : view.transactionType;
    if (sender) known.sender.add(sender);
    if (to) known.to.add(to);
    if (typeof type === "string" && /^(0|[1-9][0-9]{0,2})$/.test(type) && Number(type) <= 255) known.type.add(type);
    for (const item of view.authorizations) authorizations.set(JSON.stringify(item), item);
  }
  if (Object.values(known).some(values => values.size > 1)) return invalid;
  return { conflict: false, core: addressValue(first.protocolAddress), sender: [...known.sender][0] ?? null, to: [...known.to][0] ?? null, type: [...known.type][0] ?? null, authorizations: [...authorizations.values()] };
}

export function TransactionExecutionPanel({ operations, t, address }: {
  operations: RecordData[];
  t: (key: Key) => string;
  address: (value: string, kind?: "address" | "token") => ReactNode;
}) {
  const view = executionSummary(operations);
  return <section className="card transaction-execution" aria-label={t("transactionExecutionTitle")}>
    <h2>{t("transactionExecutionTitle")}</h2>
    {view.conflict ? <p className="muted">{t("fundingInvalid")}</p> : <>
      <dl className="public-fields">
        <dt>{t("fundingSubmitter")}<TermHelp term="fundingSubmitter" label={t("fundingSubmitter")} t={t}/></dt><dd>{view.sender ? address(view.sender, "address") : t("fundingUnknown")}<p className="muted">{t("fundingSubmitterCaveat")}</p></dd>
        <dt>{t("fundingTransactionTo")}</dt><dd>{view.to ? address(view.to, "address") : t("fundingUnknown")}</dd>
        <dt>{t("transactionCore")}</dt><dd>{view.core ? address(view.core, "address") : t("fundingUnknown")}</dd>
        <dt>{t("fundingType")}<TermHelp term="fundingType" label={t("fundingType")} t={t}/></dt><dd>{view.type === "4" ? <strong>EIP-7702 · {t("fundingType")} 4</strong> : view.type !== null ? t("fundingRegularTransaction").replace("{type}", view.type) : t("fundingUnknown")}</dd>
      </dl>
      {view.authorizations.length > 0 && <details className="funding-calls">
        <summary>{t("fundingAuthorizations")}</summary><p className="muted">{t("fundingAuthorizationCaveat")}</p>
        <ul>{view.authorizations.map((item, i) => <li key={i}>
          <div>{t("fundingAuthority")}: {item.authority ? address(item.authority, "address") : t("fundingUnknown")}</div>
          <div>{t("fundingDelegate")}: {address(item.address, "address")}</div>
          <small>chain ID {item.chainId} · nonce {item.nonce} · {t(item.signatureValid ? "fundingSignatureValid" : "fundingSignatureInvalid")}</small>
        </li>)}</ul>
      </details>}
    </>}
  </section>;
}
