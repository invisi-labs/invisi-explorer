import { useState, type ReactNode } from "react";
import { fundingView } from "./funding";
import { RecognitionSummary } from "./RecognitionSummary";
import type { Key } from "./i18n";

type Props = {
  operation: Record<string, unknown>;
  t: (key: Key) => string;
  address: (value: string, kind?: "address" | "token") => ReactNode;
  amount: (value: string, token: string) => ReactNode;
};
const zero = "0x0000000000000000000000000000000000000000";
const warningKeys: Record<string, Key> = {
  "evidence-unavailable": "fundingUnavailable",
  "legacy-evidence": "fundingLegacy",
  "ambiguous-evidence": "fundingAmbiguous",
  "inventory-boundary": "fundingInventory",
  "execution-failed": "fundingFailed",
  "orphaned-operation": "fundingOrphaned",
  "execution-unknown": "fundingUnknownExecution",
  "source-not-beneficial-owner": "fundingSourceCaveat",
  "source-unresolved": "fundingSourceUnknown",
  "authorization-not-execution-proof": "fundingAuthorizationCaveat",
  "invalid-evidence": "fundingInvalid",
  "context-conflict": "fundingInvalid",
};

export function FundingPanel({ operation, t, address, amount }: Props) {
  const view = fundingView(operation);
  const [expanded, setExpanded] = useState(false);
  const transfers = expanded ? view.transfers : view.transfers.slice(0, 12);
  const asset = (token: string) =>
    token.toLowerCase() === zero ? t("fundingNative") : address(token, "token");
  return (
    <section className="funding-panel" aria-label={t("fundingTitle")}>
      <h3>{t("fundingTitle")}</h3>
      <p className="muted">{t("fundingGeneric")}</p>
      {view.warnings.length > 0 && (
        <ul className="funding-notices">
          {[
            ...new Set(
              view.warnings.map(
                (code) => warningKeys[code] ?? "fundingInvalid",
              ),
            ),
          ].map((key) => (
            <li key={key}>{t(key)}</li>
          ))}
        </ul>
      )}
      {typeof operation.unsupportedReason === "string" &&
        operation.unsupportedReason && (
          <RecognitionSummary
            data={{ reason: operation.unsupportedReason }}
            t={t}
            status={() => null}
          />
        )}
      {view.execution !== "executed" || !view.rolesConfirmed ? null : (
        <>
          <dl className="funding-roles">
            <dt>{t("fundingSources")}</dt>
            <dd>
              {view.sourceCandidates.length ? (
                view.sourceCandidates.map((source) => (
                  <div className="funding-role" key={source.address}>
                    {address(source.address, "address")}
                    {source.assets.map((value) => (
                      <div key={value.tokenAddress}>
                        {asset(value.tokenAddress)}
                      </div>
                    ))}
                  </div>
                ))
              ) : (
                <span className="muted">{t("fundingSourceUnknown")}</span>
              )}
            </dd>
            <dt>{t("fundingCorePayers")}</dt>
            <dd>
              {view.coreFunding.length
                ? view.coreFunding.map((funding, index) => (
                    <div className="funding-role" key={index}>
                      {address(funding.payer, "address")}
                      <div>
                        {asset(funding.tokenAddress)} ·{" "}
                        {amount(funding.amount, funding.tokenAddress)}
                      </div>
                    </div>
                  ))
                : t("fundingUnknown")}
            </dd>
            <dt>{t("fundingCaller")}</dt>
            <dd>
              {view.calls.length
                ? address(view.calls[view.calls.length - 1].from, "address")
                : t("fundingUnknown")}
            </dd>
            <dt>{t("fundingSubmitter")}</dt>
            <dd>
              {view.submitter
                ? address(view.submitter, "address")
                : t("fundingUnknown")}
              <p className="muted">{t("fundingSubmitterCaveat")}</p>
            </dd>
            {typeof operation.feeRecipient === "string" &&
              operation.feeRecipient !== zero &&
              typeof operation.feeAmount === "string" &&
              /^[1-9][0-9]*$/.test(operation.feeAmount) && (
                <>
                  <dt>{t("fundingFeeRecipient")}</dt>
                  <dd>{address(operation.feeRecipient, "address")}</dd>
                </>
              )}
            <dt>{t("fundingExecution")}</dt>
            <dd>
              {view.is7702Transaction ? (
                <strong>EIP-7702 · {t("fundingType")} 4</strong>
              ) : (
                t("funding7702Unknown")
              )}
              {view.transactionType && !view.is7702Transaction && (
                <p className="muted">
                  {t("fundingType")} {view.transactionType}
                </p>
              )}
            </dd>
          </dl>
          {view.transfers.length > 0 && (
            <div className="funding-flow">
              <h4>{t("fundingAssets")}</h4>
              <p className="muted">{t("fundingAssetsCaveat")}</p>
              <div className="table-wrap">
                <table className="funding-table">
                  <thead>
                    <tr>
                      <th>{t("fundingFrom")}</th>
                      <th>{t("fundingTo")}</th>
                      <th>{t("fundingAsset")}</th>
                      <th>{t("fundingAmount")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {transfers.map((transfer, index) => (
                      <tr key={index}>
                        <td data-label={t("fundingFrom")}>
                          {address(transfer.from, "address")}
                        </td>
                        <td data-label={t("fundingTo")}>
                          {address(transfer.to, "address")}
                        </td>
                        <td data-label={t("fundingAsset")}>
                          {asset(transfer.tokenAddress)}
                          {transfer.eventKind && (
                            <small>
                              {t(
                                transfer.eventKind === "wrapped-native-deposit"
                                  ? "fundingWrap"
                                  : "fundingUnwrap",
                              )}
                            </small>
                          )}
                        </td>
                        <td data-label={t("fundingAmount")}>
                          {amount(transfer.amount, transfer.tokenAddress)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              {view.transfers.length > 12 && (
                <button
                  type="button"
                  aria-expanded={expanded}
                  onClick={() => setExpanded(!expanded)}
                >
                  {t(expanded ? "fundingLess" : "fundingMore")} (
                  {view.transfers.length})
                </button>
              )}
            </div>
          )}
          {view.calls.length > 0 && (
            <details className="funding-calls">
              <summary>{t("fundingCalls")}</summary>
              <ol>
                {view.calls.map((call) => (
                  <li key={call.path}>
                    <span className="funding-call-meta">
                      {call.type} · {call.path}
                    </span>
                    <div>
                      {address(call.from, "address")}{" "}
                      <span aria-label={t("fundingTo")}>→</span>{" "}
                      {address(call.to, "address")}
                    </div>
                  </li>
                ))}
              </ol>
            </details>
          )}
          {view.authorizations.length > 0 && (
            <details className="funding-calls">
              <summary>{t("fundingAuthorizations")}</summary>
              <p className="muted">{t("fundingAuthorizationCaveat")}</p>
              <ul>
                {view.authorizations.map((authorization, index) => (
                  <li key={index}>
                    <div>
                      {t("fundingAuthority")}:{" "}
                      {authorization.authority
                        ? address(authorization.authority, "address")
                        : t("fundingUnknown")}
                    </div>
                    <div>
                      {t("fundingDelegate")}:{" "}
                      {address(authorization.address, "address")}
                    </div>
                    <small>
                      chain ID {authorization.chainId} · nonce{" "}
                      {authorization.nonce} ·{" "}
                      {t(
                        authorization.signatureValid
                          ? "fundingSignatureValid"
                          : "fundingSignatureInvalid",
                      )}
                    </small>
                  </li>
                ))}
              </ul>
            </details>
          )}
          {view.legacy && <p className="muted">{t("fundingLegacy")}</p>}
        </>
      )}
    </section>
  );
}
