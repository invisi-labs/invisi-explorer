import type { Key } from "./i18n";
const effectKeys: Record<string,Key> = {
  created:"creditStateCreated", updated:"creditStateUpdated",
  advanced:"creditStateAdvanced", revoked:"creditStateRevoked",
};
export function GrantStateEffect({effect,t}:{effect:string;t:(key:Key)=>string}) {
  return <>{t(effectKeys[effect] ?? "fundingUnknown")}</>;
}
