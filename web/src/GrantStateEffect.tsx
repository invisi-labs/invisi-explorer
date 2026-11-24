import type { Key } from "./i18n";
const effectKeys: Record<string,Key> = {
  pre_revoked:"grantPreRevoked", revoked_after_creation:"grantRevokedAfterCreation",
  created:"creditStateCreated", updated:"creditStateUpdated",
  advanced:"creditStateAdvanced", revoked:"creditStateRevoked",
};
export function GrantStateEffect({effect,t}:{effect:string;t:(key:Key)=>string}) {
  return <>{t(effectKeys[effect] ?? "fundingUnknown")}</>;
}
