# Explorer public API

Explorer serves its browser and REST API on `127.0.0.1:18670` by default. It owns an independent SQLite query database. It consumes explicit Protocol RPC and optional Provenance and Broadcaster endpoints. It never opens upstream databases, evaluates recognition, constructs proofs, or signs certificates.

All responses are JSON. GET routes are read-only; search is POST-only with `Content-Type: application/json`. Errors are `{ "error": { "code": "INVALID_PARAMS" } }`; errors never include upstream URLs, response bodies or credentials. HTTP statuses: invalid/secret inputs 400, absent/index-unobserved 404, changed snapshot 409, unsupported query 422, unavailable/index-pending 503.

| Route | Result |
| --- | --- |
| `GET /healthz` | Process HTTP liveness, independent of upstream readiness |
| `GET /api/v1/metadata` | Configured networks and configured/cached token metadata |
| `GET /api/v1/overview` | Counts in the indexed canonical prefix, snapshot and recognition count availability |
| `GET /api/v1/transactions` | Settlement-operation rows, including unsupported or failed public operations |
| `GET /api/v1/transactions/{txHash}` | Operations grouped for a transaction, remote recognition and snapshot |
| `GET /api/v1/notes`, `credit-states`, `commitments`, `nullifiers`, `grants` | Typed public projections |
| `GET /api/v1/notes/{commitment}`, `credit-states/{commitment}`, `nullifiers/{hash}` | Selected object and known occurrences |
| `GET /api/v1/notes/{commitment}/occurrences`, `credit-states/{commitment}/occurrences` | Canonical and orphan occurrence history |
| `GET /api/v1/commitments/{commitment}` | Domain-qualified occurrences; no Note/Credit substitution |
| `GET /api/v1/grants/{chainId:protocol:token:payer:grantId}` | Grant activity and Protocol RPC status at the snapshot |
| `GET /api/v1/addresses/{address}` | Public payer/sender/token/protocol/fee-participant activity; no recipient inference |
| `GET /api/v1/trees`, `trees/{namespace:treeId}` | Protocol/recognition physical tree directory; selected Protocol roots from RPC |
| `GET /api/v1/roots`, `roots/{rootId-or-rootHash}` | Cached published Provenance certificates; explicit unavailable when no current source |
| `GET /api/v1/broadcasters`, `broadcasters/{operator}` | Canonical RPC registry directory; declarations do not imply endpoint health |
| `GET /api/v1/diagnostics`, `diagnostics/{scopeId}` | Sanitized Explorer synchronization and optional-source readiness |
| `POST /api/v1/search` | `{ "query": "0x..." }`; public hashes/addresses only; private material rejected |

List filters are `chainId`, `protocolAddress`, `tokenAddress`, `kind`, `namespace` (or `type`), `limit` (1–200; default 20), and opaque `cursor`. Unknown or repeated query fields fail. A page contains `items`, decimal-string `total`, `snapshot`, optional `unit` and `nextCursor`. Broadcaster directory uses `items`, `sources`, `status`, `totalBasis` and `directoryCompleteness` instead of a fabricated total.

Snapshots bind configured deployments, indexed block anchors, canonical operation count, local chain generation and recognition/projection revision. Cursors are HMAC authenticated with a locally generated random key, bind route/filter/limit and snapshot, and contain no service signing authority. Append-only synchronization preserves a cursor's original prefix. Chain rollback invalidates it; changed recognition or enrichment returns `CONFLICT`. Restart keeps the cursor key. Deleting the key invalidates old cursors. Browser pagination renews an expired snapshot and rebuilds cursors to the requested page, up to ten pages. Deeper sessions and chain reorganizations restart at page one with a notice. Recovery retries are bounded. Page numbers change only after successful responses; transport failures retain the last valid rows and total, and Retry obtains a fresh snapshot.

Ingestion follows RPC continuation even across empty pages. It stages a bounded 128-block window durably and publishes operations/projections and its prefix marker atomically only after exhausting the cursor and rechecking the canonical header. It resumes persisted cursors after restart. Eight upstream pages are processed per turn; backlog drains continuously, then the configured polling interval applies. Reorg reconciliation retains orphan operations and walks saved range anchors back to a common prefix. A failed or incomplete upstream response never publishes an empty completed range.

Protocol history works without Provenance or Broadcaster. Optional services run independently. Browser recognition reads use a public-only local cache and never make per-row upstream requests; missing or stale evidence is `provider_unavailable`, never recognized. Recognition cache entries retain the remote scope/generation/checkpoint. Unsupported Protocol operations can be enriched only by a Provenance operation with the exact same canonical locator, block number, scope and execution success. Returned evidence is recursively allowlisted, excluding private job identifiers and proof/witness material. Configure `issuerId` to additionally bind the expected Provenance issuer.

Published-root synchronization refreshes the current generation first, then backfills at most 200 historical certificates from one prior generation per pass. Historical progress is durable across restarts; cached totals describe the certificates fetched so far, and old signatures/generation labels are retained. Historical source failures do not suppress fresh current roots.

Recognition totals are present only when every indexed ordinary Note has fresh cached evidence; otherwise `recognizedNotes` is null and `recognizedNotesBasis` states unavailability. Upstream discovery is not used to choose service endpoints or deployment pins. Optional codeHash/startBlock pins are checked against the explicitly selected Protocol RPC deployment. Broadcaster endpoints declared in the registry are never fetched automatically.

Amounts appear only where public or publicly derivable from authenticated public funding. Credit commitments remain typed authority state. Private Note amounts, recipient ownership and private openings are never inferred. Historical unsupported indirect failures retain RPC's trace-attribution limitation. A fresh Explorer may show more unsupported chain records than the old Provenance-owned browser; counts describe its indexed prefix, not upstream recognition coverage.

SQLite supports one running Explorer per query database, enforced by a local file lock. Back up the database together with its `.cursor-key`; use a dedicated path, never a Provenance/RPC/Broadcaster database. HTTPS termination and authentication, if required for deployment, belong at the operator's reverse proxy.

Protocol tree metadata is bounded by the selected RPC root index. Active tree ID, tree count and contract `rootHistory` membership are omitted when that API does not establish them; empty or false values are not fabricated. A snapshot older than the RPC's retained root history can report the tree unavailable.

## Token metadata and amount display

`GET /api/v1/metadata` returns configured tokens and automatically discovered ERC-20 metadata, keyed by chain ID and address. `symbol` and `decimals` can be independently absent for automatically resolved tokens. An absent `decimals` means unknown; zero is a valid known precision. Configured entries take priority. Metadata requests read the local cache and do not call upstream services.

A background worker reads `symbol()` and `decimals()` through Protocol RPC at a finalized block. Successful entries refresh after 24 hours; incomplete or failed reads retry after five minutes and retain previous successful fields. The cache persists in the Explorer database. Discovery covers historical published operations and their funding transfers; native-asset addresses are not queried as ERC-20 contracts.

Transaction lists display at most six fractional digits, mark rounded values with `≈`, and show nonzero values below `0.000001` as a bound. Detail views retain full precision. Hovering an amount reveals its exact value; the adjacent copy action copies the exact ungrouped decimal number. Unknown precision remains explicitly labeled in raw units. All API amounts remain unchanged integer strings.
