import { useEffect, useMemo, useSyncExternalStore } from "react";
import { CursorPager } from "./pagination";
import { request } from "./public";
import { normalizeRecordPage, pagePath } from "./recordPages";

export function usePager<P extends { nextCursor?: string }>(
  pager: CursorPager<P>,
) {
  const state = useSyncExternalStore(
    pager.subscribe,
    pager.getSnapshot,
    pager.getSnapshot,
  );
  useEffect(() => {
    void pager.start();
    return pager.stop;
  }, [pager]);
  return {
    ...state,
    next: pager.next,
    previous: pager.previous,
    retry: pager.retry,
  };
}
export function useCursorPagination<P extends { nextCursor?: string }>(
  path: string,
) {
  const pager = useMemo(
    () =>
      new CursorPager<P>(async (cursor, signal) =>
        normalizeRecordPage(await request<P>(pagePath(path, cursor), signal)),
      ),
    [path],
  );
  return usePager(pager);
}
