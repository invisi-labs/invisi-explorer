import { describe, expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { DiagnosticsPanel, type DiagnosticRecord } from "./DiagnosticsPanel";
import { dictionaries, type Key } from "./i18n";
import { fullTime } from "./public";

const record: DiagnosticRecord = {
  chainId: "97",
  status: "ready",
  indexedHead: { blockNumber: "144182203" },
  chainHead: { blockNumber: "144182205" },
  finality: {
    finalized: { status: "available", block: { blockNumber: "144182202" } },
  },
  updatedAt: "1795336448",
};
function render(items: DiagnosticRecord[], dictionary = dictionaries.en) {
  return renderToStaticMarkup(
    <DiagnosticsPanel
      items={items}
      t={(key: Key) => dictionary[key]}
      network={(row) => <span>Network {row.chainId}</span>}
    />,
  );
}

describe("public diagnostics", () => {
  for (const [language, dictionary] of Object.entries(dictionaries)) {
    it(`shows backend fields in the network list in ${language}`, () => {
      const html = render([record], dictionary);
      for (const key of [
        "sync",
        "indexed",
        "chainHead",
        "confirmed",
        "backendUpdated",
        "syncReady",
      ] as const)
        expect(html).toContain(dictionary[key]);
      for (const value of [
        "Network 97",
        "144182203",
        "144182205",
        "144182202",
        fullTime(record.updatedAt),
      ])
        expect(html).toContain(value);
      expect(html).toContain("<table");
      expect(html.match(/<th scope="col"/g)).toHaveLength(6);
      expect(html).not.toContain("<article");
      for (const key of [
        "network",
        "sync",
        "indexed",
        "chainHead",
        "confirmed",
        "backendUpdated",
      ] as const)
        expect(html).toContain(`data-label="${dictionary[key]}"`);
      expect(html).not.toContain("pagination");
      expect(html).not.toContain(dictionary.refreshed);
    });
  }
  it("keeps each network and its backend values together in one table row", () => {
    const html = render([
      record,
      {
        chainId: "1",
        status: "unavailable",
        reason: "CHAIN_UNAVAILABLE",
        indexedHead: { blockNumber: "21000001" },
      },
    ]);
    const rows = [...html.matchAll(/<tr>([\s\S]*?)<\/tr>/g)].map(
      (match) => match[1],
    );
    expect(rows).toHaveLength(3);
    expect(rows[1]).toContain("Network 97");
    expect(rows[1]).toContain("144182203");
    expect(rows[1]).not.toContain("21000001");
    expect(rows[2]).toContain("Network 1");
    expect(rows[2]).toContain("21000001");
    expect(rows[2]).toContain("CHAIN_UNAVAILABLE");
    expect(rows[2]).not.toContain("144182203");
  });
  for (const status of ["unsupported", "unavailable", "inconsistent"]) {
    it(`hides old finalized height when ${status}`, () => {
      const html = render([
        {
          ...record,
          finality: {
            finalized: { status, block: { blockNumber: "99999999" } },
          },
        },
      ]);
      const key = {
        unsupported: "finalityUnsupported",
        unavailable: "finalityUnavailable",
        inconsistent: "finalityConflict",
      }[status]!;
      expect(html).toContain(dictionaries.en[key as Key]);
      expect(html).not.toContain("99999999");
    });
  }
  it("distinguishes unavailable, unchecked and unknown states and escapes reasons", () => {
    const html = render([
      { chainId: "97", status: "unavailable", reason: "<script>bad</script>" },
      { chainId: "1", status: "not_checked" },
      { chainId: "2", status: "FUTURE_STATUS" },
    ]);
    expect(html).toContain(dictionaries.en.syncUnavailable);
    expect(html).toContain(dictionaries.en.syncNotChecked);
    expect(html).toContain(dictionaries.en.unknown);
    expect(html).toContain("FUTURE_STATUS");
    expect(html).toContain("&lt;script&gt;bad&lt;/script&gt;");
    expect(html).not.toContain("<script>");
    expect(html).not.toContain(dictionaries.en.syncReady);
    expect(html).not.toContain("1970-");
  });
  it("handles missing fields without inventing a finalized height or timestamp", () => {
    const html = render([
      {
        chainId: "97",
        status: "not_checked",
        finality: { finalized: { status: "available" } },
      },
    ]);
    expect(html).toContain(dictionaries.en.unknown);
    expect(html).toContain("—");
    expect(html).not.toContain("undefined");
    expect(html).not.toContain("Invalid Date");
  });
  it("shows an empty state when there are no configured networks", () => {
    expect(render([])).toContain(dictionaries.en.empty);
    expect(render([])).not.toContain("<dl");
  });
});
