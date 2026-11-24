type RecordData = Record<string, any>;
export const isCreditOperation = (kind: unknown): boolean =>
  ["createCredit", "updateCredit", "revokeCredit", "creditDraw"].includes(String(kind));

export function creditGrantRefs(op: RecordData): RecordData[] {
  const refs: RecordData[] = Array.isArray(op.grantRefs) ? op.grantRefs : [];
  const funding: RecordData[] = Array.isArray(op.funding) ? op.funding : [];
  return refs.map(ref => {
    const payer = ref.payer ?? ref.owner ?? (/^(0|[1-9][0-9]*)$/.test(ref.payerIndex ?? "") ? funding[Number(ref.payerIndex)]?.payer : undefined);
    return {...ref, payer};
  });
}
export function creditSlotId(op: RecordData, ref: RecordData): string | undefined {
  if (!/^[1-9][0-9]*$/.test(String(op.chainId)) || !/^[1-9][0-9]*$/.test(String(ref.grantId))) return;
  if (BigInt(ref.grantId) >= 1n << 128n) return;
  if (![op.protocolAddress, op.tokenAddress, ref.payer].every(v => typeof v === "string" && /^0x[0-9a-fA-F]{40}$/.test(v))) return;
  return [op.chainId, op.protocolAddress.toLowerCase(), op.tokenAddress.toLowerCase(), ref.payer.toLowerCase(), ref.grantId].join(":");
}
