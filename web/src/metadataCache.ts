type Metadata = Record<string, any>;
const key = "invisi.explorer.metadata.v1";
const ttl = 5 * 60 * 1000;
export function readMetadataCache(storage: Pick<Storage,"getItem">, now = Date.now()): Metadata | undefined {
 try {
  const cached = JSON.parse(storage.getItem(key) ?? "null");
  if (cached && now >= cached.time && now-cached.time < ttl && Array.isArray(cached.data?.networks) && Array.isArray(cached.data?.tokens)) return cached.data;
 } catch { /* Storage is optional. */ }
}
export function writeMetadataCache(storage: Pick<Storage,"setItem">, data: Metadata, now = Date.now()) {
 if (!Array.isArray(data.networks) || !Array.isArray(data.tokens)) return;
 try { storage.setItem(key,JSON.stringify({time:now,data})); } catch { /* Storage is optional. */ }
}
