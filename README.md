# Invisi Explorer

Independent public explorer for Invisi Protocol. The Go backend owns the browser API and its rebuildable public query index; the React/TypeScript frontend is served by this service. Protocol RPC supplies chain facts, Provenance supplies source-recognition decisions and evidence, and Broadcaster supplies optional public service information.

Explorer neither shares a database with these services nor receives their signing keys, proof witnesses, private tracking IDs or submission journals. Provenance remains responsible for independent chain authentication, screening, proofs and signed recognition checkpoints.

```text
Browser → Explorer frontend / browser API
                    ├── Protocol RPC (chain facts)
                    ├── Provenance JSON-RPC (recognition and evidence)
                    └── Broadcaster JSON-RPC (optional public service information)
```

## Development

Requirements: the Go version in `go.mod` and Node 24. The service uses the same Go / `net/http` / `urfave/cli/v3` conventions as the sibling services; the UI uses React 19, TypeScript and Vite.

```sh
go build -trimpath -o bin/invisi-explorer ./cmd/invisi-explorer
bin/invisi-explorer --help
cd web
npm ci
npm run dev
```

Vite serves the UI at `http://127.0.0.1:5173` and proxies `/api` to the standalone backend at `http://127.0.0.1:18670`. `VITE_API_PROXY_TARGET` can select another **Explorer backend**. It is not a Protocol RPC or Provenance endpoint. Public upstreams are configured on the backend, keeping the browser API same-origin.

See [cammand.md](cammand.md) for initialization, upstream scope configuration and production operation, [api.md](api.md) for the public API, and [web/README.md](web/README.md) for frontend behavior and tests.

## Verification

```sh
go vet ./...
go test -race -timeout 10m ./...
go build -trimpath -o bin/invisi-explorer ./cmd/invisi-explorer
cd web
npm ci
npm test
npm run build
```

## Service separation

The browser sources were moved from `invisi-provenance/web`. Explorer now owns `/api/v1/*` and static hosting. Provenance continues to serve `/v1` discovery and `/v1/chains/{chainId}` JSON-RPC; its old browser routes are removed.

Deploy Explorer and point the explorer hostname/reverse proxy to its listener. Configure its upstream scopes against the existing RPC and Provenance services. See the [migration guide](docs/migration.md) for the cutover sequence and optional live checks. Retain Provenance's database, issuer identity, cursor/signing keys and published history in their existing service directory. Do not copy them into Explorer or reinitialize the issuer during migration. Explorer uses its own database and can rebuild public query data from its upstreams.

Chain execution, provenance recognition and broadcast acceptance remain independent. A Provenance outage must not turn an available transaction into a failed execution or an unrecognized verdict. Broadcaster availability is optional. Public Note/Credit state and amounts retain their separate domains, and Explorer does not infer decrypted owners, private balances or commitment-to-nullifier associations.

## Container

`docker build -t invisi-explorer:local .` builds both UI and service. The runtime is unprivileged, includes CA certificates, and listens on port 18670 when run with its default command. Mount a writable, separately initialized Explorer data directory at `/var/lib/invisi-explorer`; configure static files at `/opt/invisi/explorer`. Initialization and upstream configuration remain explicit, as in the other Invisi services.


For example, after building the image:

```sh
docker volume create invisi-explorer-data
docker run --rm -v invisi-explorer-data:/var/lib/invisi-explorer invisi-explorer:local init
docker run --rm -v invisi-explorer-data:/var/lib/invisi-explorer invisi-explorer:local settings set --web-dir /opt/invisi/explorer
# Add explicit chain/token configuration with the CLI before running.
docker run --rm -p 18670:18670 -v invisi-explorer-data:/var/lib/invisi-explorer invisi-explorer:local
```
