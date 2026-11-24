import {expect,it} from "vitest";
import {readMetadataCache,writeMetadataCache} from "./metadataCache";
it("retains verified token precision and network metadata across page reloads with bounded freshness",()=>{
 let value:string|null=null;
 const storage={getItem:()=>value,setItem:(_key:string,v:string)=>{value=v;}};
 const data={networks:[{chainId:"97"}],tokens:[{chainId:"97",address:"token",symbol:"iUSD",decimals:18}]};
 writeMetadataCache(storage,data,1000);
 expect(readMetadataCache(storage,2000)).toEqual(data);
 expect(readMetadataCache(storage,301000)).toBeUndefined();
 expect(readMetadataCache(storage,999)).toBeUndefined();
 value="invalid";expect(readMetadataCache(storage,2000)).toBeUndefined();
});
it("tolerates unavailable storage and rejects partial metadata",()=>{
 const storage={getItem:()=>{throw Error();},setItem:()=>{throw Error();}};
 expect(readMetadataCache(storage)).toBeUndefined();
 expect(()=>writeMetadataCache(storage,{networks:[],tokens:[]})).not.toThrow();
 let saved=false;writeMetadataCache({setItem:()=>{saved=true;}},{networks:[]});expect(saved).toBe(false);
});
