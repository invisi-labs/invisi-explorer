import type { ReactElement, ReactNode } from "react";
import { afterAll, beforeAll, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";

const { render } = vi.hoisted(() => ({ render: vi.fn() }));
vi.mock("react-dom/client", () => ({ createRoot: () => ({ render }) }));
vi.mock("react-router-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-router-dom")>();
  return {
    ...actual,
    BrowserRouter: ({ children }: { children: ReactNode }) => (
      <actual.MemoryRouter initialEntries={["/networks"]}>
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
  application = render.mock.calls[0][0] as ReactElement;
});
afterAll(() => vi.unstubAllGlobals());

it("replaces the standalone sync page with Networks & contracts", () => {
  const html = renderToStaticMarkup(application);
  const main = html.match(/<main>([\s\S]*?)<\/main>/)?.[1];
  expect(main).toContain("Networks &amp; contracts");
  expect(main).toContain("Network");
  expect(main).not.toContain("<form");
  expect(main).not.toContain("pagination");
  expect(html).not.toContain('href="/diagnostics"');
  expect(html).toContain('href="/networks"');
});
it("keeps network discovery out of settings and footer dialogs", () => {
  const html = renderToStaticMarkup(application);
  const footer = html.match(/<footer\b[^>]*>([\s\S]*?)<\/footer>/)?.[1];
  expect(footer).toContain("Invisi Labs");
  expect(footer).not.toContain("<button");
  expect(html).not.toContain('role="dialog"');
});
