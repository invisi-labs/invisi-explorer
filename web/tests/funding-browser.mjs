// Exercise only the isolated development harness; no production API fixtures are injected.
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
let checks = 0;
try {
  for (const width of [320, 390, 1440]) {
    const page = await browser.newPage({ viewport: { width, height: 1000 } });
    const errors = [];
    page.on("pageerror", (error) => errors.push(error.message));
    await page.goto(base + "/tests/funding-preview.html");
    for (const language of ["en", "zh-Hans", "zh-Hant"]) {
      await page.getByLabel("Language").selectOption(language);
      for (const scenario of [
        "7702",
        "ambiguous",
        "legacy",
        "failed",
        "orphaned",
      ]) {
        await page.getByLabel("Scenario").selectOption(scenario);
        await page.locator(".funding-panel").waitFor();
        assert.equal(
          await page.evaluate(
            () => document.documentElement.scrollWidth > innerWidth,
          ),
          false,
          `${width}/${language}/${scenario} overflows`,
        );
        assert.ok(await page.locator(".recognition-reason").innerText());
        if (scenario === "7702") {
          assert.equal(
            await page.locator(".funding-table tbody tr").count(),
            2,
          );
          const details = page.locator(".funding-calls");
          await details.first().locator("summary").click();
          await details.last().locator("summary").click();
          assert.ok(
            (await page.locator(".funding-panel").innerText()).includes(
              "EIP-7702",
            ),
          );
          if (width === 1440 && language === "zh-Hans")
            await page.screenshot({
              path: "/tmp/explorer-funding-desktop.png",
              fullPage: true,
            });
          if (width === 320 && language === "zh-Hans")
            await page.screenshot({
              path: "/tmp/explorer-funding-mobile.png",
              fullPage: true,
            });
        }
        if (["failed", "orphaned"].includes(scenario))
          assert.equal(await page.locator(".funding-table").count(), 0);
        checks++;
      }
    }
    assert.deepEqual(errors, []);
    await page.close();
  }
  console.log(`${checks} funding browser scenarios passed`);
} finally {
  await browser.close();
}
