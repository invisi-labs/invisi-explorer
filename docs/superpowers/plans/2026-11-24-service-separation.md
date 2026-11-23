# Explorer Service Separation Implementation Plan

> **For agentic workers:** Use independently scoped parallel workers for the Explorer backend and Provenance removal; the parent migrates the frontend, documentation and integration, then reviews the complete change.

**Goal:** Deliver standalone invisi-explorer frontend/backend and JSON-RPC-only invisi-provenance.

**Architecture:** Explorer reads the three services through public machine APIs and owns browser REST, query projections and presentation. Provenance remains an independent recognition authority. No service database sharing or Explorer proxy to the old Provenance REST is permitted.

**Tech Stack:** Go 1.26.1, net/http, urfave/cli v3; React 19, TypeScript, Vite, Vitest.

**Spec:** docs/superpowers/specs/2026-11-24-service-separation.md

## Global Constraints
- Preserve the existing frontend and public JSON contracts where practical.
- Do not move or reset any initialized issuer identity/database/history.
- Keep chain-fact, recognition and broadcast decisions separate.
- Unknown/unavailable dependencies cannot become empty or recognized success.
- Work in the explicit existing project directories; no worktree is necessary for these clean checkouts and no unrelated edits are present.

## Review Focus
- Independent upstream lag, mismatched scopes/block hashes, reorgs and stale cursors.
- Missing optional Provenance/Broadcaster endpoints and partial upstream outages.
- Public-only output allowlists and rejection of known secret search prefixes.
- Startup and reload without existing service databases or private keys.
- Removal of all live Explorer/static routes from Provenance without affecting machine APIs or issuer data.

### Task 1: Independent Explorer Go backend
- [x] Add public-machine clients and owned projection/aggregation backend; test against controlled JSON-RPC servers, including outages, paging and reorgs.
- [x] Add independent init/run/config CLI, HTTP/static serving and browser route compatibility.
- [x] Verify go test -race ./..., go vet ./..., and build.

### Task 2: Provenance becomes machine-only
- [x] Remove Explorer wiring, REST/static serving and web CLI hosting options.
- [x] Preserve machine JSON-RPC, recognition storage, authentication and production configuration compatibility.
- [x] Update tests/docs; verify API/CLI tests and full relevant Go tests.

### Task 3: Frontend and project integration
- [x] Copy tracked frontend source/assets/tests before removing old source.
- [x] Rename package/config/docs; connect Vite to Explorer backend.
- [x] Supply development/production CLI commands and dependency configuration examples.
- [x] Build/test the UI and run the new service with controlled and available live upstreams.

### Task 4: Whole-change verification
- [x] Review service boundaries, contracts, migration instructions and failure semantics.
- [x] Run tests/builds and inspect both Git diffs. Do not publish or push.


## Implementation and verification notes

- Both existing clean project checkouts were used at the user-requested paths; no worktree was created or moved.
- Frontend sources, assets and tests moved before removing old tracked files. Standalone test fixtures no longer import old service files.
- Independent review found and fixed public machine wire differences, blocking optional calls, missing scope binding, private-field exposure and broadcaster anchor shape; deterministic regressions cover each.
- Additional preserved business contracts: initial Credit payment amount separation, private transact amount suppression, known-secret search rejection and credit authority changes rendering not-applicable without depending on an issuer.
- Explorer owns its SQLite schema/application identity and writer lock; it refuses an existing foreign database. No service signing key or private proof/tracking state was transferred.
- Current and historical published roots are cached through public RPC with durable bounded history progress. Per-operation INDEX_PENDING/NOT_FOUND is separated from endpoint outages to prevent cache starvation.
- Explorer Go race suite/vet/build, frontend 54 tests/build, Linux/arm64 container build and HTTP/static checks passed. Live read-only contracts passed for configured Protocol RPC, Provenance and Broadcaster.
- Provenance targeted race checks, native build/vet, machine-only HTTP/restart/legacy-issuer preservation checks passed. Full race suite first hit its too-short 10m timeout inside unchanged dense ingest recovery code, with no assertion/race failure; retried using the existing 30m project convention. The standard 30m full race run passed (ingest 1287.628s, all packages green).
- Local PostgreSQL integration was not run (no DSN); Provenance CI retains its PostgreSQL service.
- Live Explorer initial public-history backfill remains asynchronous and limited by Protocol RPC replay throughput; local counts represent its indexed prefix. Query DB is SQLite/single-writer.

- Final protocol-tree contract regression also passed: active-tree identity/count and contract root-history membership remain absent when the current Protocol RPC does not provide them; the adapter does not fabricate false/true values.
