import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { SearchForm } from "./SearchForm";
import { dictionaries } from "./i18n";
it("renders seven real search categories and a matching placeholder with accessible labels", () => {
  for (const type of [
    "all",
    "transaction",
    "address",
    "note",
    "creditState",
    "nullifier",
    "root",
  ] as const) {
    const html = renderToStaticMarkup(
      <SearchForm
        query=""
        type={type}
        loading={false}
        t={(key) => dictionaries.en[key]}
        onQuery={() => {}}
        onType={() => {}}
        onSubmit={() => {}}
      />,
    );
    expect(html).toContain('role="combobox"');
    expect(html).not.toContain("<select");
    expect(html).toContain("Search type");
    expect(html).toContain('aria-expanded="false"');
  }
  const html = renderToStaticMarkup(
    <SearchForm
      query=""
      type="address"
      loading
      t={(key) => dictionaries.en[key]}
      onQuery={() => {}}
      onType={() => {}}
      onSubmit={() => {}}
    />,
  );
  expect(html).toContain("Enter a public address");
  expect(html).toContain('aria-busy="true"');
  expect(html).toContain("Searching…");
});
