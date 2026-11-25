import {useId,useState} from "react";
import type {Language} from "./i18n";
import {defaultDisplayPreferences,displayTimeZone,formattedTimestamp,fullTimestamp,readDisplayPreferences,timeZoneOptions,writeDisplayPreferences,type DisplayPreferences} from "./displayPreferences";
const strings={
 en:{title:"Settings",intro:"Customize how this site looks to you. Preferences are saved in this browser and do not change on-chain data.",language:"Language",languageHint:"Choose your preferred interface language.",zone:"Time zone",zoneHint:"Used for full dates, transaction times and time tooltips.",local:"Follow device",utc:"UTC",other:"Other time zones",display:"Time display",displayHint:"Choose the default display in transaction and activity lists.",relative:"Relative time",absolute:"Full date and time",preview:"Preview",reset:"Restore defaults",saved:"Saved in this browser",temporary:"Browser storage is unavailable. Changes apply only for this session.",zoneNotice:"Times use the selected time zone. Raw API and JSON values remain unchanged."},
 "zh-Hans":{title:"设置",intro:"调整当前网站的显示偏好。设置保存在此浏览器中，不会改变链上数据。",language:"语言",languageHint:"选择界面使用的语言。",zone:"时区",zoneHint:"用于完整日期、交易时间及时间悬停提示。",local:"跟随设备",utc:"UTC",other:"其他时区",display:"时间显示",displayHint:"选择交易和操作列表的默认时间显示方式。",relative:"相对时间",absolute:"完整日期时间",preview:"预览",reset:"恢复默认设置",saved:"已保存到此浏览器",temporary:"浏览器存储不可用，修改仅在当前会话生效。",zoneNotice:"时间按所选时区显示，API 和 JSON 的原始值保持不变。"},
 "zh-Hant":{title:"設定",intro:"調整目前網站的顯示偏好。設定儲存在此瀏覽器中，不會改變鏈上資料。",language:"語言",languageHint:"選擇介面使用的語言。",zone:"時區",zoneHint:"用於完整日期、交易時間及時間懸停提示。",local:"跟隨裝置",utc:"UTC",other:"其他時區",display:"時間顯示",displayHint:"選擇交易與操作列表的預設時間顯示方式。",relative:"相對時間",absolute:"完整日期時間",preview:"預覽",reset:"恢復預設設定",saved:"已儲存至此瀏覽器",temporary:"瀏覽器儲存空間無法使用，修改僅在目前工作階段生效。",zoneNotice:"時間按所選時區顯示，API 與 JSON 的原始值維持不變。"},
};
export function settingsLabel(language:Language){return strings[language].title;}
export function SettingsPage({language,onLanguageChange,defaultLanguage}:{language:Language;onLanguageChange:(value:Language)=>void;defaultLanguage:()=>Language}) {
 const [preferences,setPreferences]=useState(readDisplayPreferences),[status,setStatus]=useState<"saved"|"temporary">();
 const id=useId(),t=strings[language];
 const save=(next:DisplayPreferences)=>{setPreferences(next);setStatus(writeDisplayPreferences(next)?"saved":"temporary");};
 const changeLanguage=(value:Language)=>{let stored=true;try{localStorage.setItem("invisi.language",value);}catch{stored=false;}onLanguageChange(value);setStatus(stored?"saved":"temporary");};
 return <>
  <h1>{t.title}</h1><p className="muted settings-intro">{t.intro}</p>
  <section className="card settings-panel" aria-label={t.title}>
   <div className="settings-row"><div><label htmlFor={`${id}-language`}>{t.language}</label><p>{t.languageHint}</p></div><select id={`${id}-language`} value={language} onChange={event=>changeLanguage(event.target.value as Language)}><option value="zh-Hans">简体中文</option><option value="zh-Hant">繁體中文</option><option value="en">English</option></select></div>
   <div className="settings-row"><div><label htmlFor={`${id}-zone`}>{t.zone}</label><p>{t.zoneHint}</p></div><select id={`${id}-zone`} value={preferences.timeZone} onChange={event=>save({...preferences,timeZone:event.target.value})}><option value="local">{t.local} ({Intl.DateTimeFormat().resolvedOptions().timeZone})</option><option value="UTC">{t.utc}</option><optgroup label={t.other}>{Array.from(new Set([...timeZoneOptions(),...(preferences.timeZone!=="local"&&preferences.timeZone!=="UTC"?[preferences.timeZone]:[])])).filter(zone=>zone!=="UTC").map(zone=><option key={zone} value={zone}>{zone}</option>)}</optgroup></select></div>
   <div className="settings-row"><div><label htmlFor={`${id}-display`}>{t.display}</label><p>{t.displayHint}</p></div><select id={`${id}-display`} value={preferences.timeDisplay} onChange={event=>save({...preferences,timeDisplay:event.target.value as DisplayPreferences["timeDisplay"]})}><option value="relative">{t.relative}</option><option value="absolute">{t.absolute}</option></select></div>
   <div className="settings-preview"><strong>{t.preview}</strong><time title={`${fullTimestamp(Date.now()-180000)} · ${displayTimeZone()}`}>{formattedTimestamp(Date.now()-180000,language)}</time><small>{displayTimeZone()} · {t.zoneNotice}</small></div>
   <div className="settings-actions"><button type="button" onClick={()=>{let stored=true;try{localStorage.removeItem("invisi.language");}catch{stored=false;}const next=defaultLanguage();onLanguageChange(next);setPreferences({...defaultDisplayPreferences});setStatus(writeDisplayPreferences({...defaultDisplayPreferences})&&stored?"saved":"temporary");}}>{t.reset}</button><span role="status">{status?t[status]:""}</span></div>
  </section>
 </>;
}
