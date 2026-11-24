import { exactAmount } from "./public";

// Keep display rounding separate from the exact decimal copied by the user.
// Amounts never pass through JavaScript's floating point Number type.
export function amountView(value: unknown, decimals?: number | string, compact = false) {
  const formatted = exactAmount(value, decimals);
  const raw = formatted.endsWith(" (raw)");
  const exact = raw ? formatted.slice(0, -6) : formatted;
  const valid = formatted !== "—";
  const copyValue = exact.replaceAll(",", "");
  let display = exact;
  if (valid && !raw && compact && Number(decimals) > 6) {
    const amount = BigInt(String(value));
    const negative = amount < 0n;
    const absolute = negative ? -amount : amount;
    const scale = 10n ** BigInt(Number(decimals) - 6);
    const remainder = absolute % scale;
    if (absolute > 0n && absolute < scale) {
      display = negative ? ">-0.000001" : "<0.000001";
    } else if (remainder !== 0n) {
      const rounded = absolute / scale + (remainder * 2n >= scale ? 1n : 0n);
      display = "≈" + exactAmount((negative ? -rounded : rounded).toString(), 6);
    }
  }
  return { display, exact, copyValue, raw, valid };
}
