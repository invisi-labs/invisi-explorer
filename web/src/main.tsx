import React, { createContext, useContext, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  BrowserRouter,
  Link,
  Navigate,
  useLocation,
  useNavigate,
} from "react-router-dom";
import { dictionaries, initialLanguage, type Language, type Key } from "./i18n";
import {
  request,
  fullTime,
  age,
  validateSearch,
  objectIdentity,
  statusKey,
  recognitionQueryStatus,
  showsSourceRecognition,
  collectionFor,
  type ApiError,
} from "./public";
import "./style.css";
import { readMetadataCache, writeMetadataCache } from "./metadataCache";
import { AdvancedTools } from "./AdvancedTools";
import { CopyButton } from "./CopyButton";
import { AmountDisplay } from "./AmountDisplay";
import { AddressReference } from "./AddressReference";
import { useCursorPagination } from "./useCursorPagination";
import { CreditOperationPanel } from "./CreditOperationPanel";
import { GrantHistory } from "./GrantHistory";
import { grantStatusKey, compactIdentifier, isCreditOperation } from "./credit";
import { FundingPanel } from "./FundingPanel";
import { SourceRecognition } from "./SourceRecognition";
import { NoteGenerationLabel, TransactionOperationLabel, TransactionAmounts } from "./TransactionPresentation";
import { NetworkIcon } from "./NetworkIcon";
import { DiagnosticsPanel, type DiagnosticsResponse } from "./DiagnosticsPanel";
type RecordData = Record<string, any>;
type Page = {
  items: RecordData[];
  total: string | number;
  nextCursor?: string;
  snapshot?: RecordData;
};
const Context = createContext({
  t: (key: Key): string => key,
  metadata: {} as RecordData,
  metadataPending: true,
});
const routes: Key[] = [
  "transactions",
  "notes",
  "credit-states",
  "grants",
  "nullifiers",
  "trees",
  "roots",
  "diagnostics",
  "broadcasters",
];
function useData<T>(path: string, refreshMs = 0) {
  const [state, set] = useState<{
    data?: T;
    error?: ApiError;
    loading: boolean;
    fetchedAt?: number;
  }>(()=>{
    if (path === "/metadata") {
      try { const data=readMetadataCache(sessionStorage) as T | undefined; if(data) return {data,loading:true}; } catch {}
    }
    return {loading:true};
  });
  const [retry, bump] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    set(previous => refreshMs && previous.data ? { ...previous, error: undefined } : { loading: true });
    request<T>(path, controller.signal)
      .then((data) => {
        if (path === "/metadata") { try { writeMetadataCache(sessionStorage,data as RecordData); } catch {} }
        set({ data, loading: false, fetchedAt: Date.now() });
      })
      .catch((error) => {
        if (!controller.signal.aborted) set(previous => refreshMs && previous.data ? { ...previous, error, loading: false } : { error, loading: false });
      });
    return () => controller.abort();
  }, [path, retry, refreshMs]);
  useEffect(() => {
    if (!refreshMs) return;
    const timer = setInterval(() => bump(x => x + 1), refreshMs);
    return () => clearInterval(timer);
  }, [refreshMs]);
  useEffect(()=>{
    if(path !== "/metadata" || !state.error) return;
    const timer=setTimeout(()=>bump(x=>x+1),3000);
    return ()=>clearTimeout(timer);
  },[path,state.error]);
  return { ...state, retry: () => bump((x) => x + 1) };
}
function State({
  loading,
  error,
  retry,
  empty,
}: {
  loading?: boolean;
  error?: ApiError;
  retry?: () => void;
  empty?: boolean;
}) {
  const { t } = useContext(Context);
  let key: Key = loading
    ? "loading"
    : empty
      ? "empty"
      : error?.status === 404
        ? "notIndexed"
        : ["REORG", "CHAIN_REORGED"].includes(error?.code ?? "")
          ? "reorg"
          : error?.code === "CONFLICT"
            ? "paginationChanging"
          : error?.code === "PROVIDER_UNAVAILABLE"
            ? "providerUnavailable"
            : "unavailable";
  return (
    <div className="state" role={error ? "alert" : "status"}>
      <p>{t(key)}</p>
      {error && (
        <>
          <code>{error.code}</code>{" "}
          <button onClick={retry}>{t("retry")}</button>
        </>
      )}
    </div>
  );
}
function Copy({ value }: { value: string }) {
  const { t } = useContext(Context);
  return <CopyButton value={value} t={t}/>;
}
function Network({ data }: { data: RecordData }) {
  const { metadata, t } = useContext(Context);
  const n = metadata.networks?.find(
    (n: RecordData) => String(n.chainId) === String(data.chainId),
  );
  return (
    <span className="network">
      <NetworkIcon chainId={data.chainId} />
      {n?.name ?? t("unknown")}
    </span>
  );
}
function Address({
  value,
  kind,
  data,
  internalHref,
  compact = false,
  displayLabel,
}: {
  value: string;
  kind?: "tx" | "address" | "token";
  data?: RecordData;
  internalHref?: string;
  compact?: boolean;
  displayLabel?: string;
}) {
  const { metadata, t } = useContext(Context);
  const n = metadata.networks?.find(
    (n: RecordData) => String(n.chainId) === String(data?.chainId),
  );
  const base =
    n?.explorerBaseUrl ??
    n?.explorerUrl ??
    n?.explorerTxUrl?.replace(/\/tx\/?$/, "");
  return <AddressReference value={value} kind={kind} explorerUrl={base} internalHref={internalHref} compact={compact} displayLabel={displayLabel} t={t}/>;
}

