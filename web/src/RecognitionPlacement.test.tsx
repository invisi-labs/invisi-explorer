import type {ReactElement, ReactNode} from "react";
import {afterAll, beforeAll, expect, it, vi} from "vitest";
import {renderToStaticMarkup} from "react-dom/server";
const fixture = vi.hoisted(() => ({path:"/", render:vi.fn(), data:{} as Record<string,any>}));
vi.mock("react-dom/client", () => ({createRoot:()=>({render:fixture.render})}));
vi.mock("react", async (original) => {
 const actual = await original<typeof import("react")>();
 return {...actual, useState:(initial:any) => {
  const value=typeof initial === "function" ? initial() : initial;
  return actual.useState(value && typeof value === "object" && value.loading === true && Object.keys(value).length===1 ? {data:fixture.data,loading:false} : value);
 }};
});
vi.mock("./useCursorPagination", () => ({useCursorPagination:()=>({data:{items:[fixture.data],total:1},loading:false})}));
vi.mock("react-router-dom", async (original) => {
 const actual=await original<typeof import("react-router-dom")>();
 return {...actual,BrowserRouter:({children}:{children:ReactNode})=><actual.MemoryRouter initialEntries={[fixture.path]}>{children}</actual.MemoryRouter>};
});
let application:ReactElement;
const operation={transactionHash:"0x"+"1".repeat(64),chainId:"97",protocolAddress:"0x"+"2".repeat(40),kind:"shield",success:true,recognition:[{status:"recognized"}]};
beforeAll(async()=>{
 vi.stubGlobal("document",{getElementById:()=>({})});
 vi.stubGlobal("navigator",{language:"en"});
 await import("./main");application=fixture.render.mock.calls[0][0];
});
afterAll(()=>vi.unstubAllGlobals());
it("omits recognition only from transaction listings, keeping Note recognition",()=>{
 fixture.data=operation;fixture.path="/";
 const list=renderToStaticMarkup(application);
 expect(list).not.toContain('data-label="Source recognition"');
 expect(list).not.toMatch(/<th\b[^>]*>Source recognition(?:<|$)/);
 expect(list).not.toContain('aria-label="Source recognition · ?"');
 fixture.path="/notes";
 expect(renderToStaticMarkup(application)).toMatch(/<th\b[^>]*>Source recognition(?:<|$)/);
});
it("places single-transaction recognition inline before funding instead of in an aside",()=>{
 fixture.path="/transactions/"+operation.transactionHash;
 fixture.data={...operation,operations:[operation]};
 const html=renderToStaticMarkup(application);
 expect(html).toContain('class="detail-grid detail-grid-single"');
 expect(html).toContain('class="operation-recognition"');
 expect(html).not.toContain('<aside');
 expect(html.indexOf('class="operation-recognition"')).toBeLessThan(html.indexOf('class="funding-panel"'));
});
it("keeps aggregate recognition scoped to applicable internal operations",()=>{
 fixture.path="/transactions/"+operation.transactionHash;
 fixture.data={operations:[operation,{...operation,kind:"revokeCredit",recognition:[{status:"not_applicable"}]}]};
 const html=renderToStaticMarkup(application);
 expect(html.match(/aria-label="Source recognition"/g)).toHaveLength(1);
 expect(html).not.toContain('<aside');
});
it("uses one transaction execution summary and consistent public-funding headings for single and aggregate transactions",()=>{
 fixture.path="/transactions/"+operation.transactionHash;
 for(const operations of [[operation],[operation,{...operation,operationIndex:"1"}]]) {
  fixture.data={operations};
  const html=renderToStaticMarkup(application);
  expect(html.match(/<h2>Transaction execution<\/h2>/g)).toHaveLength(1);
  expect(html.match(/<h3>Public funding<\/h3>/g)).toHaveLength(operations.length);
  expect(html).not.toContain('Funding and execution');
 }
});
