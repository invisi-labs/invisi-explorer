import type { ReactElement, ReactNode } from "react";
import { afterAll, beforeAll, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

const state = vi.hoisted(() => ({ render: vi.fn(), path: "/" }));
vi.mock("react-dom/client", () => ({
  createRoot: () => ({ render: state.render }),
}));
vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return {
    ...actual,
    BrowserRouter: ({ children }: { children: ReactNode }) => (
      <actual.MemoryRouter initialEntries={[state.path]}>
        {children}
      </actual.MemoryRouter>
    ),
  };
});
let application: ReactElement;
beforeAll(async () => {
  vi.stubGlobal("document", { getElementById: () => ({}) });
  vi.stubGlobal("navigator", { language: "en" });
  await import("./main");
  application = state.render.mock.calls[0][0] as ReactElement;
});
afterAll(() => vi.unstubAllGlobals());

it("separates Home and Transactions navigation and shows the home overview", () => {
  state.path = "/";
  const html = renderToStaticMarkup(application);
  const nav = html.match(/<nav[^>]*>([\s\S]*?)<\/nav>/)?.[1];
  const main = html.match(/<main>([\s\S]*?)<\/main>/)?.[1];
  expect(nav).toMatch(
    /aria-current="page"[^>]*href="\/"[^>]*>Home<\/a>|href="\/"[^>]*aria-current="page"[^>]*>Home<\/a>/,
  );
  expect(nav).toContain('href="/transactions"');
  expect(nav).toContain('href="/networks"');
  expect(main).toContain("<h1>InvisiPay Explorer</h1>");
  expect(main).toContain("Supported networks");
  expect(main).toContain("Latest transactions");
  expect(main).toContain("home-panels");
  expect(main).toContain("<form");
});
it("keeps Transactions active on the transaction list and details", () => {
  for (const path of ["/transactions", "/transactions/0x" + "1".repeat(64)]) {
    state.path = path;
    const html = renderToStaticMarkup(application);
    const nav = html.match(/<nav[^>]*>([\s\S]*?)<\/nav>/)?.[1];
    const entry = nav?.match(/<a\b[^>]*>Transactions<\/a>/)?.[0];
    expect(entry).toContain('href="/transactions"');
    expect(entry).toContain('aria-current="page"');
    if (path === "/transactions") {
      expect(html).toContain("Transaction filters");
      expect(html).toContain("Contains operation");
      expect(html).not.toContain("home-panels");
    }
  }
});
