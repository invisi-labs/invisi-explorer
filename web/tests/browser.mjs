// Exercise the real browser and same-origin API without production mock data.
import { createRequire } from "node:module";
import assert from "node:assert/strict";
const require = createRequire(import.meta.url);
let chromium;
try {
  ({ chromium } = require("playwright"));
} catch {
  ({ chromium } = require("/tmp/invisi-ui-tools/node_modules/playwright"));
}
const browser = await chromium.launch({
  executablePath:
    process.env.CHROME_PATH ??
    "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
  headless: true,
});
const base = process.env.EXPLORER_URL ?? "http://127.0.0.1:5173";
const routes = [
  "/",
  "/transactions",
  "/notes",
  "/credit-states",
  "/grants",
  "/nullifiers",
  "/trees",
  "/roots",
  "/diagnostics",
  "/broadcasters",
];
let checked = 0;
try {
  for (const width of [320, 390, 1440]) {
    const page = await browser.newPage({ viewport: { width, height: 900 } });
    const errors = [];
    page.on("pageerror", (e) => errors.push(e.message));
    for (const language of ["en", "zh-Hans", "zh-Hant"]) {
      for (const route of routes) {
        await page.goto(base + route);
        await page.locator(".language-trigger").click();
        await page.locator(`[data-language="${language}"]`).click();
        assert.equal(await page.locator("html").getAttribute("lang"), language);
        await page.locator("h1").waitFor();
        assert.equal(
          await page.evaluate(
            () => document.documentElement.scrollWidth > innerWidth,
          ),
          false,
          `${route}/${language}/${width} overflow`,
        );
        checked++;
      }
      await page.goto(base + "/");
      let searches = 0;
      page.on("request", (r) => {
        if (r.url().includes("/search")) searches++;
      });
      const field = page.locator(".search input");
      await field.fill("0isecret-material");
      await page.locator(".search button").click();
      await page.locator(".search-area p[role=alert]").waitFor();
      assert.equal(searches, 0);
      assert.equal(new URL(page.url()).search, "");
      assert.equal(
        await page.evaluate(() =>
          Object.keys(localStorage).some((k) => k !== "invisi.language"),
        ),
        false,
      );
    }
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log(
    `PASS ${checked} real-browser route/language/viewport checks; local secret rejection, no query persistence, no page errors`,
  );
} finally {
  await browser.close();
}
