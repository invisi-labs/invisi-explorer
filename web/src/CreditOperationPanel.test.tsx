import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { MemoryRouter } from "react-router-dom";
import { CreditOperationPanel } from "./CreditOperationPanel";
import { dictionaries } from "./i18n";
const owner = "0x" + "1".repeat(40), token = "0x" + "2".repeat(40);
const render = (operation: Record<string, any>) => renderToStaticMarkup(<MemoryRouter><CreditOperationPanel operation={{chainId:"97",protocolAddress:owner,tokenAddress:token,success:true,canonical:true,feeAmount:"0",...operation}} t={k=>dictionaries.en[k]} address={v=><code>{v}</code>} amount={v=><span>AMOUNT({v})</span>} href={(kind,id)=>`/${kind}/${id}`}/></MemoryRouter>);
it("renders management without transfer fields and keeps private limits undisclosed",()=>{
 for (const kind of ["createCredit","updateCredit","revokeCredit"]) {
  const html=render({kind,grantRefs:[{payer:owner,grantId:"77"}],creditOutputs:[]});
  expect(html).toContain("77");expect(html).toContain("Grant owner");
  expect(html).not.toContain("Withdrawal");expect(html).not.toContain("Funding trace");expect(html).not.toContain("Deposit amount");
 }
 expect(render({kind:"createCredit"})).toContain("Limits and policy are not public");
});
it("labels initial payment and draw payer totals without allocating them to each grant",()=>{
 const funding=[{payer:owner,tokenAddress:token,amount:"7"}];
 expect(render({kind:"createCredit",funding,noteOutputs:[{noteCommitment:"note"}]})).toContain("Initial payment");
 const html=render({kind:"creditDraw",funding,grantRefs:[{payer:owner,grantId:"77"},{payer:owner,grantId:"88"}]});
 expect(html.match(/AMOUNT\(7\)/g)).toHaveLength(1);
 expect(html).toContain("77");expect(html).toContain("88");expect(html).toContain("Payer total");
});
it("does not show failed or orphaned attempts as settled funding or credit states",()=>{
 for (const flag of [{success:false},{canonical:false}]) {
  const html=render({kind:"creditDraw",...flag,funding:[{payer:owner,tokenAddress:token,amount:"7"}],creditOutputs:[{creditCommitment:"new-state"}]});
  expect(html).not.toContain("AMOUNT(7)");expect(html).not.toContain("new-state");
 }
});
