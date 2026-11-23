# Service separation verification — 2026-11-24

The Explorer frontend and backend are in this repository. Provenance changes are
in the sibling `invisi-provenance` checkout. Neither set of changes is pushed or
published by this work.

| Check | Result |
| --- | --- |
| Explorer `go vet ./...` | Passed |
| Explorer `go test -race -timeout 5m ./...` | Passed |
| Explorer standalone `GOWORK=off go build` | Passed |
| Frontend `npm test` | 54 tests passed |
| Frontend `npm run build` | Passed |
| Linux/arm64 container build | Passed |
| Container CLI/init/settings/run + HTTP/static smoke | Passed; HTML, metadata, fonts and SVG/PNG assets served; dotfile/unknown API/missing asset paths returned 404 |
| Live opt-in read-only upstream integration | Passed against configured Protocol RPC, Provenance and Broadcaster using a fresh temporary Explorer database |
| Browser interaction | Chinese/English switch, sync heads, published roots; 320px viewport without horizontal overflow; local brand/network images loaded; no browser console errors observed |
| Provenance targeted race/vet/native build | Passed |
| Provenance existing issuer restart and legacy configuration | Passed; no operator files or database histories migrated/reset |
| Provenance full race suite | Passed with the repository-standard 30m timeout (ingestion 1287.628s); the initial 10m timeout was insufficient |

Controlled tests cover public-only DTO filtering; private transact amount
suppression; initial Credit payment gross/net separation; non-applicable Credit
management without Provenance; exact scope/operation/deployment binding;
JSON-RPC positional envelopes and sanitized typed errors; optional-service
outages and per-operation pending records; persisted staging, empty continuation,
reorg/orphan retention, cursor tampering/conflicts; current and historical
published roots; complete broadcaster block anchors and upstream page limits;
SQLite ownership and single-writer locks.

Live queries were read-only. No proof, quote, broadcast, payment, private tracking
or signer operations were sent. Live Explorer initial history is still backfilling
its own public query index, so current list counts cover its indexed prefix.
SQLite is single-process; local PostgreSQL integration was not run (no DSN).
