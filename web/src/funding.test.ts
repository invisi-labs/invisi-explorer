import { describe, expect, it } from "vitest";
import { fundingView } from "./funding";
const a = (n: number) => `0x${n.toString(16).padStart(40, "0")}`;
const ZERO = a(0),
  SOURCE = a(1),
  ROUTER = a(2),
  CORE = a(3),
  POOL = a(4),
  GAS = a(5),
  TOKEN = a(6),
  INPUT = a(7),
  DELEGATE = a(8);
const edge = (
  from: string,
  to: string,
  tokenAddress: string,
  amount: string,
  order: string,
) => ({
  kind: tokenAddress === ZERO ? "native" : "erc20",
  from,
  to,
  tokenAddress,
  amount,
  callPath: `0/${order}`,
  contextPath: "0",
  order,
  ...(tokenAddress === ZERO ? {} : { logIndex: order }),
});
function operation(
  transfers = [
    edge(SOURCE, ROUTER, ZERO, "20", "0"),
    edge(ROUTER, POOL, ZERO, "12", "1"),
    edge(POOL, ROUTER, TOKEN, "100", "2"),
    edge(ROUTER, CORE, TOKEN, "100", "3"),
    edge(ROUTER, SOURCE, ZERO, "8", "4"),
  ],
) {
  return {
    success: true,
    canonical: true,
    protocolAddress: CORE,
    executionPath: "0/3",
    funding: [{ payer: ROUTER, tokenAddress: TOKEN, amount: "100" }],
    fundingEvidence: {
      version: "core-funding-v1",
      transactionSender: SOURCE,
      coreCallPath: "0/3",
      coverage: "scoped-flow",
      transfers,
    },
  };
}
describe("public funding roles", () => {
  it("nets refunds when present in selected evidence without calling pools sources", () => {
    const input = operation(),
      before = JSON.stringify(input),
      view = fundingView(input);
    expect(view.coreFunding).toEqual(input.funding);
    expect(view.fundedBy).toEqual([SOURCE]);
    expect(view.sourceCandidates).toEqual([
      {
        address: SOURCE,
        basis: "selected-flow-outflow",
        assets: [{ tokenAddress: ZERO, amount: "12" }],
      },
    ]);
    expect(view.intermediaries).toEqual([ROUTER, POOL]);
    expect(view.submitter).toBe(SOURCE);
    expect(view.transfers).toEqual(input.fundingEvidence.transfers);
    expect(view.rolesConfirmed).toBe(true);
    expect(view.warnings).toContain("source-not-beneficial-owner");
    expect(JSON.stringify(input)).toBe(before);
  });
  it("keeps arbitrary ERC20 swap inputs separate from settlement assets without rounding", () => {
    const view = fundingView(
      operation([
        edge(SOURCE, ROUTER, INPUT, "9007199254740993123456", "0"),
        edge(ROUTER, POOL, INPUT, "9007199254740993123456", "1"),
        edge(POOL, ROUTER, TOKEN, "100", "2"),
        edge(ROUTER, CORE, TOKEN, "100", "3"),
      ]),
    );
    expect(view.fundedBy).toEqual([SOURCE]);
    expect(view.sourceCandidates[0].assets).toEqual([
      { tokenAddress: INPUT, amount: "9007199254740993123456" },
    ]);
    expect(view.coreFunding[0].amount).toBe("100");
    expect(view.intermediaries).toContain(POOL);
  });
  it("separates type-4 gas submitter, authority, delegate and executed Core payer", () => {
    const input = {
      ...operation(),
      executionContext: {
        transactionType: "4",
        transactionSender: GAS,
        transactionTo: SOURCE,
        coreCallPath: "0/3",
        calls: [
          { path: "0", type: "CALL", from: GAS, to: SOURCE },
          { path: "0/0", type: "CALL", from: SOURCE, to: ROUTER },
          { path: "0/3", type: "CALL", from: ROUTER, to: CORE },
        ],
        authorizations: [
          {
            chainId: "1",
            address: DELEGATE,
            authority: SOURCE,
            nonce: "9",
            signatureValid: true,
          },
        ],
      },
    };
    input.fundingEvidence.transactionSender = GAS;
    const view = fundingView(input);
    expect(view.is7702Transaction).toBe(true);
    expect(view.submitter).toBe(GAS);
    expect(view.fundedBy).toEqual([SOURCE]);
    expect(view.coreFunding[0].payer).toBe(ROUTER);
    expect(view.calls).toEqual(input.executionContext.calls);
    expect(view.authorizations).toEqual(input.executionContext.authorizations);
    expect(view.warnings).toContain("authorization-not-execution-proof");
    expect(view.intermediaries).not.toContain(DELEGATE);
  });
  it("does not infer execution or a source from authorizations alone", () => {
    const view = fundingView({
      success: true,
      canonical: true,
      executionContext: {
        transactionType: "4",
        transactionSender: GAS,
        authorizations: [
          {
            chainId: "1",
            address: DELEGATE,
            authority: SOURCE,
            nonce: "0",
            signatureValid: true,
          },
        ],
      },
    });
    expect(view.is7702Transaction).toBe(true);
    expect(view.fundedBy).toEqual([]);
    expect(view.calls).toEqual([]);
    expect(view.coverage).toBe("unavailable");
  });
  it.each(["contract-inventory", "ambiguous"])(
    "leaves %s upstream sources unresolved",
    (coverage) => {
      const input = operation();
      input.fundingEvidence.coverage = coverage;
      const view = fundingView(input);
      expect(view.coverage).toBe(coverage);
      expect(view.fundedBy).toEqual([]);
      expect(view.observedAddresses).toContain(SOURCE);
      expect(view.warnings).toContain(
        coverage === "ambiguous" ? "ambiguous-evidence" : "inventory-boundary",
      );
    },
  );
  it.each([
    {
      success: false,
      canonical: true,
      execution: "failed",
      warning: "execution-failed",
    },
    {
      success: true,
      canonical: false,
      execution: "orphaned",
      warning: "orphaned-operation",
    },
  ])(
    "suppresses executed roles for $execution operations",
    ({ success, canonical, execution, warning }) => {
      const view = fundingView({ ...operation(), success, canonical });
      expect(view.execution).toBe(execution);
      expect(view.rolesConfirmed).toBe(false);
      expect(view.fundedBy).toEqual([]);
      expect(view.intermediaries).toEqual([]);
      expect(view.warnings).toContain(warning);
      expect(view.coreFunding).toHaveLength(1);
    },
  );
  it("shows legacy metadata without inventing generic trace or sender", () => {
    const view = fundingView({
      success: true,
      canonical: true,
      funding: [{ payer: ROUTER, tokenAddress: TOKEN, amount: "100" }],
      routerFunding: {
        routerAddress: ROUTER,
        playerAddress: SOURCE,
        nativeBudget: "20",
        nativeSpent: "12",
        nativeRefund: "8",
      },
    });
    expect(view.coverage).toBe("legacy");
    expect(view.legacy?.playerAddress).toBe(SOURCE);
    expect(view.submitter).toBeNull();
    expect(view.fundedBy).toEqual([]);
    expect(view.transfers).toEqual([]);
  });
  it("does not guess tx sender or upstream owner when traces are unavailable", () => {
    const view = fundingView({
      success: true,
      canonical: true,
      from: GAS,
      funding: [{ payer: ROUTER, tokenAddress: TOKEN, amount: "100" }],
    });
    expect(view.submitter).toBeNull();
    expect(view.coverage).toBe("unavailable");
    expect(view.fundedBy).toEqual([]);
    expect(view.warnings).toContain("evidence-unavailable");
  });
  it("keeps each asset separate and omits fully refunded assets", () => {
    const view = fundingView(
      operation([
        edge(SOURCE, ROUTER, ZERO, "20", "0"),
        edge(ROUTER, SOURCE, ZERO, "20", "1"),
        edge(SOURCE, ROUTER, INPUT, "30", "2"),
        edge(ROUTER, POOL, INPUT, "30", "3"),
        edge(POOL, ROUTER, TOKEN, "100", "4"),
        edge(ROUTER, CORE, TOKEN, "100", "5"),
      ]),
    );
    expect(view.sourceCandidates).toEqual([
      {
        address: SOURCE,
        basis: "selected-flow-outflow",
        assets: [{ tokenAddress: INPUT, amount: "30" }],
      },
    ]);
  });
  it("never calls zero-address minting or a balanced pool a player", () => {
    const view = fundingView(
      operation([
        edge(ZERO, POOL, TOKEN, "100", "0"),
        edge(POOL, ROUTER, TOKEN, "100", "1"),
        edge(ROUTER, CORE, TOKEN, "100", "2"),
      ]),
    );
    expect(view.fundedBy).toEqual([]);
    expect(view.observedAddresses).not.toContain(ZERO);
    expect(view.warnings).toContain("source-unresolved");
  });
  it("fails conservatively on context conflicts and malformed evidence", () => {
    const conflict = fundingView({
      ...operation(),
      executionContext: {
        transactionType: "2",
        transactionSender: GAS,
        coreCallPath: "0/3",
      },
    });
    expect(conflict.fundedBy).toEqual([]);
    expect(conflict.warnings).toContain("context-conflict");
    const malformed = fundingView(
      operation([{ ...edge(SOURCE, CORE, TOKEN, "1", "0"), amount: "1.5" }]),
    );
    expect(malformed.fundedBy).toEqual([]);
    expect(malformed.warnings).toContain("invalid-evidence");
    expect(fundingView(null).execution).toBe("unknown");
  });
  it("retains multiple source assets without adding their quantities", () => {
    const view = fundingView(
      operation([
        edge(SOURCE, ROUTER, ZERO, "12", "0"),
        edge(SOURCE, ROUTER, INPUT, "30", "1"),
        edge(ROUTER, POOL, ZERO, "12", "2"),
        edge(ROUTER, POOL, INPUT, "30", "3"),
        edge(POOL, ROUTER, TOKEN, "100", "4"),
        edge(ROUTER, CORE, TOKEN, "100", "5"),
      ]),
    );
    expect(view.sourceCandidates[0].assets).toEqual([
      { tokenAddress: ZERO, amount: "12" },
      { tokenAddress: INPUT, amount: "30" },
    ]);
  });
  it("leaves a mixed cross-asset account unresolved instead of asserting ownership", () => {
    const view = fundingView(
      operation([
        edge(SOURCE, ROUTER, INPUT, "30", "0"),
        edge(ROUTER, SOURCE, TOKEN, "2", "1"),
        edge(ROUTER, CORE, TOKEN, "100", "2"),
      ]),
    );
    expect(view.fundedBy).toEqual([]);
    expect(view.observedAddresses).toContain(SOURCE);
    expect(view.warnings).toContain("source-unresolved");
  });
  it("does not treat an unknown canonical state as executed", () => {
    const view = fundingView({ ...operation(), canonical: undefined });
    expect(view.execution).toBe("unknown");
    expect(view.rolesConfirmed).toBe(false);
    expect(view.fundedBy).toEqual([]);
  });
  it("rejects impossible native zero-address origins and duplicate transfer order", () => {
    const mintedNative = fundingView(
      operation([
        edge(ZERO, ROUTER, ZERO, "12", "0"),
        edge(ROUTER, CORE, TOKEN, "100", "1"),
      ]),
    );
    expect(mintedNative.rolesConfirmed).toBe(false);
    expect(mintedNative.warnings).toContain("invalid-evidence");
    const duplicate = fundingView(
      operation([
        edge(SOURCE, ROUTER, INPUT, "30", "0"),
        edge(ROUTER, CORE, INPUT, "30", "0"),
      ]),
    );
    expect(duplicate.fundedBy).toEqual([]);
    expect(duplicate.warnings).toContain("invalid-evidence");
  });
});
