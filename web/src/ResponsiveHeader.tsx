import {useEffect,useId,useRef,useState,type ReactNode} from "react";
import {useLocation} from "react-router-dom";

export function ResponsiveHeader({brand,actions,children,navigationLabel,openLabel,closeLabel}:{
 brand:ReactNode;actions:ReactNode;children:ReactNode;
 navigationLabel:string;openLabel:string;closeLabel:string;
}) {
 const [open,setOpen]=useState(false);
 const root=useRef<HTMLElement>(null),trigger=useRef<HTMLButtonElement>(null);
 const id=useId(),location=useLocation();
 useEffect(()=>setOpen(false),[location.pathname,location.search]);
 useEffect(()=>{
  if(!open)return;
  const outside=(event:PointerEvent)=>{if(!root.current?.contains(event.target as Node))setOpen(false);};
  document.addEventListener("pointerdown",outside);
  return()=>document.removeEventListener("pointerdown",outside);
 },[open]);
 return <header className="site-header" ref={root} onKeyDown={event=>{
  if(event.key==="Escape"&&open&&!event.defaultPrevented){event.preventDefault();setOpen(false);trigger.current?.focus();}
 }} onBlur={event=>{if(!event.currentTarget.contains(event.relatedTarget as Node))setOpen(false);}}>
  <div className="nav">
   {brand}
   <div className="header-actions">
    {actions}
    <button ref={trigger} type="button" className="mobile-nav-toggle" aria-label={open?closeLabel:openLabel} aria-expanded={open} aria-controls={id} onClick={()=>setOpen(value=>!value)}>
     <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" aria-hidden="true">
      {open?<path d="m6 6 12 12M6 18 18 6"/>:<path d="M4 6h16M4 12h16M4 18h16"/>}
     </svg>
    </button>
   </div>
  </div>
  <nav id={id} className="site-navigation" aria-label={navigationLabel} data-open={open||undefined} onClick={event=>{
   if((event.target as Element).closest("a"))setOpen(false);
  }}>{children}</nav>
 </header>;
}
