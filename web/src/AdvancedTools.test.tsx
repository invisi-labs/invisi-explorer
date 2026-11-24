import {expect,it} from "vitest";
import {renderToStaticMarkup} from "react-dom/server";
import {MemoryRouter} from "react-router-dom";
import {AdvancedTools} from "./AdvancedTools";
import {dictionaries} from "./i18n";
it.each(["/nullifiers", "/nullifiers/0x123", "/trees", "/trees/protocol%3A1", "/roots", "/roots/checkpoint"])("marks advanced navigation active on %s",path=>{
 const html=renderToStaticMarkup(<MemoryRouter initialEntries={[path]}><AdvancedTools t={key=>dictionaries.en[key]}/></MemoryRouter>);
 expect(html).toContain('data-active="true"');
 expect(html).toContain('aria-expanded="false"');
 expect(html).toContain("Advanced tools");
 expect(html).not.toContain('class="advanced-tools-panel"');
});
it("does not mark advanced tools active on primary pages",()=>{
 const html=renderToStaticMarkup(<MemoryRouter initialEntries={["/grants"]}><AdvancedTools t={key=>dictionaries.en[key]}/></MemoryRouter>);
 expect(html).not.toContain("data-active");
});
