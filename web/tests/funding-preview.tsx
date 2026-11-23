// Isolated visual test harness, not a route or fixture in the production app.
import { createRoot } from "react-dom/client";
import { useState } from "react";
import { FundingPanel } from "../src/FundingPanel";
import { RecognitionSummary } from "../src/RecognitionSummary";
import { dictionaries, type Language } from "../src/i18n";
import { exactAmount } from "../src/public";
import { operation } from "./funding-fixture";
import "../src/style.css";
function Preview() {
  const [language, setLanguage] = useState<Language>("en");
  const [scenario, setScenario] = useState("7702");
  const t = (key: keyof typeof dictionaries.en) => dictionaries[language][key];
  const op: Record<string, unknown> =
    scenario === "7702"
      ? operation
      : scenario === "ambiguous"
        ? {
            ...operation,
            fundingEvidence: {
              ...operation.fundingEvidence,
              coverage: "ambiguous",
            },
          }
        : scenario === "legacy"
          ? {
              ...operation,
              executionContext: undefined,
              fundingEvidence: undefined,
            }
          : scenario === "failed"
            ? { ...operation, success: false }
            : { ...operation, canonical: false };
  return (
    <main>
      <h1>Test fixture preview</h1>
      <p>This page is a development-only test harness, not a chain record.</p>
      <select
        aria-label="Language"
        value={language}
        onChange={(event) => {
          const value = event.target.value as Language;
          setLanguage(value);
          document.documentElement.lang = value;
        }}
      >
        {Object.keys(dictionaries).map((value) => (
          <option key={value}>{value}</option>
        ))}
      </select>
      <select
        aria-label="Scenario"
        value={scenario}
        onChange={(event) => setScenario(event.target.value)}
      >
        {["7702", "ambiguous", "legacy", "failed", "orphaned"].map((value) => (
          <option key={value}>{value}</option>
        ))}
      </select>
      <section className="card">
        <RecognitionSummary
          data={{
            status: "pending_source",
            reasonCode: "FUNDING_EVIDENCE_INCOMPLETE",
          }}
          t={t}
          status={() => <strong>{t("pending")}</strong>}
        />
        <FundingPanel
          operation={op}
          t={t}
          address={(value) => (
            <span className="address">
              <code>{value}</code>
            </span>
          )}
          amount={(value, token) => (
            <span className="amount">
              <span className="amount-value">
                {exactAmount(value, token.endsWith("0000") ? 18 : 6)}
              </span>
            </span>
          )}
        />
      </section>
    </main>
  );
}
createRoot(document.getElementById("root")!).render(<Preview />);