function GrantToken({data}: {data: RecordData}) {
 const {metadata,t} = useContext(Context);
 const token = metadata.tokens?.find((r: RecordData)=>String(r.chainId)===String(data.chainId) && r.address?.toLowerCase()===data.tokenAddress?.toLowerCase());
 if (data.tokenAddress?.toLowerCase() === "0x0000000000000000000000000000000000000000") return <span className="muted" title={data.tokenAddress}>-</span>;
 return <Address value={data.tokenAddress} kind="token" data={data} compact displayLabel={token?.symbol}/>;
}

function Status({ value }: { value: unknown }) {
  const { t } = useContext(Context);
  const raw = String(value ?? "UNKNOWN");
  const key = statusKey(value);
  if (key === "notApplicable") return <span className="muted">—</span>;
  return (
    <span
      className={
        "badge " +
        (key === "recognized" || key === "success"
          ? "good"
          : key === "rejected" ||
              key === "verificationFailed" ||
              key === "failed"
            ? "bad"
            : "")
      }
    >
      {key !== "unknown" ? t(key) : `${t("unknown")} (${raw})`}
    </span>
  );
}
function scope(data: RecordData) {
  return data.chainId && data.protocolAddress
    ? "?chainId=" +
        encodeURIComponent(data.chainId) +
        "&protocolAddress=" +
        encodeURIComponent(data.protocolAddress)
    : "";
}
function objectHref(
  type: string,
  r: RecordData,
  id = objectIdentity(collectionFor(type), r),
) {
  type = collectionFor(type);
  return (
    "/" +
    type +
    "/" +
    encodeURIComponent(id) +
    scope(r) +
    (type === "nullifiers" && r.domain
      ? "&namespace=" + encodeURIComponent(r.domain)
      : "")
  );
}
function RecognitionBadges({ data }: { data: RecordData }) {
  const {t}=useContext(Context);
  const rows = Array.isArray(data.recognition)
    ? data.recognition
    : data.recognition
      ? [data.recognition]
      : [];
  return (
    <span className="recognition-badges">
      {rows.length ? (
        rows.map((row: RecordData, i: number) => (
          <span
            className="recognition-scope"
            key={i}
          >
            {Number(data.operationCount)>1 && row.operationIndex!==undefined && <small>{t("operationNumber").replace("{number}",String(BigInt(row.operationIndex)+1n))}</small>}
            <Status value={recognitionQueryStatus(row)} />
          </span>
        ))
      ) : (
        <Status value={data.recognitionStatus ?? "pending_source"} />
      )}
    </span>
  );
}

