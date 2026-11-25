type RecordData = Record<string, any>;
export function pagePath(path: string, cursor: string) {
  const [pathname, query = ""] = path.split("?");
  const params = new URLSearchParams(query);
  params.set("limit", "20");
  params.delete("cursor");
  if (cursor) params.set("cursor", cursor);
  return pathname + "?" + params.toString();
}
export function normalizeRecordPage<P extends RecordData>(
  data: P,
): P & { nextCursor?: string } {
  return {
    ...data,
    nextCursor: data.nextCursor ?? data.activities?.nextCursor,
    total: data.total ?? data.activities?.total,
  };
}
export function recordRows(data: RecordData): RecordData[] | undefined {
  return data.activities?.items ?? data.occurrences ?? data.items;
}
