import { HelpTip } from "./HelpTip";
import type { Key } from "./i18n";
export function TermHelp({
  term,
  label,
  t,
}: {
  term: string;
  label: string;
  t: (key: Key) => string;
}) {
  const keys: Record<string, Key> = {
    recognition: "termHelp_recognition",
    noteCommitmentLabel: "termHelp_noteCommitmentLabel",
    noteInputsCount: "termHelp_noteInputsCount",
    nullifier: "termHelp_nullifier",
    creditCommitmentLabel: "termHelp_creditCommitmentLabel",
    confirmed: "termHelp_confirmed",
    protocol: "termHelp_protocol",
    treeIdLabel: "termHelp_treeIdLabel",
    leafIndexLabel: "termHelp_leafIndexLabel",
    fundingSubmitter: "termHelp_fundingSubmitter",
    fundingType: "termHelp_fundingType",
  };
  const key = keys[term];
  return key ? <HelpTip label={label + " · ?"} text={t(key)} /> : null;
}
