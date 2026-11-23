# Invisi Explorer

React + TypeScript + Vite public Explorer. The visual system follows the InvisiPay website and documentation; production pages never embed sample records. The frontend and its backend are owned by this repository.

The Transactions homepage at `/` combines search, protocol statistics and the transaction list. Navigation has one Transactions entry; `/transactions` redirects to `/`, while `/transactions/{txHash}` remains the transaction detail route. Backend overview and transaction API endpoints remain separate.

```sh
npm ci
npm run dev
npm run build
npm test
node tests/browser.mjs
```

Vite proxies `/api` to `http://127.0.0.1:18670`. To preview the local UI with live public data, run `VITE_API_PROXY_TARGET=https://explorer.invisipay.fi npm run dev`. Production hosting must serve `dist/`, fall back to `index.html` for browser routes, and forward `/api/v1/*` to the standalone Explorer backend on the same origin. Keep API caching disabled and exclude request bodies and search terms from server logs. No analytics or external fonts are loaded.

Browser tests use installed `playwright`, falling back to the development machine's `/tmp/invisi-ui-tools/node_modules/playwright`. `CHROME_PATH` overrides the Chrome executable; `EXPLORER_URL` overrides the base URL. They run actual API requests, without injecting fake production data. An unconfigured backend is rendered as unavailable rather than empty or recognized.

REST collections use `{items,total,nextCursor?,snapshot?}` and opaque cursor pagination. Search uses `POST /api/v1/search` with `{query}`. Only the manually chosen language is persisted under `invisi.language`; search terms never enter browser URLs or storage. Details retain chain/protocol scope in their public URLs. Network labels display the configured name without appending testnet/mainnet badges. Metadata comes from `/api/v1/metadata`; unknown networks and assets never receive guessed explorer links or decimals.

Public API responses are the privacy boundary. Return only allowlisted public fields: the expandable public JSON is intentionally not a private operator console. Secret-wrapper rejection cannot distinguish arbitrary 32-byte secrets from syntactically valid public identifiers.

Synchronization diagnostics have a single navigation entry labeled “Sync Status” (同步状态 / 同步狀態) at `/diagnostics`, using the existing list/table layout with one row per network and columns for service status, indexed head, chain head, finalized head and backend `updatedAt` (Unix seconds rendered in the browser timezone). This endpoint returns `{items}` without a collection total or cursor; it uses neither transaction hash/time columns nor pagination. The displayed backend update time is not the browser fetch time. Empty, unchecked, unavailable and unknown states are explicit. The footer displays `Invisi Labs` and the browser timezone; it has no duplicate synchronization dialog. Synchronization diagnostics display indexed, latest and finalized positions separately. Pending native finality is distinct from missing Source evidence or execution failure; unsupported, unavailable and conflicting finality have explicit English, Simplified Chinese and Traditional Chinese labels.

## Contributing

See the [project development guide](../README.md) for pull requests, relevant checks and handling sensitive data.

## Contract funding and execution

Transaction details show each Core operation's observed funding sources, Core funding payer, Core caller (which can be any third-party Router), transaction submitter, service-fee recipient, selected asset transfers, and authenticated Core call ancestry. There is no Router whitelist. A transaction submitter is not automatically the funder or a registered Broadcaster. Intermediate DEX pools are not labeled players merely because they transfer the settlement token.

Source candidates are observations at the selected flow boundary, not beneficial ownership or complete historical provenance. Asset transfers retain their own token units. Selected evidence can omit later refunds, so source quantities are not displayed as final wallet spending. Contract inventory and ambiguous flows retain explicit unresolved attribution. Failed and orphaned operations never display completed funding roles.

The optional `executionContext` comes from the signed canonical transaction and certified trace. Type 4 authorizations are shown as signed intents, not proof of effective installation. Transactions using an existing delegation may have no new authorization list; a type 2 transaction must not be guessed to be an ordinary EOA or an EIP-7702 execution. Historical records without the new context remain readable and explicitly lack these details. The metadata changes neither recognition inputs, evidence digests nor circuits; old blocks are not automatically reindexed.

Recognition reasons include a readable explanation for known codes and retain the original reason code, including unknown future codes. Raw public JSON remains available.

To check the isolated development-only funding UI harness (no production data or API mocks):

```sh
npm run dev -- --port 5183 --strictPort
EXPLORER_URL=http://127.0.0.1:5183 node tests/funding-browser.mjs
```

It covers English, Simplified and Traditional Chinese at 320, 390 and 1440 pixels, with type 4, ambiguous, historical, failed and orphaned records. The harness is excluded from the production entry graph.
