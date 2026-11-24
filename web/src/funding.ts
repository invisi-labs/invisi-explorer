/** Public observations only. A source candidate is never a beneficial-owner claim. */
export interface FundingAsset {
  tokenAddress: string;
  amount: string;
}
export interface CoreFunding extends FundingAsset {
  payer: string;
}
export interface FundingTransfer extends FundingAsset {
  kind: "native" | "erc20";
  from: string;
  to: string;
  eventKind?: string;
  callPath: string;
  contextPath: string;
  logIndex?: string;
  order: string;
}
export interface ExecutionCall {
  path: string;
  type: string;
  from: string;
  to: string;
}
export interface ExecutionAuthorization {
  chainId: string;
  address: string;
  authority?: string;
  nonce: string;
  signatureValid: boolean;
}
export interface FundingSourceCandidate {
  address: string;
  /** Amounts are scoped to selected evidence, not final wallet spending; later refunds may be absent. */
  assets: FundingAsset[];
  basis: "selected-flow-outflow";
}
export type FundingCoverage =
  "scoped-flow" | "contract-inventory" | "ambiguous" | "unavailable" | "legacy";
export type FundingWarning =
  | "evidence-unavailable"
  | "legacy-evidence"
  | "ambiguous-evidence"
  | "inventory-boundary"
  | "execution-failed"
  | "orphaned-operation"
  | "execution-unknown"
  | "source-not-beneficial-owner"
  | "source-unresolved"
  | "authorization-not-execution-proof"
  | "invalid-evidence"
  | "context-conflict";
export interface FundingView {
  /** Recorded Core payer rows. Gate executed role labels on rolesConfirmed. */
  coreFunding: CoreFunding[];
  sourceCandidates: FundingSourceCandidate[];
  /** Address convenience projection; same uncertainty as sourceCandidates. */
  fundedBy: string[];
  intermediaries: string[];
  /** Transfer participants, excluding the mint/burn zero address. */
  observedAddresses: string[];
  submitter: string | null;
  transactionTo: string | null;
  transactionType: string | null;
  coreCallPath: string | null;
  /** Observed call frames, not authorization-derived execution. */
  calls: ExecutionCall[];
  authorizations: ExecutionAuthorization[];
  is7702Transaction: boolean;
  directCoreCall: boolean;
  coverage: FundingCoverage;
  transfers: FundingTransfer[];
  warnings: FundingWarning[];
  execution: "executed" | "failed" | "orphaned" | "unknown";
  rolesConfirmed: boolean;
  /** Historical metadata retained without reconstructing absent trace facts. */
  legacy: Record<string, string> | null;
}

const ZERO = "0x0000000000000000000000000000000000000000";
const object = (v: unknown): Record<string, unknown> =>
  v !== null && typeof v === "object" && !Array.isArray(v)
    ? (v as Record<string, unknown>)
    : {};
const list = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);
const address = (v: unknown): string | null =>
  typeof v === "string" && /^0x[0-9a-f]{40}$/i.test(v) ? v.toLowerCase() : null;
const uint = (v: unknown): v is string =>
  typeof v === "string" && /^(0|[1-9][0-9]{0,77})$/.test(v);
const path = (v: unknown): v is string =>
  typeof v === "string" &&
  v.length <= 512 &&
  /^0(?:\/(?:0|[1-9][0-9]*))*$/.test(v);
const text = (v: unknown): string | null =>
  typeof v === "string" && v.length > 0 ? v : null;

/**
 * Builds a pure presentation model from the public operation API. The backend
 * authenticates evidence; this function checks shape and keeps missing evidence
 * explicit. It never guesses a Router identity, transaction sender, or owner.
 */
