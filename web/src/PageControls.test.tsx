import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { PageControls } from "./PageControls";
import { RecordTable } from "./RecordTable";
import { dictionaries } from "./i18n";
const t = (key: keyof typeof dictionaries.en) => dictionaries.en[key];
it("shows totals and disables unsafe navigation during loads or failed continuations", () => {
  const props = {
    page: 1,
    total: "205",
    loading: false,
    hasNext: true,
    previous: () => {},
    next: () => {},
    t,
  };
  const html = renderToStaticMarkup(<PageControls {...props} />);
  expect(html).toContain("205 records · Page 1");
  expect(html).toMatch(/<button disabled="">Previous/);
  expect(html).toContain("<button>Next");
  expect(renderToStaticMarkup(<PageControls {...props} loading />)).toMatch(
    /<button disabled="">Next/,
  );
  expect(
    renderToStaticMarkup(
      <PageControls {...props} page={2} error={{ code: "UNAVAILABLE" }} />,
    ),
  ).toMatch(/<button disabled="">Next/);
});
it("keeps occurrence references, networks and deployments separate in a responsive table", () => {
  const html = renderToStaticMarkup(
    <RecordTable
      rows={[
        {
          txHash: "tx",
          chainId: "97",
          protocolAddress: "0x" + "1".repeat(40),
          kind: "shield",
          timestamp: "101",
        },
      ]}
      t={t}
      reference={(row) => (
        <a href={"/transactions/" + row.txHash}>{row.txHash}</a>
      )}
      network={(row) => <span>network-{row.chainId}</span>}
    />,
  );
  expect(html).toContain('href="/transactions/tx"');
  expect(html).toContain("network-97");
  expect(html).toContain('data-label="Protocol"');
  expect(html).toContain(t("shield"));
});
