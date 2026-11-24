import type { ReactNode } from "react";
import { RecognitionSummary } from "./RecognitionSummary";
import { showsSourceRecognition } from "./public";
import type { Key } from "./i18n";
type RecordData=Record<string,any>;
export function SourceRecognition({data,t,status,aside=false}:{data:RecordData;t:(key:Key)=>string;status:(value:unknown)=>ReactNode;aside?:boolean}) {
  if (!showsSourceRecognition("",data)) return null;
  const rows: RecordData[]=Array.isArray(data.recognition)?data.recognition:data.recognition && typeof data.recognition==="object"?[data.recognition]:[];
  return <section className={aside?"card":"operation-recognition"} aria-label={t("recognition")}>
    {aside?<h2>{t("recognition")}</h2>:<h3>{t("recognition")}</h3>}
    {rows.length?rows.map((row,i)=><div key={i}>
      <RecognitionSummary data={row} t={t} status={status}/>
      <details><summary>{t("recognitionDetails")}</summary><pre>{JSON.stringify(row,null,2)}</pre></details>
    </div>):status(data.recognitionStatus ?? data.status)}
    <p className="muted">{t("privacy")}</p>
  </section>;
}
