import { useId } from "react";
import type { Key } from "./i18n";
import { Select } from "./Select";
import { NetworkIcon } from "./NetworkIcon";
import {
  networkGroups,
  type PublicNetwork,
  type TransactionFilters as Filters,
} from "./networkDirectory";
export function TransactionFilters({
  value,
  networks,
  tokens = [],
  onChange,
  t,
}: {
  value: Filters;
  networks: PublicNetwork[];
  tokens?: Record<string, any>[];
  onChange: (next: Filters) => void;
  t: (key: Key) => string;
}) {
  const id = useId(),
    groups = networkGroups(networks),
    deployments =
      groups.find((g) => g.chainId === value.chainId)?.deployments ?? [];
  const update = (patch: Partial<Filters>) => onChange({ ...value, ...patch });
  const tokenChoices = tokens
    .filter(
      (token) => !value.chainId || String(token.chainId) === value.chainId,
    )
    .filter(
      (token, i, all) =>
        all.findIndex(
          (other) =>
            other.address?.toLowerCase() === token.address?.toLowerCase(),
        ) === i,
    );
  return (
    <section
      className="transaction-filters"
      aria-label={t("transactionFilters")}
    >
      <div className="transaction-filter-grid">
        <div>
          <label id={id + "-network"}>{t("network")}</label>
          <Select
            label={t("network")}
            value={value.chainId}
            onChange={(chainId) =>
              update({ chainId, protocolAddress: "", tokenAddress: "" })
            }
            options={[
              { value: "", label: t("allNetworks") },
              ...groups.map((g) => ({
                value: g.chainId,
                label: g.name ?? t("unknown"),
                icon: <NetworkIcon chainId={g.chainId} />,
              })),
            ]}
          />
        </div>
        <div>
          <label>{t("protocolDeployment")}</label>
          <Select
            label={t("protocolDeployment")}
            value={value.protocolAddress}
            disabled={!value.chainId}
            onChange={(protocolAddress) => update({ protocolAddress })}
            options={[
              {
                value: "",
                label: value.chainId
                  ? t("allDeployments")
                  : t("selectNetworkFirst"),
              },
              ...deployments.map((d) => ({
                value: d.protocolAddress.toLowerCase(),
                label:
                  d.protocolAddress.slice(0, 10) +
                  "…" +
                  d.protocolAddress.slice(-8),
              })),
            ]}
          />
        </div>
        <div>
          <label>{t("operationFilter")}</label>
          <Select
            label={t("operationFilter")}
            value={value.kind}
            onChange={(kind) => update({ kind })}
            options={[
              { value: "", label: t("allOperations") },
              ...(
                [
                  "shield",
                  "transact",
                  "creditDraw",
                  "createCredit",
                  "updateCredit",
                  "revokeCredit",
                  "unsupported",
                ] as const
              ).map((kind) => ({
                value: kind,
                label: t(
                  kind === "unsupported" ? "unsupportedOperation" : kind,
                ),
              })),
            ]}
          />
        </div>
        <div>
          <label>{t("token")}</label>
          <Select
            label={t("token")}
            value={value.tokenAddress}
            onChange={(tokenAddress) => update({ tokenAddress })}
            options={[
              { value: "", label: t("allTokens") },
              ...tokenChoices.map((token) => ({
                value: token.address.toLowerCase(),
                label:
                  (token.symbol ?? t("unknown")) +
                  " · " +
                  token.address.slice(0, 8) +
                  "…" +
                  token.address.slice(-4),
              })),
            ]}
          />
        </div>
      </div>
      <div className="transaction-filter-footer">
        <p className="muted">{t("operationFilterHint")}</p>
        <button
          className="plain"
          onClick={() =>
            onChange({
              chainId: "",
              protocolAddress: "",
              kind: "",
              tokenAddress: "",
            })
          }
        >
          {t("resetFilters")}
        </button>
      </div>
    </section>
  );
}
