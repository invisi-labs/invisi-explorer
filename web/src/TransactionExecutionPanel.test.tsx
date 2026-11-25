import { expect, it } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { TransactionExecutionPanel } from "./TransactionExecutionPanel";
import { OperationNoteInputs } from "./OperationNoteInputs";
import { dictionaries } from "./i18n";
const core="0x"+"1".repeat(40),sender="0x"+"2".repeat(40),other="0x"+"3".repeat(40);
const locator={chainId:"97",protocolAddress:core,txHash:"0x"+"a".repeat(64),blockHash:"0x"+"b".repeat(64),blockNumber:"42"};
const operations=Array.from({length:4},(_,i)=>({...locator,operationIndex:String(i),canonical:true,success:true,transactionObservation:{...locator,transactionSender:sender,transactionTo:core,transactionType:"0"}}));
const render=(ops:Record<string,any>[])=>renderToStaticMarkup(<TransactionExecutionPanel operations={ops} t={k=>dictionaries.en[k]} address={v=><code>{v}</code>}/>);
it("shows one bound outer transaction summary for four members",()=>{
 const html=render(operations);
 expect(html.match(/<dt>Transaction submitter \/ Broadcaster/g)).toHaveLength(1);
 expect(html.match(new RegExp(sender,"g"))).toHaveLength(1);
 expect(html).toContain("Regular transaction (Type 0)");
 expect(html).toContain("Core contract");
});
it("does not select the first sender when member observations conflict",()=>{
 const changed=operations.map((op,i)=>i===3?{...op,transactionObservation:{...op.transactionObservation,transactionSender:other}}:op);
 const html=render(changed);
 expect(html).toContain(dictionaries.en.fundingInvalid);
 expect(html).not.toContain(sender);expect(html).not.toContain(other);
});
it("ignores unbound observations and does not borrow another transaction's identity",()=>{
 const changed=operations.map(op=>({...op,transactionObservation:{...op.transactionObservation,txHash:"0x"+"c".repeat(64)}}));
 expect(render(changed)).not.toContain(sender);
 expect(render(changed)).toContain(dictionaries.en.fundingUnknown);
 expect(render([{...operations[0]}, {...operations[1],txHash:"0x"+"d".repeat(64)}])).toContain(dictionaries.en.fundingInvalid);
});
it("uses bound observations when another member has no optional observation",()=>{
 expect(render([operations[0],{...operations[1],transactionObservation:undefined}])).toContain(sender);
});
it("lists only public consumed Note identifiers without disclosing local owner data",()=>{
 const n="0x"+"e".repeat(64);
 const html=renderToStaticMarkup(<OperationNoteInputs operation={{success:true,canonical:true,noteNullifiers:[n],owner:"Bob",amount:"SECRET_AMOUNT"}} t={k=>dictionaries.en[k]} reference={value=><a href={'/nullifiers/'+value+'?namespace=note'}>{value}</a>}/>);
 expect(html).toContain(n);expect(html).toContain("Private Note inputs (1)");
 expect(html).not.toContain("Bob");expect(html).not.toContain("SECRET_AMOUNT");
 const failed=renderToStaticMarkup(<OperationNoteInputs operation={{success:false,canonical:true,noteNullifiers:[n]}} t={k=>dictionaries.en[k]} reference={v=><span>{v}</span>}/>);
 expect(failed).not.toContain(n);
});
it("preserves authenticated execution contexts when the optional transaction read is unavailable",()=>{
 const ops=operations.map(op=>({...op,transactionObservation:undefined,executionContext:{transactionSender:sender,transactionTo:core,transactionType:"4",authorizations:[{chainId:"97",address:other,authority:sender,nonce:"1",signatureValid:true}]}}));
 const html=render(ops);
 expect(html).toContain(sender);expect(html).toContain("EIP-7702");
 expect(html.match(/nonce 1/g)).toHaveLength(1);
 const conflict=ops.map((op,i)=>i===3?{...op,executionContext:{...op.executionContext,transactionSender:other}}:op);
 expect(render(conflict)).toContain(dictionaries.en.fundingInvalid);
 expect(render(conflict)).not.toContain(sender);
});
