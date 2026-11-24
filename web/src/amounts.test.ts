import { expect, it } from "vitest";
import { amountView } from "./amounts";

it("shortens a list amount while preserving its exact copy value", () => {
 expect(amountView("2070045901362772257",18,true)).toMatchObject({display:"≈2.070046",exact:"2.070045901362772257",copyValue:"2.070045901362772257",raw:false});
 expect(amountView("2070045901362772257",18,false).display).toBe("2.070045901362772257");
});
it("formats grouped whole amounts without floating point conversion", () => {
 expect(amountView("75000000000000000000000",18,true)).toMatchObject({display:"75,000",copyValue:"75000"});
 expect(amountView("900719925474099312345678901234567890",18,true)).toMatchObject({display:"≈900,719,925,474,099,312.345679",copyValue:"900719925474099312.34567890123456789"});
 expect(amountView("1999999999999999999",18,true).display).toBe("≈2");
});
it("does not present small positive amounts as zero", () => {
 expect(amountView("1",18,true)).toMatchObject({display:"<0.000001",copyValue:"0.000000000000000001"});
 expect(amountView("-1",18,true).display).toBe(">-0.000001");
 expect(amountView("0",18,true).display).toBe("0");
 expect(amountView("1234567000000",18,true).display).toBe("≈0.000001");
});
it("keeps exact short fractions and unknown raw units distinct", () => {
 expect(amountView("1010000000000000000","18",true).display).toBe("1.01");
 expect(amountView("1234567",6,true).display).toBe("1.234567");
 expect(amountView("1000000000000000000",undefined,true)).toMatchObject({display:"1,000,000,000,000,000,000",copyValue:"1000000000000000000",raw:true});
 expect(amountView("12",0,true)).toMatchObject({display:"12",raw:false});
 expect(amountView("12",256,true).raw).toBe(true);
 expect(amountView(undefined,18,true).valid).toBe(false);
});
