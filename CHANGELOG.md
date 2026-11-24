# Changelog

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
