import { it, expect } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { GrantHistory } from "./GrantHistory";
import { dictionaries } from "./i18n";
it("renders linked slot history including initial payments without inventing shared allocations",()=>{
 const data={chainId:"97",protocolAddress:"core",tokenAddress:"token",owner:"owner",grantId:"77",status:"2",historyStatus:"backfilling",activities:{items:[
  {kind:"revokeCredit",actions:["revokeCredit"],txHash:"tx3",timestamp:"100"},
  {kind:"creditDraw",actions:["creditDraw"],txHash:"tx2",timestamp:"90"},
  {kind:"createCredit",actions:["createCredit","creditDraw"],txHash:"tx1",timestamp:"80",slotFundingAmount:"7"},
 ]}};
 const html=renderToStaticMarkup(<MemoryRouter><GrantHistory data={data} t={k=>dictionaries.en[k]} address={v=><code>{v}</code>} transaction={r=><a href={`/transactions/${r.txHash}`}>{r.txHash}</a>} amount={v=><span>AMOUNT({v})</span>} href={(type,_row,id)=>`/${type}/${id}`}/></MemoryRouter>);
 expect(html).toContain("Revoked");expect(html).toContain("backfilled");expect(html).toContain('href="/transactions/tx1"');
 expect(html).toContain("Create credit · Credit draw");
 expect(html).toContain("Shared payer amount");expect(html.match(/AMOUNT\(7\)/g)).toHaveLength(1);
 expect(html).not.toContain("Source recognition");
});