export function fundingView(input: unknown): FundingView {
  const op = object(input),
    evidence = object(op.fundingEvidence),
    context = object(op.executionContext);
  const warnings = new Set<FundingWarning>();
  const execution: FundingView["execution"] =
    op.canonical === false
      ? "orphaned"
      : op.success === false
        ? "failed"
        : op.success === true && op.canonical === true
          ? "executed"
          : "unknown";
  if (execution === "orphaned") warnings.add("orphaned-operation");
  if (execution === "failed") warnings.add("execution-failed");
  if (execution === "unknown") warnings.add("execution-unknown");
  const coreFunding: CoreFunding[] = [];
  for (const item of list(op.funding)) {
    const f = object(item),
      payer = address(f.payer),
      tokenAddress = address(f.tokenAddress);
    if (payer && payer !== ZERO && tokenAddress && uint(f.amount))
      coreFunding.push({ payer, tokenAddress, amount: f.amount });
    else warnings.add("invalid-evidence");
  }
  let legacy: Record<string, string> | null = null;
  if (op.routerFunding && typeof op.routerFunding === "object") {
    legacy = {};
    for (const [key, value] of Object.entries(object(op.routerFunding)))
      if (typeof value === "string") legacy[key] = value;
  }
  let coverage: FundingCoverage = "unavailable";
  let validEvidence = false;
  if (op.fundingEvidence !== undefined && op.fundingEvidence !== null) {
    if (
      evidence.version === "core-funding-v1" &&
      ["scoped-flow", "contract-inventory", "ambiguous"].includes(
        String(evidence.coverage),
      ) &&
      path(evidence.coreCallPath) &&
      address(evidence.transactionSender) &&
      Array.isArray(evidence.transfers)
    ) {
      coverage = evidence.coverage as FundingCoverage;
      validEvidence = true;
    } else warnings.add("invalid-evidence");
  } else if (legacy) coverage = "legacy";
  if (coverage === "unavailable") warnings.add("evidence-unavailable");
  if (coverage === "legacy") warnings.add("legacy-evidence");
  if (coverage === "ambiguous") warnings.add("ambiguous-evidence");
  if (coverage === "contract-inventory") warnings.add("inventory-boundary");
  const transfers: FundingTransfer[] = [];
  let previousOrder: bigint | null = null;
  for (const item of list(evidence.transfers)) {
    const t = object(item),
      from = address(t.from),
      to = address(t.to),
      tokenAddress = address(t.tokenAddress);
    const native =
      t.kind === "native" &&
      tokenAddress === ZERO &&
      from !== ZERO &&
      to !== ZERO;
    const token =
      t.kind === "erc20" && tokenAddress !== ZERO && uint(t.logIndex);
    if (
      !from ||
      !to ||
      !tokenAddress ||
      (from === ZERO && to === ZERO) ||
      !(native || token) ||
      !uint(t.amount) ||
      BigInt(t.amount) === 0n ||
      !uint(t.order) ||
      !path(t.callPath) ||
      !path(t.contextPath)
    ) {
      validEvidence = false;
      warnings.add("invalid-evidence");
      continue;
    }
    if (previousOrder !== null && BigInt(t.order) <= previousOrder) {
      validEvidence = false;
      warnings.add("invalid-evidence");
    }
    previousOrder = BigInt(t.order);
    const transfer: FundingTransfer = {
      kind: t.kind as FundingTransfer["kind"],
      tokenAddress,
      from,
      to,
      amount: t.amount,
      callPath: t.callPath,
      contextPath: t.contextPath,
      order: t.order,
    };
    if (typeof t.eventKind === "string") transfer.eventKind = t.eventKind;
    if (uint(t.logIndex)) transfer.logIndex = t.logIndex;
    transfers.push(transfer);
  }
  const calls: ExecutionCall[] = [];
  for (const item of list(context.calls)) {
    const c = object(item),
      from = address(c.from),
      to = address(c.to),
      type = text(c.type);
    if (path(c.path) && from && to && type)
      calls.push({ path: c.path, type, from, to });
    else warnings.add("invalid-evidence");
  }
  const authorizations: ExecutionAuthorization[] = [];
  for (const item of list(context.authorizations)) {
    const a = object(item),
      delegate = address(a.address),
      authority = address(a.authority);
    if (
      uint(a.chainId) &&
      uint(a.nonce) &&
      delegate &&
      typeof a.signatureValid === "boolean"
    ) {
      authorizations.push({
        chainId: a.chainId,
        address: delegate,
        ...(authority ? { authority } : {}),
        nonce: a.nonce,
        signatureValid: a.signatureValid,
      });
    } else warnings.add("invalid-evidence");
  }
  if (authorizations.length) warnings.add("authorization-not-execution-proof");
  const observation = object(op.transactionObservation);
  const observed = observation.chainId === op.chainId &&
    address(observation.protocolAddress) === address(op.protocolAddress) &&
    typeof op.txHash === "string" && String(observation.txHash).toLowerCase() === op.txHash.toLowerCase() &&
    typeof op.blockHash === "string" && String(observation.blockHash).toLowerCase() === op.blockHash.toLowerCase() &&
    observation.blockNumber === op.blockNumber;
  const observedSender = observed ? address(observation.transactionSender) : null;
  const observedTo = observed ? address(observation.transactionTo) : null;
  const observedType = observed && uint(observation.transactionType) ? observation.transactionType : null;
  const transactionType = uint(context.transactionType) ? context.transactionType : observedType;
  const contextSender = address(context.transactionSender),
    evidenceSender = address(evidence.transactionSender);
  if ((observedSender && contextSender && observedSender !== contextSender) ||
      (observedSender && evidenceSender && observedSender !== evidenceSender) ||
      (observedType && uint(context.transactionType) && observedType !== context.transactionType) ||
      (observedTo && address(context.transactionTo) && observedTo !== address(context.transactionTo))) warnings.add("context-conflict");
  const contextPath = path(context.coreCallPath) ? context.coreCallPath : null;
  const evidencePath = path(evidence.coreCallPath)
    ? evidence.coreCallPath
    : null;
  if (
    (contextSender && evidenceSender && contextSender !== evidenceSender) ||
    (contextPath && evidencePath && contextPath !== evidencePath) ||
    (text(op.executionPath) &&
      evidencePath &&
      op.executionPath !== evidencePath)
  )
    warnings.add("context-conflict");
  const rolesConfirmed =
    execution === "executed" &&
    !warnings.has("context-conflict") &&
    !warnings.has("invalid-evidence");
  const observedAddresses = [
    ...new Set(
      transfers.flatMap((t) => [t.from, t.to]).filter((a) => a !== ZERO),
    ),
  ];
  const sourceCandidates: FundingSourceCandidate[] = [];
  let intermediaries: string[] = [];
  if (rolesConfirmed && validEvidence && coverage === "scoped-flow") {
    // Net only the provided evidence per asset; later refunds may be absent.
    // These amounts must not be presented as final wallet spending.
    // Cross-asset net receivers (e.g. DEX pools) remain intermediaries, even
    // when they supply all of the final settlement token.
    const changes = new Map<string, Map<string, bigint>>();
    const change = (account: string, token: string, delta: bigint) => {
      const assets = changes.get(account) ?? new Map<string, bigint>();
      assets.set(token, (assets.get(token) ?? 0n) + delta);
      changes.set(account, assets);
    };
    for (const t of transfers) {
      change(t.from, t.tokenAddress, BigInt(t.amount));
      change(t.to, t.tokenAddress, -BigInt(t.amount));
    }
    const core = address(op.protocolAddress);
    for (const [account, assets] of changes) {
      if (
        account === ZERO ||
        account === core ||
        [...assets.values()].some((v) => v < 0n)
      )
        continue;
      const outgoing = [...assets]
        .filter(([, amount]) => amount > 0n)
        .map(([tokenAddress, amount]) => ({
          tokenAddress,
          amount: amount.toString(),
        }));
      if (outgoing.length)
        sourceCandidates.push({
          address: account,
          assets: outgoing,
          basis: "selected-flow-outflow",
        });
    }
    const origins = new Set(sourceCandidates.map((s) => s.address));
    intermediaries = observedAddresses.filter(
      (a) =>
        a !== core &&
        !origins.has(a) &&
        transfers.some((t) => t.from === a) &&
        transfers.some((t) => t.to === a),
    );
    if (!sourceCandidates.length) warnings.add("source-unresolved");
  }
  if (sourceCandidates.length) warnings.add("source-not-beneficial-owner");
  return {
    coreFunding,
    sourceCandidates,
    fundedBy: sourceCandidates.map((s) => s.address),
    intermediaries,
    observedAddresses,
    submitter: contextSender ?? evidenceSender ?? observedSender,
    transactionTo: address(context.transactionTo) ?? observedTo,
    transactionType,
    coreCallPath:
      contextPath ??
      evidencePath ??
      (path(op.executionPath) ? op.executionPath : null),
    calls,
    authorizations,
    is7702Transaction: transactionType === "4",
    directCoreCall: rolesConfirmed && observedTo !== null && observedTo === address(op.protocolAddress),
    coverage,
    transfers,
    warnings: [...warnings],
    execution,
    rolesConfirmed,
    legacy,
  };
}
