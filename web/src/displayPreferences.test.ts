import {beforeEach,expect,it,vi} from "vitest";
import {defaultDisplayPreferences,fullTimestamp,readDisplayPreferences,writeDisplayPreferences} from "./displayPreferences";
let values:Map<string,string>;
beforeEach(()=>{values=new Map();vi.stubGlobal('localStorage',{getItem:(k:string)=>values.get(k)??null,setItem:(k:string,v:string)=>values.set(k,v)});});
it('formats the same instant in UTC and a selected timezone without changing source data',()=>{
 const instant='2026-11-25T08:30:00Z';
 writeDisplayPreferences({timeZone:'UTC',timeDisplay:'absolute'});expect(fullTimestamp(instant)).toBe('2026-11-25 08:30:00');
 writeDisplayPreferences({timeZone:'Asia/Shanghai',timeDisplay:'absolute'});expect(fullTimestamp(instant)).toBe('2026-11-25 16:30:00');
 expect(instant).toBe('2026-11-25T08:30:00Z');
});
it('handles daylight saving with named timezones',()=>{
 writeDisplayPreferences({timeZone:'America/New_York',timeDisplay:'absolute'});
 expect(fullTimestamp('2026-07-25T12:00:00Z')).toBe('2026-07-25 08:00:00');
 expect(fullTimestamp('2026-11-25T12:00:00Z')).toBe('2026-11-25 07:00:00');
});
it('rejects invalid zones and recovers malformed stored preferences',()=>{
 expect(writeDisplayPreferences({timeZone:'invalid-zone',timeDisplay:'absolute'})).toBe(false);
 values.set('invisi.display-preferences.v1','{"timeZone":"invalid-zone","timeDisplay":"bad"}');expect(readDisplayPreferences()).toEqual(defaultDisplayPreferences);
 values.set('invisi.display-preferences.v1','malformed');expect(readDisplayPreferences()).toEqual(defaultDisplayPreferences);
});
it('persists preferences and restores the default display',()=>{
 writeDisplayPreferences({timeZone:'UTC',timeDisplay:'absolute'});expect(readDisplayPreferences()).toEqual({timeZone:'UTC',timeDisplay:'absolute'});
 writeDisplayPreferences({...defaultDisplayPreferences});expect(readDisplayPreferences()).toEqual(defaultDisplayPreferences);
});
it('handles seconds and milliseconds equally, without showing hour 24 at midnight',()=>{
 writeDisplayPreferences({timeZone:'UTC',timeDisplay:'absolute'});
 expect(fullTimestamp(0)).toBe('1970-01-01 00:00:00');expect(fullTimestamp('invalid')).toBe('—');
 expect(fullTimestamp(1795595400)).toBe(fullTimestamp(1795595400000));
});
