export type DisplayPreferences = {timeZone:string; timeDisplay:"relative"|"absolute"};
export const defaultDisplayPreferences:DisplayPreferences={timeZone:"local",timeDisplay:"relative"};
const key="invisi.display-preferences.v1";
let temporary:DisplayPreferences|undefined;
export function validTimeZone(value:unknown):value is string {
 if(typeof value!=="string")return false;
 if(value==="local")return true;
 try {new Intl.DateTimeFormat("en",{timeZone:value}).format();return true;}catch{return false;}
}
export function readDisplayPreferences():DisplayPreferences {
 if(temporary)return {...temporary};
 try {const v=JSON.parse(localStorage.getItem(key)??"null");return {timeZone:validTimeZone(v?.timeZone)?v.timeZone:"local",timeDisplay:v?.timeDisplay==="absolute"?"absolute":"relative"};}catch{return {...defaultDisplayPreferences};}
}
export function writeDisplayPreferences(value:DisplayPreferences):boolean {
 if(!validTimeZone(value.timeZone)||!["relative","absolute"].includes(value.timeDisplay))return false;
 let stored=true;
 temporary=undefined;
 try{localStorage.setItem(key,JSON.stringify(value));}catch{temporary={...value};stored=false;}
 if(typeof window!=="undefined")window.dispatchEvent(new Event("invisi-display-change"));
 return stored;
}
export function displayTimeZone():string {
 const zone=readDisplayPreferences().timeZone;
 return zone==="local"?Intl.DateTimeFormat().resolvedOptions().timeZone||"UTC":zone;
}
export function timestampDate(value:unknown):Date {
 const numeric=typeof value==="number"||typeof value==="string"&&/^\d+$/.test(value);
 return new Date(numeric?Number(value)*(Number(value)<1e12?1000:1):String(value));
}
export function fullTimestamp(value:unknown):string {
 const date=timestampDate(value);if(!Number.isFinite(date.getTime()))return "—";
 const parts=new Intl.DateTimeFormat("en-GB",{timeZone:displayTimeZone(),year:"numeric",month:"2-digit",day:"2-digit",hour:"2-digit",minute:"2-digit",second:"2-digit",hourCycle:"h23"}).formatToParts(date);
 const part=(type:string)=>parts.find(p=>p.type===type)?.value??"";
 return `${part("year")}-${part("month")}-${part("day")} ${part("hour")}:${part("minute")}:${part("second")}`;
}
export function formattedTimestamp(value:unknown,language:string):string {
 const date=timestampDate(value),diff=Math.round((date.getTime()-Date.now())/1000);
 if(readDisplayPreferences().timeDisplay==="absolute"||!Number.isFinite(diff)||Math.abs(diff)>=86400)return fullTimestamp(value);
 const unit=Math.abs(diff)<60?"second":Math.abs(diff)<3600?"minute":"hour";
 return new Intl.RelativeTimeFormat(language,{numeric:"auto"}).format(Math.round(diff/(unit==="second"?1:unit==="minute"?60:3600)),unit);
}
export function timeZoneOptions():string[] {
 try{return (Intl as typeof Intl&{supportedValuesOf:(key:string)=>string[]}).supportedValuesOf("timeZone");}catch{return ["America/New_York","America/Los_Angeles","Europe/London","Europe/Paris","Asia/Shanghai","Asia/Hong_Kong","Asia/Taipei","Asia/Singapore","Asia/Tokyo","Australia/Sydney"];}
}
