import { expect, it, vi } from "vitest";
import { CursorPager } from "./pagination";
const page=(name:string,nextCursor?:string,total="52")=>({items:[{id:name}],nextCursor,total});
const conflict={status:409,code:"CONFLICT"};
it("commits page numbers on success and ignores duplicate next clicks",async()=>{
 let finish!:(value:ReturnType<typeof page>)=>void;
 const load=vi.fn(async(cursor:string)=>cursor?new Promise<ReturnType<typeof page>>(resolve=>{finish=resolve}):page("first","second"));
 const p=new CursorPager(load);await p.start();const next=p.next();const duplicate=p.next();
 expect(p.getSnapshot()).toMatchObject({page:1,loading:true,data:{total:"52"}});expect(load).toHaveBeenCalledTimes(2);
 finish(page("second","third"));await Promise.all([next,duplicate]);
 expect(p.getSnapshot()).toMatchObject({page:2,loading:false,data:{items:[{id:"second"}]}});
});
it("rebases an expired third-page cursor on a fresh snapshot",async()=>{
 let fresh=false;const calls:string[]=[];
 const p=new CursorPager(async(cursor:string)=>{
  calls.push(cursor);if(cursor==="old-third"){fresh=true;throw conflict;}
  if(!cursor)return page("first",fresh?"fresh-second":"old-second");
  if(cursor==="old-second")return page("second","old-third");
  if(cursor==="fresh-second")return page("second","fresh-third");
  if(cursor==="fresh-third")return page("third");throw new Error("unexpected cursor");
 });
 await p.start();await p.next();await p.next();
 expect(calls).toEqual(["","old-second","old-third","","fresh-second","fresh-third"]);
 expect(p.getSnapshot()).toMatchObject({page:3,loading:false,notice:"updated",data:{items:[{id:"third"}]}});
 await p.previous();expect(calls.at(-1)).toBe("fresh-second");expect(p.getSnapshot().page).toBe(2);
});
it("retains page and total on network errors and retries with fresh cursors",async()=>{
 let broken=true;const calls:string[]=[];
 const p=new CursorPager(async(cursor:string)=>{
  calls.push(cursor);if(!cursor)return page("first",broken?"old-second":"fresh-second");
  if(broken)throw {status:503,code:"UNAVAILABLE"};return page("second");
 });
 await p.start();await p.next();
 expect(p.getSnapshot()).toMatchObject({page:1,data:{items:[{id:"first"}],total:"52"},error:{code:"UNAVAILABLE"},loading:false});
 broken=false;await p.retry();expect(calls).toEqual(["","old-second","","fresh-second"]);
 expect(p.getSnapshot()).toMatchObject({page:2,error:undefined,data:{items:[{id:"second"}]}});
});
it("restarts from page one after chain reorganization",async()=>{
 let fresh=false;const p=new CursorPager(async(cursor:string)=>{
  if(cursor){fresh=true;throw {status:409,code:"CHAIN_REORGED"};}
  return page(fresh?"new canonical first":"old first","second");
 });
 await p.start();await p.next();expect(p.getSnapshot()).toMatchObject({page:1,notice:"restarted",data:{items:[{id:"new canonical first"}]}});
});
it("uses the last existing page when refreshed history shrinks",async()=>{
 let fresh=false;const p=new CursorPager(async(cursor:string)=>{
  if(cursor==="third"){fresh=true;throw conflict;}
  if(!cursor)return page("first",fresh?"new-second":"second",fresh?"25":"52");
  return page("second",fresh?undefined:"third",fresh?"25":"52");
 });
 await p.start();await p.next();await p.next();expect(p.getSnapshot()).toMatchObject({page:2,notice:"updated",data:{total:"25",nextCursor:undefined}});
});
it("bounds recovery when the source keeps changing",async()=>{
 const load=vi.fn(async()=>{throw conflict});const p=new CursorPager(load);
 await p.start();expect(load).toHaveBeenCalledTimes(3);expect(p.getSnapshot()).toMatchObject({page:1,loading:false,error:conflict});
});
it("ignores responses after cancellation even if transport resolves",async()=>{
 let finish!:(value:ReturnType<typeof page>)=>void;
 const p=new CursorPager<ReturnType<typeof page>>(()=>new Promise(resolve=>{finish=resolve}));const pending=p.start();p.stop();finish(page("stale"));await pending;
 expect(p.getSnapshot().data).toBeUndefined();
});
