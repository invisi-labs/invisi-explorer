# Changelog

## Unreleased

- Simplify mobile branding and add localized, accessible terminology help for source recognition, private Notes, execution roles and finality.

## 1.3.7

- Persist recognized provenance results across temporary query failures while continuing pending-result refresh. Fence cached results by canonical bindings, source generations and reorg epochs.
- Explain private Note inputs without treating absent public funding traces as failures. Show aggregate transaction execution once, retaining per-operation inputs, outputs, public funding and recognition.
- Refine related-grant cards and link directly from grant identifiers.

- Keep relay operating statistics private to the authenticated Broadcaster console. Remove public statistics probing, response fields and directory presentation while preserving service health and token capability discovery.

## 1.3.6

- Discover registered relay services from Protocol RPC and cache bounded background connectivity, deployment, token-catalog and service-capability observations. Keep RPC registration, Explorer health checks and service declarations separate; preserve stale and missing data explicitly.
- Add dedicated relay directory and detail views with configured-token availability, scheduling, Router support, provenance-check freshness and collapsed technical registration information. Support older relay versions without treating missing token catalogs as empty support.
- Distinguish pre-revoked grant slots from creation-and-revocation history while keeping authorization labels Active/Revoked. Count full canonical slot history and bind credit-draw outputs to the matching owner and GrantID using proof-authenticated input order.
- Consolidate technical navigation under Advanced tools, add Note generation/public-amount columns and precise query-state labels, and simplify list references and copy actions. Preserve exact amount hover/copy, metadata cache/retry behavior, private amount boundaries, canonical checks and reorg handling.
- Retain existing databases, cursor keys, deployment scopes and service identities. No reset or reindex is required.

## 1.3.5

- Package current root documentation and built frontend from a clean source export without requiring removed migration documents. Keep archive smoke checks aligned with the maintained release contents.
- Present credit creation, updates, draws and revocations with operation-specific public fields. Automatically backfill canonical grant identity and group fixed grant slots into one entity with paginated action/state history. Keep private policy and balances undisclosed and distinguish shared payer amounts.
- Paginate transaction instances rather than operation rows, with aggregate operation counts, per-operation detail recognition, mixed execution outcomes and separate per-token public funding totals. Preserve scoped snapshots and restart legacy cursors.
- Bind optional transaction sender/type/recipient observations to indexed receipts and fresh canonical headers. Derive public withdrawal recipients only from authenticated statements, retain evidence uncertainty, and expose public transfer execution data across transaction kinds.
- Align copy/explorer actions, Note/Credit output tables, localized tree positions, source-recognition applicability and private Note amount layout. Consolidate primary grant navigation and distinct grant statistics.

## 1.3.3

- Discover ERC-20 tokens in public history and funding transfers; read symbol and decimals through finalized Protocol RPC calls and persist an independent Explorer metadata cache. Preserve configured overrides, unknown precision and retry behavior without interrupting indexing.
- Shorten list amounts with explicit rounding markers, retain exact detail values, and show full precision on hover or keyboard focus. Copy the exact ungrouped amount without floating point conversion.
- Recover expired pagination snapshots with fresh authenticated cursors, commit page numbers only after successful loads, prevent duplicate navigation and retain valid rows and totals during errors. Preserve backend conflict and reorg validation.
- Use complete localized pagination labels, including “共 52 条 · 第 3 页”, and omit unknown totals.

## 1.3.2

- Prefetch up to four complete finalized history windows concurrently, then validate fresh boundaries and publish in order through one atomic SQLite transaction per window. Retain sequential near-head scanning, durable dense-page cursors and reorg recovery.
- Fall back to durable scanning when bounded prefetch times out; discard interrupted terminal staging before refetching and retain accurate failure/last-success diagnostics. Public APIs and database ownership remain unchanged.

## 1.3.0

- Release Explorer as an independent service with its own browser API, public query index and React frontend. Protocol RPC supplies chain facts, Provenance supplies recognition evidence and Broadcaster supplies optional public service information.
- Preserve independent execution and recognition states, public Note/Credit views, synchronization diagnostics and explicit upstream scope configuration. Explorer holds no upstream signing keys or private submission journals.
- Publish Linux amd64/arm64 container images and Linux/macOS amd64/arm64 CLI archives containing the built frontend, documentation, Apache-2.0 license and SHA-256 checksums.
- Validate clean-source builds and smoke-test release archives and container static assets before publication. Existing Provenance databases, issuer identities, signing keys and published history stay with Provenance during the Explorer cutover.
