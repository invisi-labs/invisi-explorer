import { describe, it, expect } from "vitest";
import selectedRefundFunding from "../tests/fixtures/selected-refund-funding.json";
import { fundingView } from "./funding";
import { renderToStaticMarkup } from "react-dom/server";
import { FundingPanel } from "./FundingPanel";
import { RecognitionSummary, recognitionReasons } from "./RecognitionSummary";
import { dictionaries, type Key } from "./i18n";
import { operation } from "../tests/funding-fixture";
const [player, relay, router, core, delegate] = [1, 2, 3, 4, 5].map(
  (n) => "0x" + n.toString(16).padStart(40, "0"),
);

describe("public funding panel", () => {
  for (const [language, dictionary] of Object.entries(dictionaries)) {
    const t = (key: Key) => dictionary[key];
    it(`shows a third-party contract and 7702 evidence in ${language}`, () => {
      const html = renderToStaticMarkup(
        <FundingPanel
          operation={operation}
          t={t}
          address={(value) => <code>{value}</code>}
          amount={(value) => <span>{value}</span>}
        />,
      );
      for (const address of [player, relay, router, core, delegate])
        expect(html).toContain(address);
      expect(html).toContain("EIP-7702");
      expect(html).toContain("100200000");
      expect(html).toContain(dictionary.fundingAuthorizationCaveat);
      expect(html).toContain(dictionary.fundingSubmitterCaveat);
    });
    it(`explains source decisions without hiding original codes in ${language}`, () => {
      const html = renderToStaticMarkup(
        <RecognitionSummary
          data={{
            recognitionState: "pending_source",
            reasonCode: "FUNDING_EVIDENCE_INCOMPLETE",
          }}
          t={t}
          status={(value) => <span>{String(value)}</span>}
        />,
      );
      expect(html).toContain("pending_source");
      expect(html).toContain(dictionary.reasonFundingIncomplete);
      expect(html).toContain("FUNDING_EVIDENCE_INCOMPLETE");
    });
  }
  it("does not present failed or replaced calls as completed funding", () => {
    for (const overrides of [{ success: false }, { canonical: false }]) {
      const html = renderToStaticMarkup(
        <FundingPanel
          operation={{ ...operation, ...overrides }}
          t={(key) => dictionaries.en[key]}
          address={(value) => <code>{value}</code>}
          amount={(value) => <span>{value}</span>}
        />,
      );
      expect(html).not.toContain(player);
      expect(html).not.toContain("100200000");
    }
  });
  it("does not label backend-selected gross input as final spending when a later refund is omitted", () => {
    // This fixture is checked against the actual Go funding-evidence builder.
    const actual = selectedRefundFunding;
    const view = fundingView(actual);
    expect(view.sourceCandidates[0].assets[0].amount).toBe("20");
    const html = renderToStaticMarkup(
      <FundingPanel
        operation={actual}
        t={(key) => dictionaries.en[key]}
        address={(value) => <code>{value}</code>}
        amount={(value) => <span>AMOUNT({value})</span>}
      />,
    );
    const roles = html.match(/<dl class="funding-roles">([\s\S]*?)<\/dl>/)?.[1];
    expect(roles).toBeDefined();
    expect(roles).not.toContain("AMOUNT(20)");
    expect(html).toContain("Later refunds may be absent");
    expect(html).toContain("AMOUNT(20)");
  });
  it("preserves unknown reasons, removes duplicates and escapes service text", () => {
    expect(
      recognitionReasons({
        reason: "NEW_CODE",
        reasonCode: "NEW_CODE",
        reasons: ["OTHER", {}, ""],
      }),
    ).toEqual(["NEW_CODE", "OTHER"]);
    const html = renderToStaticMarkup(
      <RecognitionSummary
        data={{ reason: "<script>alert(1)</script>" }}
        t={(key) => dictionaries.en[key]}
        status={() => null}
      />,
    );
    expect(html).not.toContain("<script>");
    expect(html).toContain("&lt;script&gt;");
  });
});

const privateOperation = {
 kind:"transact",canonical:true,success:true,chainId:"97",protocolAddress:core,
 txHash:"0x"+"a".repeat(64),blockHash:"0x"+"b".repeat(64),blockNumber:"42",
 funding:[],withdrawalAmount:"0",feeAmount:"0",
 noteNullifiers:["0x"+"c".repeat(64)],noteOutputs:[{noteCommitment:"0x"+"d".repeat(64)}],
 transactionObservation:{chainId:"97",protocolAddress:core,txHash:"0x"+"a".repeat(64),blockHash:"0x"+"b".repeat(64),blockNumber:"42",transactionSender:relay,transactionTo:core,transactionType:"0"},
};
const renderFunding=(op:Record<string,unknown>,hideTransaction=false)=>renderToStaticMarkup(<FundingPanel operation={op} hideTransaction={hideTransaction} t={k=>dictionaries.en[k]} address={v=><code>{v}</code>} amount={v=><span>{v}</span>}/>);
it("explains private Note funding without implying missing public trace or naming owners",()=>{
 const html=renderFunding(privateOperation);
 expect(html).toContain("Uses private Note inputs");
 expect(html).not.toContain(dictionaries.en.fundingUnavailable);
 expect(html).not.toContain(dictionaries.en.fundingSourceUnknown);
 expect(html).toContain(dictionaries.en.fundingNoDeposit);
 expect(html).toContain(dictionaries.en.fundingNoWithdrawal);
});
it("keeps missing-trace warnings for public funding and incomplete or unsupported records",()=>{
 for(const patch of [{funding:[{payer:player,tokenAddress:core,amount:"1"}]},{funding:undefined},{unsupportedReason:"UNSUPPORTED"},{noteNullifiers:[]},{noteOutputs:[{amount:"7"}]},{noteOutputs:[{}]},{success:false},{canonical:false}]){
  const html=renderFunding({...privateOperation,...patch});
  expect(html).toContain(dictionaries.en.fundingUnavailable);
  expect(html).not.toContain("Uses private Note inputs");
 }
});
it("keeps operation funding but omits outer transaction fields when grouped",()=>{
 const html=renderFunding(operation,true);
 expect(html).not.toContain(dictionaries.en.fundingSubmitter);
 expect(html).not.toContain(dictionaries.en.fundingTransactionTo);
 expect(html).not.toContain(dictionaries.en.fundingExecution);
 expect(html).toContain(dictionaries.en.fundingCorePayers);
 expect(html).toContain("100200000");
 expect(html).toContain(dictionaries.en.fundingCaller);
});
