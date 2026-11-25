import {readDisplayPreferences} from "./displayPreferences";
import {useEffect,useState} from "react";
export function useDisplayPreferenceUpdates(){
 const [,refresh]=useState(0);
 useEffect(()=>{
  const update=()=>refresh(value=>value+1);
  const stored=(event:StorageEvent)=>{if(event.key===null||event.key==="invisi.display-preferences.v1")update();};
  const timer=window.setInterval(()=>{if(readDisplayPreferences().timeDisplay==="relative")update();},60000);
  window.addEventListener("invisi-display-change",update);window.addEventListener("storage",stored);
  return()=>{window.clearInterval(timer);window.removeEventListener("invisi-display-change",update);window.removeEventListener("storage",stored);};
 },[]);
}
