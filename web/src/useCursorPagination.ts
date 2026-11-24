import { useEffect, useMemo, useSyncExternalStore } from "react";
import { CursorPager } from "./pagination";
import { request } from "./public";

export function useCursorPagination<P extends { nextCursor?: string }>(path: string) {
  const pager = useMemo(() => new CursorPager<P>((cursor, signal) =>
    request<P>(`${path}?limit=20${cursor ? "&cursor=" + encodeURIComponent(cursor) : ""}`, signal)
  ), [path]);
  const state = useSyncExternalStore(pager.subscribe, pager.getSnapshot, pager.getSnapshot);
  useEffect(() => { void pager.start(); return pager.stop; }, [pager]);
  return { ...state, next: pager.next, previous: pager.previous, retry: pager.retry };
}
