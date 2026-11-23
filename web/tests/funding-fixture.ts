// Test fixture only; never imported by the production Explorer.
const addr = (n: number) => "0x" + n.toString(16).padStart(40, "0");
const [player, relay, router, core, delegate, token] = [1, 2, 3, 4, 5, 6].map(
  addr,
);
export const operation = {
  canonical: true,
  success: true,
  protocolAddress: core,
  funding: [{ payer: router, tokenAddress: token, amount: "100200000" }],
  fundingEvidence: {
    version: "core-funding-v1",
    coverage: "scoped-flow",
    transactionSender: relay,
    coreCallPath: "0/0/1",
    transfers: [
      {
        kind: "native",
        tokenAddress: addr(0),
        from: player,
        to: router,
        amount: "10000000000000000",
        callPath: "0/0",
        contextPath: "0",
        order: "0",
      },
      {
        kind: "erc20",
        logIndex: "0",
        tokenAddress: token,
        from: router,
        to: core,
        amount: "100200000",
        callPath: "0/0/1/0",
        contextPath: "0/0/1",
        order: "1",
      },
    ],
  },
  executionContext: {
    transactionType: "4",
    transactionSender: relay,
    transactionTo: player,
    coreCallPath: "0/0/1",
    calls: [
      { path: "0", type: "CALL", from: relay, to: player },
      { path: "0/0", type: "CALL", from: player, to: router },
      { path: "0/0/1", type: "CALL", from: router, to: core },
    ],
    authorizations: [
      {
        chainId: "97",
        address: delegate,
        authority: player,
        nonce: "8",
        signatureValid: true,
      },
    ],
  },
};
