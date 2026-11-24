import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { AmountDisplay } from "./AmountDisplay";
import { dictionaries } from "./i18n";

it("shows rounded list amounts with the exact value and accessible copy action", () => {
 const html=renderToStaticMarkup(<AmountDisplay value="2070045901362772257" decimals={18} symbol="iUSD" compact t={key=>dictionaries.en[key]}/>);
 expect(html).toContain("≈2.070046");
 expect(html).toContain('title="2.070045901362772257 iUSD"');
 expect(html).toContain('aria-label="Copy full value"');
 expect(html).not.toContain("Raw units");
});
it("keeps raw units explicit and omits copying for missing amounts", () => {
 const html=renderToStaticMarkup(<AmountDisplay value="1000000000000000000" symbol="0x2056…6d60" compact t={key=>dictionaries.en[key]}/>);
 expect(html).toContain(dictionaries.en.rawUnits);
 expect(html).toContain('title="1000000000000000000 0x2056…6d60');
 const missing=renderToStaticMarkup(<AmountDisplay value={undefined} symbol="iUSD" decimals={18} t={key=>dictionaries.en[key]}/>);
 expect(missing).toContain("—");
 expect(missing).not.toContain("<button");
});
