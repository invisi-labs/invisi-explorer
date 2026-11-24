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
 expect(html).toContain("<h3>Create credit</h3>");
 expect(html).not.toContain("Create credit · Credit draw");
 expect(html).toContain("Payment amount");
 expect(html).toContain("Shared payer amount");expect(html.match(/AMOUNT\(7\)/g)).toHaveLength(1);
 expect(html).not.toContain("Source recognition");
});
it.each([
 ["pre_revoked", "not_created", "Revoked", "No creation transaction exists"],
 ["revoked_after_creation", "created", "Revoked", "creation-tx"],
])("distinguishes lifecycle %s using full history evidence", (lifecycleStatus,creationStatus,label,record)=>{
 const data={lifecycleStatus,creationStatus,creationTransaction: creationStatus === "created" ? {txHash:"creation-tx"} : undefined, activities:{items:[]}};
 const html=renderToStaticMarkup(<MemoryRouter><GrantHistory data={data} t={k=>dictionaries.en[k]} address={v=><code>{v}</code>} transaction={r=><a href={`/transactions/${r.txHash}`}>{r.txHash}</a>} amount={v=><span>{v}</span>} href={()=>"/"}/></MemoryRouter>);
 const publicInfo=html.split('<section class="card">')[1];
 expect(publicInfo).toContain(label);
 expect(publicInfo).not.toContain("Creation record");
 expect(publicInfo).not.toContain("before creation");
 expect(publicInfo).not.toContain("subsequently revoked");
 expect(html.split('<section class="card">')[2]).toContain(record);
});
it("keeps operation outputs collapsed and explains multi-grant scope with structured rows",()=>{
 const data={tokenAddress:"0x0000000000000000000000000000000000000000",activities:{items:[{kind:"creditDraw",txHash:"draw",grantRefs:[{grantId:"1"},{grantId:"2"}],creditOutputs:[{creditCommitment:"0x1111111111111111111111111111111111111111111111111111111111111111",treeId:"1",leafIndex:"40"},{creditCommitment:"0x2222222222222222222222222222222222222222222222222222222222222222",treeId:"1",leafIndex:"41"}],noteOutputs:[{noteCommitment:"0x3333333333333333333333333333333333333333333333333333333333333333",treeId:"1",leafIndex:"42",amount:"9"}]}]}};
 const html=renderToStaticMarkup(<MemoryRouter><GrantHistory data={data} t={k=>dictionaries.en[k]} address={v=><code>{v}</code>} transaction={r=><span>{r.txHash}</span>} amount={v=><span>AMOUNT({v})</span>} href={(type,_row,id)=>`/${type}/${id}`}/></MemoryRouter>);
 expect(html).toContain('<details class="grant-outputs">');
 expect(html).not.toMatch(/<details[^>]*\bopen/);
 expect(html).toContain("entire operation");
 expect(html).toContain("Operation outputs");
 expect(html).toContain("2 credit states · 1 Note outputs");
 expect(html).toContain('class="funding-table"');
 expect(html).toContain("AMOUNT(9)");
 expect(html.match(/data-label="Tree ID"/g)).toHaveLength(3);
 expect(html).toContain("0x111111…111111");
 expect(html).not.toContain("Unspecified (zero address)");
 expect(html).toContain('title="0x0000000000000000000000000000000000000000">-');
});
