# Invisi Explorer CLI

Build the command with `make build`, then run `bin/invisi-explorer --help`.
The Go module targets Go 1.26.1 and uses urfave/cli/v3.

All commands accept the global `--dir PATH` option (default `./invisi-explorer`,
or `INVISI_EXPLORER_DATA_DIR`). This directory contains `config.json` and the
default `explorer.sqlite` database. Place global options before the command.

```sh
bin/invisi-explorer --dir ./data init
bin/invisi-explorer --dir ./data chain add \
  --chain-id 1 \
  --protocol-address 0x1111111111111111111111111111111111111111 \
  --rpc-url http://127.0.0.1:18545/v1/chains/1 \
  --name Ethereum --environment mainnet \
  --explorer-url https://etherscan.io \
  --start-block 0
bin/invisi-explorer --dir ./data token add \
  --chain-id 1 --address 0x2222222222222222222222222222222222222222 \
  --symbol USDC --decimals 6
bin/invisi-explorer --dir ./data settings show
bin/invisi-explorer --dir ./data run
```

`init` creates configuration with listen address `127.0.0.1:18670`, database
`explorer.sqlite`, poll interval `5s`, frontend directory `web/dist`, and empty
chain/token lists. `init --listen HOST:PORT` sets the initial listener. Existing
configuration is never overwritten. Configuration writes use owner-only mode
0600; subsequent updates replace the file atomically.

`run --listen HOST:PORT` overrides the saved listener for that invocation.
SIGINT and SIGTERM cancel the service context for graceful shutdown. Relative
database paths resolve against `--dir`. Relative frontend paths resolve against
the process working directory, so the default serves a project build at
`web/dist`. Use an absolute `--web-dir` when starting elsewhere.

| Command | Flags and behavior |
| --- | --- |
| `settings show` | Print the complete public configuration as JSON. |
| `settings set` | Set one or more of `--listen`, `--database`, `--poll-interval`, `--web-dir`. |
| `chain list` | Print explicit chain/protocol scopes as JSON. |
| `chain show` | Select one scope using `--chain-id` and `--protocol-address`. |
| `chain set` | Select a scope and update `--rpc-url`, `--provenance-url`, `--broadcaster-url`, `--name`, `--environment` or `--explorer-url`. Deployment/issuer pins and token configuration are preserved; restart to apply. |
| `chain add` | Require `--chain-id`, `--protocol-address`, `--rpc-url`; optional `--name`, `--environment`, `--explorer-url`, `--provenance-url`, `--broadcaster-url`, `--code-hash`, `--start-block`, `--issuer-id`. |
| `chain remove` | Require `--chain-id` and `--protocol-address`. Tokens must be removed before removing their last configured chain scope. |
| `token list` | Print configured token metadata as JSON. |
| `token add` | Require `--chain-id`, `--address`, `--symbol`, `--decimals`; optional `--name`. Decimals range from 0 through 255. |
| `token remove` | Require `--chain-id` and `--address`. |

Chain IDs are positive canonical uint64 decimal strings; addresses are 20-byte
`0x`-prefixed hexadecimal strings, and optional code hashes are 32-byte hex.
RPC, provenance, broadcaster, and explorer endpoints must be explicit HTTP/S
URLs without embedded credentials. Duplicate chain/protocol scopes and duplicate
chain/token addresses are rejected. Multiple protocols on a chain can be configured.

The explorer stores only public scope and display configuration. It does not
create or accept private keys, discover endpoints, or import sibling services'
internal packages. Add provenance and broadcaster URLs only when those public
services should be queried. An optional issuer ID is operator-provided metadata.

All configuration updates apply after restarting the process. Scope identities and deployment/issuer pins are not changed by `chain set`; changing deployments requires an explicitly configured scope and a suitable independent query index.
