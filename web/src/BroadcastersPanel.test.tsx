import {expect,it} from "vitest";
import {renderToStaticMarkup} from "react-dom/server";
import {MemoryRouter} from "react-router-dom";
import {BroadcastersPanel,broadcasterState} from "./BroadcastersPanel";
import {dictionaries} from "./i18n";
const t=(key:keyof typeof dictionaries.en)=>dictionaries.en[key];
const record={operator:"0x1111111111111111111111111111111111111111",enabled:true,endpoint:"https://relay.example/v1/chains/97",chainId:"97",protocolAddress:"core",observation:{status:"available",checkedAt:1000,responseMillis:21,identityStatus:"matched",tokensStatus:"not_provided",service:{submissionHalted:false,provenance:{enabled:true,health:"available",checkedAt:"1"}}}};
function render(data:any,detail=false){return renderToStaticMarkup(<MemoryRouter><BroadcastersPanel data={data} detail={detail} t={t} href={r=>`/broadcasters/${r.operator}`} network={r=><span>Network {r.chainId}</span>} token={value=><span>TOKEN({value})</span>} address={value=><code>{value}</code>}/></MemoryRouter>);}
it("separates chain registration and observed runtime availability",()=>{
 expect(broadcasterState({...record,enabled:false})).toBe("bcNotProbed");
 expect(broadcasterState({...record,observation:{...record.observation,status:"halted"}})).toBe("bcPaused");
 expect(broadcasterState({...record,observation:{...record.observation,stale:true}})).toBe("bcStale");
 const html=render({items:[record],directoryComplete:true});
 expect(html).toContain("Enabled");expect(html).toContain("Online");expect(html).toContain("Not provided");
 expect(html).not.toContain("None currently available");
});
it("keeps old token catalogs unknown, old provenance health stale and technical details collapsed",()=>{
 const html=render(record,true);
 expect(html).toContain("does not provide a complete token catalog");
 expect(html).toContain("Stale observation");
 expect(html).not.toContain("Relay statistics");
 expect(html).toContain('<details class="card bc-technical">');
 expect(html).not.toMatch(/<details[^>]*\bopen/);
 expect(html).not.toContain("100.00%");
});
it("ignores legacy private statistics while retaining public configured token availability",()=>{
 const data={...record,observation:{...record.observation,tokensStatus:"complete",tokens:[{tokenAddress:"A",supported:true,status:"available"},{tokenAddress:"B",supported:false,status:"pricing_unavailable"}],statistics:{submitted:987654321,pending:0,successful:0,reverted:0,successRateBps:10000,knownGasCostWei:"123456789"}}};
 const html=render(data,true);
 expect(html).toContain("TOKEN(A)");expect(html).toContain("TOKEN(B)");expect(html).toContain("Pricing unavailable");
 expect(html).not.toContain("Relay statistics");expect(html).not.toContain("987654321");expect(html).not.toContain("100.00%");
 const list=render({items:[data],directoryComplete:true});expect(list).toContain("TOKEN(A)");expect(list).not.toContain("TOKEN(B)");
});
