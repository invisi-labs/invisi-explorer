import type { ReactNode } from "react";
import type { Key } from "./i18n";

const explanations: Record<string, Key> = {
  FUNDING_EVIDENCE_INCOMPLETE: "reasonFundingIncomplete",
  FUNDING_EVIDENCE_UNAVAILABLE: "reasonTraceUnavailable",
  PROOF_REQUIRED: "reasonProofRequired",
  FUNDING_REJECTED: "reasonFundingRejected",
  SOURCE_OPERATION_PENDING: "reasonDependencyPending",
  PROVIDER_UNAVAILABLE: "providerUnavailable",
  CHAIN_REORGED: "reorg",
  ROUTER_PREFLIGHT_REQUIRES_OUTER_TRANSACTION: "reasonPreflightOuter",
};
export function recognitionReasons(data: Record<string, unknown>): string[] {
  return [
    ...new Set(
      [
        data.reason,
        data.reasonCode,
        ...(Array.isArray(data.reasons) ? data.reasons : []),
      ].filter(
        (reason): reason is string =>
          typeof reason === "string" && reason.trim().length > 0,
      ),
    ),
  ];
}
export function RecognitionSummary({
  data,
  t,
  status,
}: {
  data: Record<string, unknown>;
  t: (key: Key) => string;
  status: (value: unknown) => ReactNode;
}) {
  return (
    <div className="recognition-summary">
      {status(data.status ?? data.recognitionState ?? "unknown")}
      {recognitionReasons(data).map((reason) => (
        <p className="recognition-reason" key={reason}>
          {explanations[reason] && <span>{t(explanations[reason])}</span>}
          <code>{reason}</code>
        </p>
      ))}
    </div>
  );
}
