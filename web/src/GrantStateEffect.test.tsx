import { it,expect } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { GrantStateEffect } from "./GrantStateEffect";
import { dictionaries } from "./i18n";
it("explains state transitions without inventing private balance changes",()=>{
 const t=(key:keyof typeof dictionaries.en)=>dictionaries.en[key];
 for (const [effect,expected] of [["created","Credit state created"],["updated","Credit state replaced"],["advanced","New credit state after payment"],["revoked","Authorization revoked"]]) {
  const html=renderToStaticMarkup(<GrantStateEffect effect={effect} t={t}/>);
  expect(html).toContain(expected);expect(html).not.toContain("balance");
 }
});
