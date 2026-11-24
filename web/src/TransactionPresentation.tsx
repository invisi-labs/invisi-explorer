import type { ReactNode } from "react";
import { dictionaries, type Key } from "./i18n";
type RecordData=Record<string,any>;
export function TransactionOperationLabel({data,t}:{data:RecordData;t:(key:Key)=>string}) {
  return Number(data.operationCount)>1
    ? <span>{t("aggregate")}<small className="operation-count">{t("operationCount").replace("{count}",String(data.operationCount))}</small></span>
    : <>{data.kind in dictionaries.en ? t(data.kind as Key) : data.kind ?? "—"}</>;
}
export function TransactionAmounts({data,amount,fallback}:{data:RecordData;amount:(value:string,token:string)=>ReactNode;fallback?:ReactNode}) {
  if (!Array.isArray(data.publicDepositAmounts)) return <>{fallback ?? "—"}</>;
  return data.publicDepositAmounts.length
    ? <>{data.publicDepositAmounts.map((entry:RecordData,i:number)=><div key={entry.tokenAddress ?? i}>{amount(entry.amount,entry.tokenAddress)}</div>)}</>
    : <>—</>;
}
