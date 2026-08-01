import type { MatchStatus } from "./types";

export function formatUSDC(micros: number): string {
  return new Intl.NumberFormat("en-US", {
    minimumFractionDigits: 2,
    maximumFractionDigits: 6,
  }).format(micros / 1_000_000);
}

export function shortHash(value: string): string {
  return value.length > 16 ? `${value.slice(0, 8)}…${value.slice(-6)}` : value;
}

export function statusLabel(status: MatchStatus): string {
  const labels: Record<MatchStatus, string> = {
    matched: "Matched",
    partial: "Partially paid",
    overpaid: "Overpaid",
    needs_review: "Needs review",
    unmatched: "Unmatched",
    missing_payment: "Missing payment",
    awaiting_payment: "Awaiting payment",
  };
  return labels[status];
}
