import type { ReactNode } from "react";
import { Link } from "react-router-dom";
import type { Key } from "./i18n";
import { fullTime, finalityKey } from "./public";
import {
  type NetworkGroup,
  type PublicNetwork,
  transactionPath,
} from "./networkDirectory";
export function DeploymentStatus({
  status,
  t,
}: {
  status?: string;
  t: (key: Key) => string;
}) {
  return (
    <span
      className={
        "badge " +
        (status === "ready" ? "good" : status === "unavailable" ? "bad" : "")
      }
    >
      {t(
        status === "ready"
          ? "syncReady"
          : status === "unavailable"
            ? "syncUnavailable"
            : "syncNotChecked",
      )}
    </span>
  );
}
export function NetworkDirectory({
  groups,
  t,
  network,
  address,
  tokens = [],
}: {
  groups: NetworkGroup[];
  t: (key: Key) => string;
  network: (row: PublicNetwork) => ReactNode;
  address: (value: string, row: PublicNetwork) => ReactNode;
  tokens?: Record<string, any>[];
}) {
  if (!groups.length)
    return (
      <section className="card">
        <p>{t("empty")}</p>
      </section>
    );
  return (
    <div className="network-directory">
      {groups.map((group) => (
        <section
          className="card network-group"
          key={group.chainId}
          id={"network-" + group.chainId}
        >
          <div className="section-head">
            <div>
              <h2>{network(group.deployments[0])}</h2>
              <p className="muted">
                {t("deploymentCount").replace(
                  "{count}",
                  String(group.deployments.length),
                )}
                {group.environment && (
                  <span className="network-environment">
                    {group.environment === "testnet"
                      ? t("testNetwork")
                      : group.environment === "mainnet"
                        ? t("mainNetwork")
                        : group.environment}
                  </span>
                )}
              </p>
            </div>
            <Link to={transactionPath({ chainId: group.chainId })}>
              {t("viewTransactions")}
            </Link>
          </div>
          {group.deployments.map((deployment) => {
            const observation = deployment.observation;
            const finalized = observation?.finality?.finalized;
            return (
              <article
                className="deployment-panel"
                key={deployment.protocolAddress}
              >
                <div className="section-head">
                  <h3>{t("protocolContract")}</h3>
                  <DeploymentStatus status={observation?.status} t={t} />
                </div>
                <div className="deployment-address">
                  {address(deployment.protocolAddress, deployment)}
                </div>
                <dl className="deployment-metrics">
                  <div>
                    <dt>{t("indexed")}</dt>
                    <dd>{observation?.indexedHead?.blockNumber ?? "—"}</dd>
                  </div>
                  <div>
                    <dt>{t("chainHead")}</dt>
                    <dd>{observation?.chainHead?.blockNumber ?? "—"}</dd>
                  </div>
                  <div>
                    <dt>{t("confirmed")}</dt>
                    <dd>
                      {finalized?.status === "available"
                        ? (finalized.block?.blockNumber ?? t("unknown"))
                        : finalized
                          ? t(finalityKey(finalized.status))
                          : "—"}
                    </dd>
                  </div>
                </dl>
                <div className="deployment-footer">
                  <small className="muted">
                    {t("backendUpdated")}{" "}
                    {observation?.updatedAt && observation.updatedAt !== "0"
                      ? fullTime(observation.updatedAt)
                      : "—"}
                  </small>
                  <Link to={transactionPath(deployment)}>
                    {t("viewTransactions")}
                  </Link>
                </div>
                {(deployment.startBlock || deployment.codeHash) && (
                  <details className="deployment-configuration">
                    <summary>{t("contractDetails")}</summary>
                    <dl>
                      {deployment.startBlock && (
                        <>
                          <dt>{t("indexStartBlock")}</dt>
                          <dd>{deployment.startBlock}</dd>
                        </>
                      )}
                      {deployment.codeHash && (
                        <>
                          <dt>{t("configuredCodeHash")}</dt>
                          <dd>
                            <code>{deployment.codeHash}</code>
                          </dd>
                        </>
                      )}
                    </dl>
                    <p className="muted">{t("contractConfigHint")}</p>
                  </details>
                )}
              </article>
            );
          })}
          {tokens.some((token) => String(token.chainId) === group.chainId) && (
            <details className="network-token-contracts">
              <summary>{t("knownTokenContracts")}</summary>
              <div className="network-token-list">
                {tokens
                  .filter((token) => String(token.chainId) === group.chainId)
                  .map((token) => (
                    <div key={token.address}>
                      <strong>{token.symbol ?? t("unknown")}</strong>
                      {address(token.address, group.deployments[0])}
                    </div>
                  ))}
              </div>
            </details>
          )}
        </section>
      ))}
    </div>
  );
}
export function HomeNetworks({
  groups,
  t,
  network,
  address,
  loading = false,
}: {
  loading?: boolean;
  groups: NetworkGroup[];
  t: (key: Key) => string;
  network: (row: PublicNetwork) => ReactNode;
  address: (value: string, row: PublicNetwork) => ReactNode;
}) {
  return (
    <section className="card home-networks">
      <div className="section-head">
        <h2>{t("supportedNetworks")}</h2>
        <Link to="/networks">{t("viewAll")}</Link>
      </div>
      {groups.length ? (
        <div className="home-network-list">
          {groups.map((group) => {
            const href =
              "/networks?chainId=" + encodeURIComponent(group.chainId);
            const abnormal = group.deployments.some(
              (deployment) => deployment.observation?.status === "unavailable",
            );
            return (
              <div className="home-network-row" key={group.chainId}>
                <div className="home-network-heading">
                  <Link to={href}>{network(group.deployments[0])}</Link>
                  {group.environment && (
                    <span className="network-environment">
                      {group.environment === "testnet"
                        ? t("testNetwork")
                        : group.environment === "mainnet"
                          ? t("mainNetwork")
                          : group.environment}
                    </span>
                  )}
                </div>
                <div className="home-contracts">
                  <span className="muted home-contract-label">
                    {t("protocolContract")}
                  </span>
                  {group.deployments.slice(0, 2).map((deployment) => (
                    <div
                      className="home-contract-reference"
                      key={deployment.protocolAddress}
                    >
                      {address(deployment.protocolAddress, deployment)}
                    </div>
                  ))}
                </div>
                {abnormal && (
                  <p className="home-network-warning" role="status">
                    {t("homeNetworkSyncIssue")}
                  </p>
                )}
                <div className="home-network-actions">
                  <Link to={transactionPath({ chainId: group.chainId })}>
                    {t("viewTransactions")}
                  </Link>
                  <Link to={href}>{t("viewContracts")}</Link>
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <p className="muted">{t(loading ? "loading" : "empty")}</p>
      )}
    </section>
  );
}
