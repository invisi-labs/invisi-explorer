import { useId, type ReactNode } from "react";
import { Link } from "react-router-dom";
import type { Key } from "./i18n";
import { HelpTip } from "./HelpTip";
import { fullTime } from "./public";
import {
  broadcasterScopes,
  type BroadcasterSelection,
} from "./broadcasterScopes";
type Row = Record<string, any>;
type Props = {
  data: Row;
  t: (key: Key) => string;
  network: (row: Row) => ReactNode;
  token: (address: string, row: Row) => ReactNode;
  address: (value: string, row: Row) => ReactNode;
  href: (row: Row) => string;
  detail?: boolean;
  selection?: BroadcasterSelection;
  onSelect?: (selection: BroadcasterSelection) => void;
};
export function broadcasterName(endpoint: unknown): string {
  try {
    return new URL(String(endpoint)).hostname;
  } catch {
    return String(endpoint ?? "—");
  }
}
function safeEndpoint(value: unknown): string | undefined {
  try {
    const url = new URL(String(value));
    if (url.protocol === "https:" && !url.username && !url.password)
      return url.href;
  } catch {}
  return;
}
export function broadcasterState(row: Row): Key {
  const observation = row.observation ?? {};
  if (!row.enabled) return "bcNotProbed";
  if (observation.stale && observation.checkedAt) return "bcStale";
  return (
    (
      {
        available: "bcOnline",
        halted: "bcPaused",
        identity_mismatch: "bcMismatch",
        invalid_endpoint: "bcInvalidEndpoint",
        unavailable: "bcUnreachable",
        not_probed: "bcNotProbed",
      } as Record<string, Key>
    )[observation.status] ?? "bcNotProbed"
  );
}
export function provenanceRequestObservation(
  provider: Row | undefined,
  now = Date.now() / 1000,
): { key: Key; checkedAt?: string } {
  const checked = Number(provider?.checkedAt);
  if (!Number.isSafeInteger(checked) || checked <= 0 || checked > now + 30)
    return { key: "bcSourceNotChecked" };
  return {
    key:
      provider?.health === "available"
        ? "bcSourceRequestSuccess"
        : provider?.health === "unavailable"
          ? "bcSourceRequestFailed"
          : "bcSourceRequestUnknown",
    checkedAt: String(checked),
  };
}
function tokenAvailable(token: Row): boolean {
  return (
    token.supported === true &&
    (!token.priceValidUntil || Date.parse(token.priceValidUntil) > Date.now())
  );
}
const time = (value: unknown) => (value ? fullTime(String(value)) : "—");
export function BroadcastersPanel({
  data,
  t,
  network,
  token,
  address,
  href,
  detail = false,
  selection = {},
  onSelect,
}: Props) {
  const deploymentId = useId();
  const scope = broadcasterScopes(data, selection);
  const selected = scope.selected;
  const rows: Row[] = detail
    ? selected
      ? [selected]
      : []
    : (data.items ?? []);
  const yesNo = (value: unknown) =>
    t(
      value === true
        ? "bcSupported"
        : value === false
          ? "bcNotEnabled"
          : "bcNotProvided",
    );
  return (
    <>
      <p className="muted">{t("bcDescription")}</p>
      {data.directoryComplete === false && (
        <p className="pagination-message" role="status">
          {t("bcDirectoryPending")}
        </p>
      )}
      {detail && selected && (
        <section className="card bc-service-overview">
          <h2>{broadcasterName(selected.endpoint)}</h2>
          <p className="muted">{t("bcScopeHint")}</p>
          <p className="bc-scope-counts">
            <span>
              {t("bcNetworkCount").replace(
                "{count}",
                String(scope.networks.length),
              )}
            </span>
            <span>
              {t("bcDeploymentCount").replace(
                "{count}",
                String(scope.rows.length),
              )}
            </span>
          </p>
          {scope.selectionMissing && (
            <p className="pagination-message" role="status">
              {t("bcDefaultMissing")}
            </p>
          )}
          <div
            className="bc-network-picker"
            role="group"
            aria-label={t("bcNetworkSelection")}
          >
            {scope.networks.map((row) => (
              <button
                key={row.chainId}
                type="button"
                aria-pressed={String(row.chainId) === String(selected.chainId)}
                onClick={() =>
                  onSelect?.({
                    chainId: String(row.chainId),
                    protocolAddress: scope.rows.find(
                      (item) => String(item.chainId) === String(row.chainId),
                    )!.protocolAddress,
                  })
                }
              >
                {network(row)}
              </button>
            ))}
          </div>
          <div className="bc-deployment-picker">
            {scope.deployments.length > 1 ? (
              <>
                <label htmlFor={deploymentId}>
                  {t("bcDeploymentSelection")}
                </label>
                <div className="bc-deployment-select">
                  <select
                    id={deploymentId}
                    value={selected.protocolAddress}
                    onChange={(event) =>
                      onSelect?.({
                        chainId: String(selected.chainId),
                        protocolAddress: event.target.value,
                      })
                    }
                  >
                    {scope.deployments.map((row) => (
                      <option
                        key={row.protocolAddress}
                        value={row.protocolAddress}
                      >
                        {row.protocolAddress}
                      </option>
                    ))}
                  </select>
                  <svg
                    aria-hidden="true"
                    width="16"
                    height="16"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.6"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <path d="m7 10 5 5 5-5" />
                  </svg>
                </div>
              </>
            ) : (
              <>
                <span className="bc-deployment-label">
                  {t("bcDeploymentSelection")}
                </span>
                <div className="bc-deployment-value">
                  {address(selected.protocolAddress, selected)}
                </div>
              </>
            )}
          </div>
        </section>
      )}
      {!detail ? (
        <section className="card">
          <div className="table-wrap">
            <table className="public-list broadcasters-table">
              <thead>
                <tr>
                  {(
                    [
                      "bcService",
                      "network",
                      "bcRegistration",
                      "bcRuntime",
                      "bcLastCheck",
                    ] as Key[]
                  ).map((key) => (
                    <th key={key}>{t(key)}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {rows.map((row, i) => {
                  const observation = row.observation ?? {};
                  const state = broadcasterState(row);
                  return (
                    <tr
                      key={`${row.chainId}:${row.protocolAddress}:${row.operator}:${i}`}
                    >
                      <td data-label={t("bcService")}>
                        <Link to={href(row)}>
                          {broadcasterName(row.endpoint)}
                        </Link>
                        <small className="muted">
                          <code title={row.operator}>
                            {String(row.operator).slice(0, 8)}…
                            {String(row.operator).slice(-6)}
                          </code>
                        </small>
                      </td>
                      <td data-label={t("network")}>{network(row)}</td>
                      <td data-label={t("bcRegistration")}>
                        <span
                          className={"badge " + (row.enabled ? "good" : "")}
                        >
                          {t(row.enabled ? "bcEnabled" : "bcDisabled")}
                        </span>
                      </td>
                      <td data-label={t("bcRuntime")}>
                        <span className="bc-runtime">
                          <span
                            className={
                              "badge " + (state === "bcOnline" ? "good" : "")
                            }
                          >
                            {t(state)}
                          </span>
                          {observation.checkedAt > 0 && (
                            <span className="muted">
                              · {observation.responseMillis} ms
                            </span>
                          )}
                        </span>
                      </td>
                      <td data-label={t("bcLastCheck")}>
                        {time(observation.checkedAt)}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
          {!rows.length && (
            <p className="muted">
              {t(
                data.directoryComplete
                  ? "bcNoRegistered"
                  : "bcDirectoryPending",
              )}
            </p>
          )}
        </section>
      ) : (
        rows.map((row, i) => {
          const observation = row.observation ?? {};
          const service = observation.service;
          const tokens: Row[] = observation.tokens ?? [];
          const provider = service?.provenance;
          const sourceRequest = provenanceRequestObservation(provider);
          const source = (data.sources ?? []).find(
            (source: Row) =>
              String(source.chainId) === String(row.chainId) &&
              source.protocolAddress?.toLowerCase() ===
                row.protocolAddress?.toLowerCase(),
          );
          const endpoint = safeEndpoint(row.endpoint);
          return (
            <div
              key={`${row.chainId}:${row.protocolAddress}:${i}`}
              className="bc-detail"
            >
              <section className="card">
                <h2>{t("bcSelectedScope")}</h2>
                <p className="bc-status-line">
                  {network(row)}
                  <span className={"badge " + (row.enabled ? "good" : "")}>
                    {t(row.enabled ? "bcEnabled" : "bcDisabled")}
                  </span>
                  <span
                    className={
                      "badge " +
                      (broadcasterState(row) === "bcOnline" ? "good" : "")
                    }
                  >
                    {t(broadcasterState(row))}
                  </span>
                </p>
                <div className="bc-active-deployment">
                  <span>{t("protocol")}</span>
                  {address(row.protocolAddress, row)}
                </div>
                <h3>{t("bcExplorerObservation")}</h3>
                <dl className="public-fields">
                  <dt>{t("bcLastCheck")}</dt>
                  <dd>{time(observation.checkedAt)}</dd>
                  <dt>{t("bcLatency")}</dt>
                  <dd>
                    {observation.checkedAt
                      ? `${observation.responseMillis} ms`
                      : "—"}
                  </dd>
                  <dt>{t("bcDeploymentMatch")}</dt>
                  <dd>
                    {t(
                      observation.identityStatus === "matched"
                        ? "bcMatched"
                        : observation.identityStatus === "mismatch"
                          ? "bcMismatch"
                          : "unknown",
                    )}
                  </dd>
                </dl>
                {observation.stale && (
                  <p className="muted">{t("bcStaleNotice")}</p>
                )}
              </section>
              <section className="card">
                <h2>{t("bcCapabilities")}</h2>
                <p className="muted">{t("bcServiceDeclared")}</p>
                <dl className="public-fields">
                  <dt>{t("bcAccepting")}</dt>
                  <dd>
                    {t(
                      service?.submissionHalted === false
                        ? "bcAcceptingNow"
                        : service?.submissionHalted === true
                          ? "bcPaused"
                          : "bcNotProvided",
                    )}
                  </dd>
                  <dt>{t("bcScheduling")}</dt>
                  <dd>
                    {yesNo(service?.scheduling?.enabled)}
                    {service?.scheduling?.enabled && (
                      <small className="bc-detail-hint">
                        {/^[1-9][0-9]*$/.test(
                          String(service.scheduling.maxDelaySeconds),
                        )
                          ? Number(service.scheduling.maxDelaySeconds) %
                              86400 ===
                            0
                            ? t("bcMaxDelayDays").replace(
                                "{days}",
                                String(
                                  Number(service.scheduling.maxDelaySeconds) /
                                    86400,
                                ),
                              )
                            : t("bcMaxDelay").replace(
                                "{hours}",
                                String(
                                  Number(service.scheduling.maxDelaySeconds) /
                                    3600,
                                ),
                              )
                          : t("bcNotProvided")}
                      </small>
                    )}
                  </dd>
                  <dt>{t("bcRouter")}</dt>
                  <dd>
                    {service?.paymentRouters
                      ? t("bcRouterCount").replace(
                          "{count}",
                          String(service.paymentRouters.length),
                        )
                      : t("unknown")}
                  </dd>
                  <dt>{t("bcAggregation")}</dt>
                  <dd>{yesNo(provider?.aggregateSupported)}</dd>
                  <dt>{t("bcFeeLock")}</dt>
                  <dd>{yesNo(service?.feeLock?.core)}</dd>
                  <dt>
                    {t("bcScreening")}
                    <HelpTip
                      label={t("bcScreening") + " · ?"}
                      text={t("bcSourceObservationHint")}
                    />
                  </dt>
                  <dd>
                    {yesNo(provider?.enabled)}
                    {provider?.enabled && (
                      <small className="bc-detail-hint">
                        {t(sourceRequest.key)}
                        {sourceRequest.checkedAt && (
                          <> · {time(sourceRequest.checkedAt)}</>
                        )}
                      </small>
                    )}
                  </dd>
                </dl>
              </section>
              <section className="card">
                <h2>{t("bcTokenPolicy")}</h2>
                <p className="muted">{t("bcTokenPolicyHint")}</p>
                {observation.tokensStatus === "complete" ? (
                  <div className="table-wrap">
                    <table className="funding-table">
                      <thead>
                        <tr>
                          <th>{t("token")}</th>
                          <th>{t("bcAvailability")}</th>
                          <th>{t("bcPriceExpiry")}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {tokens.map((item) => (
                          <tr key={item.tokenAddress}>
                            <td data-label={t("token")}>
                              {token(item.tokenAddress, row)}
                            </td>
                            <td data-label={t("bcAvailability")}>
                              {t(
                                observation.stale
                                  ? "bcStale"
                                  : item.status === "paused"
                                    ? "bcPaused"
                                    : tokenAvailable(item)
                                      ? "bcAvailable"
                                      : "bcPricingUnavailable",
                              )}
                            </td>
                            <td data-label={t("bcPriceExpiry")}>
                              {item.priceValidUntil
                                ? fullTime(item.priceValidUntil)
                                : "—"}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                    {!tokens.length && <p>{t("bcNoConfiguredTokens")}</p>}
                  </div>
                ) : (
                  <p className="muted">
                    {t(
                      observation.tokensStatus === "not_provided"
                        ? "bcLegacyTokens"
                        : "bcQueryUnavailable",
                    )}
                  </p>
                )}
              </section>
              <details className="card bc-technical">
                <summary>{t("bcTechnical")}</summary>
                <p className="muted">{t("bcRegistrySource")}</p>
                <dl className="public-fields">
                  <dt>{t("bcOperator")}</dt>
                  <dd>{address(row.operator, row)}</dd>
                  <dt>{t("bcEndpoint")}</dt>
                  <dd>
                    {endpoint ? (
                      <a
                        href={endpoint}
                        target="_blank"
                        rel="noopener noreferrer"
                      >
                        {row.endpoint}
                      </a>
                    ) : (
                      String(row.endpoint ?? "—")
                    )}
                  </dd>
                  <dt>{t("bcQuoteSigner")}</dt>
                  <dd>{address(row.quoteSigner, row)}</dd>
                  <dt>{t("bcFeeRecipient")}</dt>
                  <dd>{address(row.feeRecipient, row)}</dd>
                  <dt>{t("bcRegisteredAt")}</dt>
                  <dd>{time(row.updatedAt?.timestamp)}</dd>
                  <dt>{t("bcRegistry")}</dt>
                  <dd>
                    {source?.registryAddress
                      ? address(source.registryAddress, row)
                      : "—"}
                  </dd>
                  <dt>{t("bcRegistryCheck")}</dt>
                  <dd>
                    {time(source?.checkedAt)}
                    {source?.stale && ` · ${t("bcStale")}`}
                  </dd>
                  <dt>{t("bcRegistrationBlock")}</dt>
                  <dd>{source?.anchor?.blockNumber ?? "—"}</dd>
                  <dt>{t("protocol")}</dt>
                  <dd>{address(row.protocolAddress, row)}</dd>
                  <dt>{t("bcSender")}</dt>
                  <dd>
                    {service?.sender ? address(service.sender, row) : "—"}
                  </dd>
                  <dt>{t("bcApiVersion")}</dt>
                  <dd>{service?.apiVersion ?? "—"}</dd>
                  <dt>{t("bcMetadataURI")}</dt>
                  <dd>{row.metadataURI || "—"}</dd>
                </dl>
                {service?.paymentRouters?.map((router: Row, index: number) => (
                  <p key={index}>
                    {t("bcRouter")} {address(router.router, row)}
                  </p>
                ))}
              </details>
            </div>
          );
        })
      )}
      {detail && !selected && (
        <section className="card">
          <p className="muted">{t("bcNoRegistered")}</p>
        </section>
      )}
    </>
  );
}
