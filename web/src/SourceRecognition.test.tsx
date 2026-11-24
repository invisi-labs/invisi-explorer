import { it, expect } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { SourceRecognition } from "./SourceRecognition";
import { dictionaries } from "./i18n";
it("keeps each internal operation's recognition separate and hides inapplicable ones",()=>{
 const t=(key:keyof typeof dictionaries.en)=>dictionaries.en[key];
 const html=renderToStaticMarkup(<>
  <SourceRecognition data={{recognition:[{status:"recognized",reason:"OPERATION_ONE"}]}} t={t} status={v=><span>{String(v)}</span>}/>
  <SourceRecognition data={{recognition:[{status:"source_rejected",reason:"OPERATION_TWO"}]}} t={t} status={v=><span>{String(v)}</span>}/>
 </>);
 expect(html.match(/aria-label="Source recognition"/g)).toHaveLength(2);
 expect(html).toContain("OPERATION_ONE");expect(html).toContain("OPERATION_TWO");
 expect(renderToStaticMarkup(<SourceRecognition data={{recognition:[{status:"not_applicable"}]}} t={t} status={()=>null}/>)).toBe("");
});
