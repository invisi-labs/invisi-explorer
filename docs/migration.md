# Move Explorer out of Provenance

The new service preserves the `/api/v1/*` browser contract while replacing direct
Provenance database reads with its own rebuildable public read model and explicit
JSON-RPC upstreams. The explorer hostname and browser routes can remain unchanged.

## Prepare Explorer

1. Build the Go command and frontend in this repository (`make build`, `make web`).
2. Initialize a **new Explorer** data directory; configure scopes, public upstream
   URLs and token metadata using `cammand.md`. The default port is 18670.
3. Pin the intended Core address and, where configured, its runtime code hash and
   deployment start block. A JSON-RPC discovery response is not independent trust.
4. Run Explorer and allow public history to backfill. Indexing is asynchronous;
   displayed counts cover the indexed prefix rather than unseen upstream history.
   Protocol RPC performs bounded replay, so large histories may take time to scan.
5. Verify metadata, transactions, Notes/Credit, grants, roots, search and sync state
   against the selected upstreams. Verify the configured optional Provenance and
   Broadcaster statuses independently from chain execution.
6. Point the Explorer reverse proxy to the new service and serve the built frontend
   from its `webDir`. Configure the frontend dev server to use this backend.

For an explicit read-only live wire-contract check against a separately configured
service directory:

```sh
EXPLORER_LIVE_CONFIG_DIR=/absolute/path/to/explorer-data \
  go test -count=1 -timeout 2m -v ./internal/integration
```

This test opens a fresh temporary Explorer database and reads public machine APIs;
it never opens the configured query database or any issuer database, and never
submits proofs, quotes or transactions. Normal tests do not contact live services.

## Upgrade Provenance

Keep the original issuer service directory, database, signing/cursor keys, rules,
verifier pins, provider configuration and published history in place. The service
continues to own ingestion, independent chain authentication, screening, proof
jobs, recognition trees and signed publication.

The machine-only service retains `/v1` discovery and `/v1/chains/{chainId}` JSON-RPC.
Its former `/api/v1/*`, frontend and static asset routes return 404. Browser-specific
CLI options are removed. Legacy display configuration is retained when loading and
saving existing configuration, but no longer enables browser hosting. Do not run
`init` against a different Provenance directory as part of this migration.

No database transfer or issuer reset is needed. Explorer never receives private
signer material. Stop the old web development process after copying the frontend;
start the new one in `invisi-explorer/web`.
