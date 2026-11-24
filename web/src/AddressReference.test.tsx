import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { AddressReference } from "./AddressReference";
import { dictionaries } from "./i18n";
const hash="0x"+"a".repeat(64);
const t=(key:keyof typeof dictionaries.en)=>dictionaries.en[key];
it("keeps internal transaction navigation, exact copying and the network explorer link together",()=>{
 const html=renderToStaticMarkup(<MemoryRouter><AddressReference value={hash} kind="tx" explorerUrl="https://testnet.bscscan.com" internalHref={`/transactions/${hash}?chainId=97`} t={t}/></MemoryRouter>);
 expect(html).toContain(`href="https://testnet.bscscan.com/tx/${hash}"`);
 expect(html).toContain(`href="/transactions/${hash}?chainId=97"`);
 expect(html).toContain('rel="noopener noreferrer"');
 expect(html).toContain('aria-label="Copy full value"');
});
it("does not create explorer links for unsafe URLs or malformed identifiers",()=>{
 for(const explorerUrl of ['javascript:alert(1)','http://unsafe.example','https://user:password@example.com']){
  const html=renderToStaticMarkup(<AddressReference value={hash} kind="tx" explorerUrl={explorerUrl} t={t}/>);
  expect(html).not.toContain('target="_blank"');
 }
 const html=renderToStaticMarkup(<AddressReference value="0x123" kind="tx" explorerUrl="https://testnet.bscscan.com" t={t}/>);
 expect(html).not.toContain('target="_blank"');
});
