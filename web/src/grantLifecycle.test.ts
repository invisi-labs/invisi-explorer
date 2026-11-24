import {expect,it} from "vitest";
import {compactIdentifier,grantStatusKey} from "./credit";
it("compacts long decimal IDs and addresses with both ends visible",()=>{
 expect(compactIdentifier("109907769723541401126188013476061092861")).toBe("10990776…092861");
 expect(compactIdentifier("0x057a50e0224131676ddd82e5ce02395680c0c1ac")).toBe("0x057a50…c0c1ac");
 expect(compactIdentifier("77")).toBe("77");
});
it("uses the same revoked status regardless of creation history",()=>{
 expect(grantStatusKey({lifecycleStatus:"pre_revoked",status:"2"})).toBe("creditGrantRevoked");
 expect(grantStatusKey({lifecycleStatus:"revoked_after_creation"})).toBe("creditGrantRevoked");
 expect(grantStatusKey({authorizationStatus:"active"})).toBe("creditGrantActive");
 expect(grantStatusKey({status:"2"})).toBe("creditGrantRevoked");
});
