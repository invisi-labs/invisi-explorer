import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { NetworkDirectory, HomeNetworks } from "./NetworkPanels";
import { AddressReference } from "./AddressReference";
import { OverviewStats } from "./HomePanels";
import { TransactionFilters } from "./TransactionFilters";
import { networkGroups } from "./networkDirectory";
import { dictionaries } from "./i18n";
const a = "0x" + "a".repeat(40),
  b = "0x" + "b".repeat(40);
const networks = [
  {
    chainId: "97",
    protocolAddress: a,
    name: "BNB Smart Chain Testnet",
    environment: "testnet",
    startBlock: "42",
    codeHash: "0x" + "f".repeat(64),
  },
  { chainId: "97", protocolAddress: b, name: "BNB Smart Chain Testnet" },
];
const groups = networkGroups(networks, [
  {
    chainId: "97",
    protocolAddress: a,
    status: "ready",
    indexedHead: { blockNumber: "100" },
    finality: {
      finalized: { status: "unavailable", block: { blockNumber: "999" } },
    },
  },
  {
    chainId: "97",
    protocolAddress: b,
    status: "unavailable",
    indexedHead: { blockNumber: "80" },
  },
]);
const t = (key: keyof typeof dictionaries.en) => dictionaries.en[key];
it("shows one supported network count and links the overview to dedicated lists", () => {
  const html = renderToStaticMarkup(
    <MemoryRouter>
      <OverviewStats
        networkCount={groups.length}
        totals={{ transactions: "205", notes: "300", grants: "25" }}
        t={t}
      />
      <HomeNetworks
        groups={groups}
        t={t}
        network={(row) => <span>{row.name}</span>}
        address={(value, row) => (
          <AddressReference
            value={value}
            kind="address"
            compact
            explorerUrl="https://testnet.bscscan.com"
            internalHref={"/addresses/" + value + "?chainId=" + row.chainId}
            t={t}
          />
        )}
      />
    </MemoryRouter>,
  );
  expect(html).toContain("Supported networks</span><strong>1</strong>");
  expect(html).toContain('href="/transactions"');
  expect(html).not.toContain("Protocol deployments: 2");
  expect(html).toContain('href="/networks?chainId=97"');
});
it("keeps deployment status and heights separate and does not claim code verification", () => {
  const html = renderToStaticMarkup(
    <MemoryRouter>
      <NetworkDirectory
        groups={groups}
        t={t}
        network={(row) => <span>{row.name}</span>}
        address={(value) => <code>{value}</code>}
      />
    </MemoryRouter>,
  );
  const panels = [...html.matchAll(/<article[^>]*>([\s\S]*?)<\/article>/g)].map(
    (match) => match[1],
  );
  expect(panels).toHaveLength(2);
  expect(panels[0]).toContain("Operational");
  expect(panels[0]).toContain("100");
  expect(panels[0]).not.toContain("999");
  expect(panels[1]).toContain("Synchronization unavailable");
  expect(panels[1]).toContain("80");
  expect(html).toContain("do not establish source-code verification");
  expect(html).toContain("42");
});
it("uses styled filters with network icons and only enables deployment selection after a network", () => {
  const render = (chainId: string) =>
    renderToStaticMarkup(
      <TransactionFilters
        networks={networks}
        value={{ chainId, protocolAddress: "", kind: "", tokenAddress: "" }}
        onChange={() => {}}
        t={t}
      />,
    );
  expect(render("")).toMatch(
    /disabled=""[^>]*role="combobox"[^>]*aria-label="Protocol deployment"/,
  );
  expect(render("97")).toContain("/assets/networks/smartchain.png");
  expect(render("97")).not.toContain("<select");
  expect(render("97")).toContain("complete transaction is shown");
});
it("keeps checksum token names selected when filter URLs use lowercase addresses", () => {
  const html = renderToStaticMarkup(
    <TransactionFilters
      networks={networks}
      tokens={[{ chainId: "97", address: b.toUpperCase(), symbol: "USDT" }]}
      value={{ chainId: "97", protocolAddress: a, kind: "", tokenAddress: b }}
      onChange={() => {}}
      t={t}
    />,
  );
  expect(html).toContain("USDT ·");
});

it("turns home networks into contract entry points and hides normal status", () => {
  const render = (items: ReturnType<typeof networkGroups>) =>
    renderToStaticMarkup(
      <MemoryRouter>
        <HomeNetworks
          groups={items}
          t={t}
          network={(row) => <span>{row.name}</span>}
          address={(value, row) => (
            <AddressReference
              value={value}
              kind="address"
              compact
              explorerUrl="https://testnet.bscscan.com"
              internalHref={"/addresses/" + value + "?chainId=" + row.chainId}
              t={t}
            />
          )}
        />
      </MemoryRouter>,
    );
  const normal = render(
    networkGroups(
      [networks[0]],
      [{ chainId: "97", protocolAddress: a, status: "ready" }],
    ),
  );
  expect(normal).toContain(a);
  expect(normal).toContain('aria-label="Copy full value"');
  expect(normal).toContain("https://testnet.bscscan.com/address/" + a);
  expect(normal).toContain("/addresses/" + a + "?chainId=97");
  expect(normal).toContain("View contracts");
  expect(normal).toContain("/transactions?chainId=97");
  expect(normal).not.toContain("Operational");
  expect(normal).not.toContain("protocol deployments");
  expect(normal).not.toContain('role="status"');
  const unknown = render(networkGroups([networks[0]]));
  expect(unknown).not.toContain("Not checked");
  expect(unknown).not.toContain('role="status"');
  const failed = render(groups);
  expect(failed).toContain('role="status"');
  expect(failed).toContain("Synchronization issue");
});
