import type { ReactElement, ReactNode } from "react";
import { afterAll, beforeAll, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

const state = vi.hoisted(() => ({ render: vi.fn(), path: "/" }));
vi.mock("react-dom/client", () => ({ createRoot: () => ({ render: state.render }) }));
vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return {
    ...actual,
    BrowserRouter: ({ children }: { children: ReactNode }) => (
      <actual.MemoryRouter initialEntries={[state.path]}>{children}</actual.MemoryRouter>
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

it("keeps the homepage content under one Transactions navigation entry", () => {
  state.path = "/";
  const html = renderToStaticMarkup(application);
  const nav = html.match(/<nav[^>]*>([\s\S]*?)<\/nav>/)?.[1];
  const main = html.match(/<main>([\s\S]*?)<\/main>/)?.[1];
  expect(nav).not.toContain("Overview");
  expect(nav?.match(/>Transactions<\/a>/g)).toHaveLength(1);
  const entry = nav?.match(/<a\b[^>]*>Transactions<\/a>/)?.[0];
  expect(entry).toContain('href="/"');
  expect(entry).toContain('aria-current="page"');
  expect(nav).not.toContain('href="/transactions"');
  expect(main).toContain("<h1>Transactions</h1>");
  expect(main).toContain('class="stats"');
  expect(main).toContain("<form");
  expect(main).toContain("Recent protocol activity");
});

it("keeps Transactions active when viewing an individual transaction", () => {
  state.path = "/transactions/0x" + "1".repeat(64);
  const html = renderToStaticMarkup(application);
  const nav = html.match(/<nav[^>]*>([\s\S]*?)<\/nav>/)?.[1];
  const entry = nav?.match(/<a\b[^>]*>Transactions<\/a>/)?.[0];
  expect(entry).toContain('href="/"');
  expect(entry).toContain('aria-current="page"');
  expect(nav).not.toContain("Overview");
});
