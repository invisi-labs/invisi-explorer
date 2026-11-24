import { it, expect } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { TransactionOperationLabel, TransactionAmounts } from "./TransactionPresentation";
import { dictionaries } from "./i18n";
it("labels aggregate operation count and keeps single transaction labels",()=>{
 const t=(k:keyof typeof dictionaries.en)=>dictionaries.en[k];
 expect(renderToStaticMarkup(<TransactionOperationLabel data={{kind:"aggregate",operationCount:"3"}} t={t}/>)).toContain("3 operations");
 expect(renderToStaticMarkup(<TransactionOperationLabel data={{kind:"revokeCredit",operationCount:"1"}} t={t}/>)).toContain("Revoke credit");
});
it("shows distinct assets separately and never adds their base units",()=>{
 const html=renderToStaticMarkup(<TransactionAmounts data={{publicDepositAmounts:[{amount:"7",tokenAddress:"a"},{amount:"11",tokenAddress:"b"}]}} amount={(value,token)=><span>{value} {token}</span>}/>);
 expect(html).toContain("7 a");expect(html).toContain("11 b");expect(html).not.toContain("18");
 expect(renderToStaticMarkup(<TransactionAmounts data={{publicDepositAmounts:[]}} amount={()=>null}/>)).toContain("—");
});