function Amount({ value, data, compact = false }: { value: unknown; data: RecordData; compact?: boolean }) {
  const { metadata, metadataPending, t } = useContext(Context);
  if (metadataPending) return <span className="muted" role="status">{t("loading")}</span>;
  const token = metadata.tokens?.find(
    (r: RecordData) =>
      String(r.chainId) === String(data.chainId) &&
      r.address?.toLowerCase() === data.tokenAddress?.toLowerCase(),
  );
  const address = String(data.tokenAddress ?? "");
  const symbol =
    token?.symbol ??
    (address.toLowerCase() === "0x0000000000000000000000000000000000000000"
      ? t("fundingNative")
      : address ? address.slice(0, 6) + "…" + address.slice(-4) : "");
  return <AmountDisplay value={value} decimals={token?.decimals} symbol={symbol} address={address} compact={compact} t={t}/>;
}
function principal(r: RecordData) {
  if (r.publicDepositAmount !== undefined) return String(r.publicDepositAmount);
  if (!Array.isArray(r.funding) || !r.funding.length) return undefined;
  if (
    new Set(
      r.funding.map(
        (f: RecordData) =>
          f.tokenAddress?.toLowerCase() ?? r.tokenAddress?.toLowerCase(),
      ),
    ).size > 1
  )
    return undefined;
  try {
    return r.funding
      .reduce((sum: bigint, f: RecordData) => sum + BigInt(f.amount), 0n)
      .toString();
  } catch {
    return undefined;
  }
}
function Listing({
  type = "transactions",
  compact = false,
}: {
  type?: string;
  compact?: boolean;
}) {
  const { t } = useContext(Context);
  const nav = useNavigate();
  const [full, setFull] = useState(false);
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    const id = setInterval(() => setNow(Date.now()), 60000);
    return () => clearInterval(id);
  }, []);
  const result = useCursorPagination<Page>("/" + type);
  const items = result.data?.items ?? [];
  const identityLabel = t(type === "notes" ? "noteCommitmentLabel" : type === "credit-states" ? "creditCommitmentLabel" : type === "transactions" ? "transactionHashLabel" : type === "grants" ? "grantIdLabel" : "hash");
  return (
    <section className="card" aria-busy={result.loading}>
      <div className="section-head">
        <h2>{t(compact ? "recentActivity" : (type as Key))}</h2>
        <button
          className="plain mobile-time"
          onClick={() => setFull((x) => !x)}
        >
          {t(full ? "time" : "age")} ⇄
        </button>
        {result.data && (
          <small>
            {t("refreshed")} {fullTime(result.fetchedAt)}
          </small>
        )}
      </div>
      {result.notice && <p className="pagination-message" role="status">{t(result.notice === "updated" ? "paginationUpdated" : "paginationRestarted")}</p>}
      {result.error && result.data && <div className="pagination-message" role="alert">
        <span>{t(result.error.code === "CONFLICT" ? "paginationChanging" : "unavailable")}</span>
        <button className="plain" onClick={result.retry}>{t("retry")}</button>
      </div>}
      {!result.data && (result.loading || result.error) ? (
        <State {...result} />
      ) : !items.length ? (
        <State empty />
      ) : (
        <div className="table-wrap">
          <table className={type === "grants" ? "public-list grants-table" : "public-list"}>
            <thead>
              <tr>
                <th>{identityLabel}</th>
                <th>{t("network")}</th>
                {type === "transactions" && (
                  <>
                    <th>{t("transactionTypeLabel")}</th>
                    <th>{t("execution")}</th>
                    <th>{t("amount")}</th>
                    <th>{t("recognition")}</th>
                  </>
                )}
                {type === "grants" && <><th>{t("token")}</th><th>{t("creditGrantStatus")}</th><th>{t("grantHistoryCount")}</th></>}
                {type === "notes" && <><th>{t("noteGenerationMethod")}</th><th title={t("notePublicAmountHint")}>{t("notePublicAmount")}</th><th>{t("recognition")}</th></>}
                <th>
                  <button className="plain" onClick={() => setFull((x) => !x)}>
                    {t(full ? "time" : "age")} ⇄
                  </button>
                </th>
              </tr>
            </thead>
            <tbody>
              {items.map((r, i) => {
                const id = objectIdentity(type, r);
                const href = objectHref(type, r);
                return (
                  <tr
                    key={id + i}
                    tabIndex={id ? 0 : undefined}
                    role={id ? "link" : undefined}
                    aria-label={id}
                    onKeyDown={(e) => {
                      if (e.key === "Enter" && e.target === e.currentTarget && id) nav(href);
                    }}
                    onClick={() => {
                      if (id && !window.getSelection()?.toString()) nav(href);
                    }}
                  >
                    <td data-label={identityLabel}>
                      <div className={type === "grants" ? "grant-list-identifier" : undefined}>
                      <Link to={href} onClick={(e) => e.stopPropagation()}>
                        <code title={type === "grants" ? r.grantId : id}>
                          {type === "grants" ? compactIdentifier(String(r.grantId)) : id.length > 22
                            ? id.slice(0, 12) + "…" + id.slice(-8)
                            : id || "—"}
                        </code>
                      </Link>
                      {type === "grants" && <span onClick={event=>event.stopPropagation()}><CopyButton value={String(r.grantId)} t={t}/></span>}
                      </div>
                      {type === "grants" && <div className="grant-list-reference" onClick={event=>event.stopPropagation()}>
                        <Address value={r.payer} kind="address" data={r} compact/>
                      </div>}
                    </td>
                    <td data-label={t("network")}>
                      <Network data={r} />
                    </td>
                    {type === "transactions" && (
                      <>
                        <td data-label={t("transactionTypeLabel")}>
                          <TransactionOperationLabel data={r} t={t}/>
                        </td>
                        <td data-label={t("execution")}>
                          <Status value={r.executionStatus ?? r.success} />
                        </td>
                        <td className="amount-cell" data-label={t("amount")}>
                          <TransactionAmounts data={r}
                            amount={(value,token)=><Amount value={value} data={{...r,tokenAddress:token}} compact/>}
                            fallback={principal(r)!==undefined?<Amount value={principal(r)} data={r} compact/>:"—"}/>
                        </td>
                        <td
                          className="recognition-cell"
                          data-label={t("recognition")}
                        >
                          <RecognitionBadges data={r} />
                        </td>
                      </>
                    )}
                    {type === "grants" && <>
                      <td data-label={t("token")} className="grant-list-token"><div onClick={event=>event.stopPropagation()}><GrantToken data={r}/></div></td>
                      <td data-label={t("creditGrantStatus")}><span className={"badge " + (r.authorizationStatus === "active" ? "good" : "")}>{t(grantStatusKey(r))}</span></td>

                      <td data-label={t("grantHistoryCount")}><Link to={href} onClick={e=>e.stopPropagation()}>{r.historyCount === undefined ? "—" : t("grantActionCount").replace("{count}", String(r.historyCount))}</Link></td>
                    </>}
                    {type === "notes" && (
                      <>
                      <td data-label={t("noteGenerationMethod")}><NoteGenerationLabel data={r} t={t}/></td>
                      <td className="amount-cell" data-label={t("notePublicAmount")}>{r.amount !== undefined && r.amount !== null ? <Amount value={r.amount} data={r} compact/> : <span className="muted">{t("noteAmountPrivate")}</span>}</td>
                      <td
                        className="recognition-cell"
                        data-label={t("recognition")}
                      >
                        <RecognitionBadges data={r} />
                      </td>
                      </>
                    )}
                    <td
                      data-label={t(full ? "time" : "age")}
                      title={
                        fullTime(r.timestamp) +
                        " · " +
                        Intl.DateTimeFormat().resolvedOptions().timeZone
                      }
                    >
                      {full ? fullTime(r.timestamp) : age(r.timestamp, now, t)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      <div className="pagination">
        <span>
          {(result.data?.total == null
            ? t("paginationPage")
            : t("paginationSummary").replace("{total}", String(result.data.total))
          ).replace("{page}", String(result.page))}
        </span>
        <div>
          <button
            disabled={result.page === 1 || result.loading}
            onClick={result.previous}
          >
            {t("previous")}
          </button>
          <button
            disabled={!result.data?.nextCursor || result.loading || !!result.error}
            onClick={result.next}
          >
            {t("next")}
          </button>
        </div>
      </div>
    </section>
  );
}
function Search() {
  const { t } = useContext(Context);
  const nav = useNavigate();
  const [query, setQuery] = useState("");
  const [error, setError] = useState<Key>();
  const [loading, setLoading] = useState(false);
  const [results, setResults] = useState<RecordData[] | null>(null);
  return (
    <section className="search-area">
      <form
        className="search"
        onSubmit={async (e) => {
          e.preventDefault();
          const invalid = validateSearch(query);
          setError(invalid ?? undefined);
          if (invalid) return;
          setLoading(true);
          try {
            const data = await request<{ results: RecordData[] }>(
              "/search",
              undefined,
              { query: query.trim() },
            );
            setResults(data.results);
            if (data.results.length === 1) {
              const r = data.results[0];
              nav(
                objectHref(
                  collectionFor(r.type),
                  r,
                  r.hash ?? objectIdentity(collectionFor(r.type), r),
                ),
              );
            }
          } catch {
            setError("unavailable");
          } finally {
            setLoading(false);
          }
        }}
      >
        <input
          value={query}
          onChange={(e) => {
            setQuery(e.target.value);
            setError(undefined);
          }}
          aria-label={t("search")}
          placeholder={t("searchHint")}
          autoComplete="off"
          spellCheck={false}
        />
        <button className="primary" disabled={loading}>
          {t("search")}
        </button>
      </form>
      <p className="muted">{t("searchPrivacy")}</p>
      {error && <p role="alert">{t(error)}</p>}
      {loading && <State loading />}
      {results?.length === 0 && <State empty />}
      {results && results.length > 1 && (
        <div className="card">
          {results.map((r, i) => (
            <p key={i}>
              <Link
                to={objectHref(
                  r.type,
                  r,
                  r.hash ?? objectIdentity(collectionFor(r.type), r),
                )}
              >
                {r.type}: {r.hash ?? objectIdentity(collectionFor(r.type), r)}
              </Link>
              <Network data={r} />
            </p>
          ))}
        </div>
      )}
    </section>
  );
}
function PublicFields({ data, showSourceTransaction = true, showNoteAmount = false }: { data: RecordData; showSourceTransaction?: boolean; showNoteAmount?: boolean }) {
  const { t } = useContext(Context);
  const known: [string, Key][] = [
    ["kind", "operation"],
    ["timestamp", "time"],
    ["blockNumber", "block"],
    ["withdrawalAmount", "withdrawal"],
    ["feeAmount", "fee"],
    ["amount", "noteAmount"],
    ["treeId", "treeIdLabel"],
    ["leafIndex", "leafIndexLabel"],
    ["reason", "reason"],
  ];
  return (
    <dl className="public-fields">
      {"success" in data && (
        <>
          <dt>{t("execution")}</dt>
          <dd>
            <Status value={data.success} />
          </dd>
        </>
      )}{" "}
      {known.map(
        ([key, label]) =>
          (data[key] !== undefined || (key === "amount" && showNoteAmount)) && !(isCreditOperation(data.kind) && ["withdrawalAmount", "feeAmount"].includes(key)) && (
            <React.Fragment key={key}>
              <dt>{t(label)}</dt>
              <dd>
                {key === "timestamp" ? (
                  fullTime(data[key])
                ) : ["amount", "withdrawalAmount", "feeAmount"].includes(
                    key,
                  ) ? (
                  data[key] === undefined || data[key] === null ? <span className="muted">{t("notPublic")}</span> : <Amount value={data[key]} data={data} />
                ) : key === "kind" && data[key] in dictionaries.en ? (
                  t(data[key] as Key)
                ) : (
                  String(data[key])
                )}
                {key === "amount" && <p className="muted note-amount-hint">{t("notePublicAmountHint")}</p>}
              </dd>
            </React.Fragment>
          ),
      )}
      {data.chainId && (
        <>
          <dt>{t("network")}</dt>
          <dd>
            <Network data={data} />
          </dd>
        </>
      )}
      {data.protocolAddress && (
        <>
          <dt>{t("protocol")}</dt>
          <dd>
            <Address value={data.protocolAddress} kind="address" data={data} />
          </dd>
        </>
      )}
      {data.tokenAddress && (
        <>
          <dt>{t("token")}</dt>
          <dd>
            <Address value={data.tokenAddress} kind="token" data={data} />
          </dd>
        </>
      )}
      {showSourceTransaction && (data.txHash ?? data.transactionHash) && (
        <>
          <dt>{t("sourceTransaction")}</dt>
          <dd>
            <Address
              value={data.txHash ?? data.transactionHash}
              kind="tx"
              data={data}
              internalHref={objectHref("transactions", data, data.txHash ?? data.transactionHash)}
            />
          </dd>
        </>
      )}
    </dl>
  );
}
function ObjectDescription({type}: {type: string}) {
  const {t} = useContext(Context);
  return type === "trees" || type === "roots" ? <p className="muted">{t(type === "trees" ? "treesDescription" : "rootsDescription")}</p> : null;
}
function GrantDetail({id}: {id: string}) {
  const {t} = useContext(Context);
  const result = useCursorPagination<RecordData>("/grants/" + encodeURIComponent(id));
  const data = result.data;
  return <>
    <p><Link to="/grants">{t("grants")}</Link> /</p>
    <h1>{t("creditHistory")}</h1>
    {!data ? <section className="card"><State {...result}/></section> : <>
      {result.notice && <p className="pagination-message" role="status">{t(result.notice === "updated" ? "paginationUpdated" : "paginationRestarted")}</p>}
      {result.error && <p role="alert">{t("unavailable")} <button onClick={result.retry}>{t("retry")}</button></p>}
      <GrantHistory data={data} network={<Network data={data}/>} t={t}
        address={(value,kind)=><Address value={value} kind={kind ?? "address"} data={data}/>}
        transaction={row=><Address value={row.txHash} kind="tx" data={row} internalHref={objectHref("transactions",row,row.txHash)}/>}
        amount={(value,row)=><Amount value={value} data={{...data,...row}} compact/>}
        href={(collection,row,value)=>objectHref(collection,row,value)}/>
      <div className="pagination">
        <span>{t("paginationSummary").replace("{total}",String(data.activities?.total ?? 0)).replace("{page}",String(result.page))}</span>
        <div><button disabled={result.loading || result.page === 1} onClick={result.previous}>{t("previous")}</button>
        <button disabled={result.loading || !!result.error || !data.nextCursor} onClick={result.next}>{t("next")}</button></div>
      </div>
    </>}
  </>;
}
function Detail({ type, id }: { type: string; id: string }) {
  const { t, metadata } = useContext(Context);
  const location = useLocation();
  const result = useData<RecordData>(
    "/" + type + "/" + encodeURIComponent(id) + location.search,
  );
  const data = result.data;
  const operations = data?.indexAnomaly
    ? []
    : (data?.operations ?? data?.activities?.items ?? (data ? [data] : []));
  const aggregate = type === "transactions" && operations.length > 1;
  const showRecognition = !aggregate && showsSourceRecognition(type, data);
  return (
    <>
      <p>
        <Link to={type === "credit-states" ? "/grants" : "/" + type}>{t(type === "credit-states" ? "grants" : type as Key)}</Link> /
      </p>
      <h1>{t(type as Key)}</h1>
              <ObjectDescription type={type}/>
      {aggregate && <p className="muted">{t("aggregate")} · {t("operationCount").replace("{count}",String(operations.length))}</p>}
      <div className="detail-hash">
        <Address
          value={id}
          kind={type === "transactions" ? "tx" : undefined}
          data={operations[0] ?? data}
        />
      </div>
      {result.loading || result.error ? (
        <section className="card">
          <State {...result} />
        </section>
      ) : (
        <div className={showRecognition ? "detail-grid" : "detail-grid detail-grid-single"}>
          <div>
            {data?.indexAnomaly && (
              <section className="card">
                <State
                  error={{ code: "INDEX_ANOMALY", status: 409 }}
                  retry={result.retry}
                />
              </section>
            )}
            {operations.map((op: RecordData, i: number) => (
              <section className="card" key={i}>
                <h2>
                  {aggregate ? t("operationNumber").replace("{number}",String(BigInt(op.operationIndex ?? i)+1n)) + " · " + (op.kind in dictionaries.en ? t(op.kind as Key) : op.kind) : t("publicInfo")}
                </h2>
                <PublicFields data={op} showSourceTransaction={type !== "transactions"} showNoteAmount={type === "notes"} />
                {aggregate && <SourceRecognition data={op} t={t} status={value=><Status value={value}/>}/>}
                {isCreditOperation(op.kind) && <CreditOperationPanel operation={op} t={t}
                  address={(value, kind) => <Address value={value} kind={kind ?? "address"} data={op}/>}
                  amount={(value, token) => <Amount value={value} data={{...op, tokenAddress:token}}/>}
                  href={(collection, value) => objectHref(collection,op,value)}/>}
                {!isCreditOperation(op.kind) && (type === "transactions" || (Array.isArray(op.funding) && op.funding.length > 0) ||
                  op.fundingEvidence ||
                  op.routerFunding ||
                  op.executionContext ||
                  op.unsupportedReason) && (
                  <FundingPanel
                    operation={op}
                    t={t}
                    address={(value, kind) => (
                      <Address value={value} kind={kind} data={op} />
                    )}
                    amount={(value, token) => (
                      <Amount
                        value={value}
                        data={{ ...op, tokenAddress: token }}
                      />
                    )}
                  />
                )}
                {op.noteOutputs?.length > 0 && (
                  <>
                    <h3>{t("noteOutputsCount").replace("{count}", String(op.noteOutputs.length))}</h3>
                    <div className="table-wrap note-outputs-wrap">
                      <table className="note-outputs-table">
                        <thead><tr>
                          <th>{t("noteCommitmentLabel")}</th><th>{t("outputAmount")}</th>
                          <th>{t("treeIdLabel")}</th><th>{t("leafIndexLabel")}</th>
                        </tr></thead>
                        <tbody>{op.noteOutputs.map((note: RecordData, j: number) => (
                          <tr key={note.noteCommitment ?? j}>
                            <td data-label={t("noteCommitmentLabel")}><Address value={note.noteCommitment} data={op} internalHref={objectHref("notes", op, note.noteCommitment)}/></td>
                            <td data-label={t("outputAmount")}>{note.amount === undefined ? t("notPublic") : <Amount value={note.amount} data={op}/>}</td>
                            <td data-label={t("treeIdLabel")}>{note.treeId == null ? "—" : <Link to={objectHref("trees", op, "protocol:" + note.treeId)}>{note.treeId}</Link>}</td>
                            <td data-label={t("leafIndexLabel")}>{note.leafIndex ?? "—"}</td>
                          </tr>
                        ))}</tbody>
                      </table>
                    </div>
                  </>
                )}
                {!(type === "transactions" && isCreditOperation(op.kind)) && (op.draws ?? op.drawRecords)?.length > 0 && (
                  <>
                    <h3>{t("drawRecords")}</h3>
                    {(op.draws ?? op.drawRecords).map(
                      (draw: RecordData, j: number) => (
                        <div className="output" key={j}>
                          <Link
                            to={objectHref(
                              "transactions",
                              draw,
                              draw.txHash ?? draw.transactionHash,
                            )}
                          >
                            {draw.txHash ?? draw.transactionHash}
                          </Link>
                          {draw.noteCommitment && (
                            <p>
                              <Link
                                to={objectHref(
                                  "notes",
                                  draw,
                                  draw.noteCommitment,
                                )}
                              >
                                {draw.noteCommitment}
                              </Link>
                            </p>
                          )}
                          <p>{fullTime(draw.timestamp)}</p>
                        </div>
                      ),
                    )}
                  </>
                )}
                {!isCreditOperation(op.kind) && op.creditOutputs?.map((credit: RecordData, j: number) => (
                  <p key={j}>
                    <Link
                      to={objectHref(
                        "credit-states",
                        op,
                        credit.creditCommitment ?? credit.commitment,
                      )}
                    >
                      {credit.creditCommitment ?? credit.commitment}
                    </Link>
                  </p>
                ))}
                {!isCreditOperation(op.kind) && op.grantRefs?.map((grant: RecordData, j: number) => (
                  <p key={j}>
                    <Link
                      to={
                        "/grants/" +
                        encodeURIComponent(
                          [
                            op.chainId,
                            op.protocolAddress,
                            grant.tokenAddress ?? op.tokenAddress,
                            grant.payer,
                            grant.grantId,
                          ].join(":"),
                        ) +
                        scope(op)
                      }
                    >
                      {t("grants")} · {grant.payer} / {grant.grantId}
                    </Link>
                  </p>
                ))}
              </section>
            ))}
            <section className="card">
              <details>
                <summary>{t("raw")}</summary>
                <pre>{JSON.stringify(data, null, 2)}</pre>
              </details>
            </section>
          </div>
          {showRecognition && <aside>
            <SourceRecognition data={data ?? {}} t={t} status={value=><Status value={value}/>} aside/>
          </aside>}
        </div>
      )}
    </>
  );
}
function TransactionsPage() {
  const { t } = useContext(Context);
  const result = useData<RecordData>("/overview");
  return (
    <>
      <h1>{t("transactions")}</h1>
      <Search />
      <div className="stats">
        {(["transactions", "notes", "grants"] as const).map((key) => (
          <Link className="card" to={key === "transactions" ? "/" : "/" + key} key={key}>
            <span>{t(key)}</span>
            <strong>
              {result.data?.[key] ??
                "—"}
            </strong>
          </Link>
        ))}
      </div>
      {result.error && <State error={result.error} retry={result.retry} />}
      <Listing compact />
    </>
  );
}
const languageOptions: { value: Language; label: string; detail: string }[] = [
  { value: "zh-Hans", label: "简体中文", detail: "Simplified Chinese" },
  { value: "zh-Hant", label: "繁體中文", detail: "Traditional Chinese" },
  { value: "en", label: "English", detail: "英语" },
];
function LanguagePicker({ language, onChange, label }: {
  language: Language; onChange: (value: Language) => void; label: string;
}) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    root.current?.querySelector<HTMLButtonElement>('[aria-checked="true"]')?.focus();
    const outside = (event: PointerEvent) => {
      if (!root.current?.contains(event.target as Node)) setOpen(false);
    };
    const escape = (event: KeyboardEvent) => {
      if (event.key === "Escape") { setOpen(false); trigger.current?.focus(); }
    };
    document.addEventListener("pointerdown", outside);
    document.addEventListener("keydown", escape);
    return () => {
      document.removeEventListener("pointerdown", outside);
      document.removeEventListener("keydown", escape);
    };
  }, [open]);
  return <div className="language-picker" ref={root} onBlur={(event) => {
    if (!event.currentTarget.contains(event.relatedTarget as Node)) setOpen(false);
  }}>
    <button className="language-trigger" ref={trigger} aria-label={label}
      aria-haspopup="menu" aria-expanded={open} aria-controls="language-menu"
      onClick={() => setOpen(!open)}
      onKeyDown={(event) => {
        if (event.key === "ArrowDown" || event.key === "ArrowUp") {
          event.preventDefault(); setOpen(true);
        }
      }}>
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.6" aria-hidden="true">
        <circle cx="12" cy="12" r="9" /><path d="M3 12h18M12 3c-5 5-5 13 0 18 5-5 5-13 0-18Z" />
      </svg>
      <span>{languageOptions.find(item => item.value === language)?.label}</span>
      <svg className="language-chevron" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><path d="m7 10 5 5 5-5" /></svg>
    </button>
    {open && <div className="language-menu" id="language-menu" role="menu" aria-label={label}
      onKeyDown={(event) => {
        const items = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]'));
        const index = items.indexOf(document.activeElement as HTMLButtonElement);
        let next = index;
        if (event.key === "ArrowDown") next = (index + 1) % items.length;
        else if (event.key === "ArrowUp") next = (index - 1 + items.length) % items.length;
        else if (event.key === "Home") next = 0;
        else if (event.key === "End") next = items.length - 1;
        else return;
        event.preventDefault(); items[next]?.focus();
      }}>
      {languageOptions.map(item => <button key={item.value} role="menuitemradio"
        aria-checked={language === item.value} data-language={item.value}
        onClick={() => { onChange(item.value); setOpen(false); trigger.current?.focus(); }}>
        <span><span className="language-name">{item.label}</span><small>{item.detail}</small></span>
        {language === item.value && <svg width="17" height="17" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true"><path d="m5 12 4 4 10-10" /></svg>}
      </button>)}
    </div>}
  </div>;
}
function App() {
  const [language, setLanguage] = useState<Language>(initialLanguage);
  const metadata = useData<RecordData>("/metadata", 30000);
  const location = useLocation();
  const [type, id] = location.pathname.split("/").filter(Boolean);
  const t = (key: Key) => dictionaries[language][key] ?? key;
  useEffect(() => {
    document.documentElement.lang = language;
    try {
      localStorage.setItem("invisi.language", language);
    } catch {}
  }, [language]);
  return (
    <Context.Provider value={{ t, metadata: metadata.data ?? {}, metadataPending: !metadata.data }}>
      <header>
        <div className="nav">
          <Link className="brand" to="/" aria-label="InvisiPay · Explorer">
            <img className="brand-mark" src="/assets/invisipay-mark.svg" alt="" width="32" height="32" />
            <span className="brand-name">Invisi<span className="brand-pay">Pay</span></span>
            <span className="brand-product">Explorer</span>
          </Link>
          <LanguagePicker language={language} onChange={setLanguage} label={t("language")} />
        </div>
        <nav aria-label={t("all")}>
          {routes.filter(r=>!["credit-states","nullifiers","trees","roots"].includes(r)).map((r) => (
            <Link
              key={r}
              to={r === "transactions" ? "/" : "/" + r}
              aria-current={type === r || (type === "credit-states" && r === "grants") || (!type && r === "transactions") ? "page" : undefined}
            >
              {t(r === "broadcasters" ? "broadcastersNav" : r)}
            </Link>
          ))}
          <AdvancedTools t={t}/>
        </nav>
      </header>
      <main>
        {type === "transactions" && !id ? (
          <Navigate to={{ pathname: "/", search: location.search, hash: location.hash }} replace />
        ) : !type ? (
          <TransactionsPage />
        ) : type === "diagnostics" && !id ? (
          <>
            <h1>{t("diagnostics")}</h1>
            <Diagnostics />
          </>
        ) : routes.includes(type as Key) ||
          type === "addresses" ||
          type === "commitments" ? (
          id ? type === "grants" ? (
            <GrantDetail key={location.pathname + location.search} id={decodeURIComponent(id)}/>
          ) : (
            <Detail
              key={location.pathname + location.search}
              type={type}
              id={decodeURIComponent(id)}
            />
          ) : (
            <>
              <h1>{t(type as Key)}</h1>
              <ObjectDescription type={type}/>
              <Search />
              <Listing key={type} type={type} />
            </>
          )
        ) : (
          <State error={{ code: "NOT_INDEXED", status: 404 }} />
        )}
      </main>
      <footer>
        <span>Invisi Labs</span>
        <span>
          {t("timezone")}: {Intl.DateTimeFormat().resolvedOptions().timeZone}
        </span>
      </footer>
    </Context.Provider>
  );
}
function Diagnostics() {
  const { t } = useContext(Context);
  const result = useData<DiagnosticsResponse>("/diagnostics");
  return result.data ? (
    <DiagnosticsPanel
      items={result.data.items}
      t={t}
      network={(record) => <Network data={record} />}
    />
  ) : (
    <State {...result} />
  );
}
createRoot(document.getElementById("root")!).render(
  <BrowserRouter>
    <App />
  </BrowserRouter>,
);
