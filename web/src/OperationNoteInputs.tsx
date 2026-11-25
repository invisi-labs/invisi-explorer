import { TermHelp } from "./TermHelp";
import type { ReactNode } from "react";
import type { Key } from "./i18n";

export function OperationNoteInputs({operation, t, reference}: {
  operation: Record<string, unknown>;
  t: (key: Key) => string;
  reference: (nullifier: string) => ReactNode;
}) {
  if (operation.success !== true || operation.canonical !== true || !Array.isArray(operation.noteNullifiers)) return null;
  const inputs = operation.noteNullifiers.filter((value): value is string => typeof value === "string" && /^0x[0-9a-f]{64}$/i.test(value) && !/^0x0{64}$/i.test(value));
  if (!inputs.length) return null;
  return <section className="note-inputs">
    <h3>{t("noteInputsCount").replace("{count}", String(inputs.length))}<TermHelp term="noteInputsCount" label={t("noteInputsCount").replace("{count}",String(inputs.length))} t={t}/></h3>
    <p className="muted">{t("noteInputsHint")}</p>
    <ul>{inputs.map(value => <li key={value}>{reference(value)}</li>)}</ul>
  </section>;
}
