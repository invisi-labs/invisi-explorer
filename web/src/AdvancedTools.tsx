import {useEffect,useId,useRef,useState} from "react";
import {Link,useLocation} from "react-router-dom";
import type {Key} from "./i18n";
export function AdvancedTools({t}:{t:(key:Key)=>string}) {
 const [open,setOpen]=useState(false);
 const root=useRef<HTMLDivElement>(null);
 const trigger=useRef<HTMLButtonElement>(null);
 const id=useId();
 const location=useLocation();
 const type=location.pathname.split("/")[1];
 const active=["nullifiers","trees","roots"].includes(type);
 useEffect(()=>setOpen(false),[location.pathname,location.search]);
 useEffect(()=>{
  if(!open) return;
  const close=(event:PointerEvent)=>{if(!root.current?.contains(event.target as Node)) setOpen(false);};
  document.addEventListener("pointerdown",close);
  return ()=>document.removeEventListener("pointerdown",close);
 },[open]);
 return <div className="advanced-tools" ref={root} onBlur={event=>{
  if(!event.currentTarget.contains(event.relatedTarget as Node)) setOpen(false);
 }} onKeyDown={event=>{
  if(event.key === "Escape" && open) {event.preventDefault();setOpen(false);trigger.current?.focus();}
 }}>
  <button ref={trigger} className="advanced-tools-trigger" type="button" aria-expanded={open} aria-controls={id} data-active={active || undefined} onClick={()=>setOpen(value=>!value)}>
   {t("advancedTools")}<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" aria-hidden="true"><path d="m7 10 5 5 5-5"/></svg>
  </button>
  {open && <div className="advanced-tools-panel" id={id}>
   {(["nullifiers","trees","roots"] as const).map(route=><Link key={route} to={`/${route}`} aria-current={type === route ? "page" : undefined} onClick={()=>setOpen(false)}>{t(route)}</Link>)}
  </div>}
 </div>;
}
