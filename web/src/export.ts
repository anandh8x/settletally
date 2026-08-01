import { formatUSDC } from "./format";
import type { Report } from "./types";

const header = [
  "record_type",
  "reference",
  "status",
  "direction",
  "expected_usdc",
  "settled_usdc",
  "difference_usdc",
  "counterparty",
  "transaction_hash",
  "memo_id",
  "memo_text",
  "fee_usdc",
  "confidence",
  "reason",
];

export function reportToCSV(report: Report): string {
  const rows: string[][] = [header];
  for (const match of report.matches) {
    if (match.payments.length === 0) {
      rows.push([
        "expected",
        match.expected.reference,
        match.status,
        match.expected.direction,
        formatUSDC(match.expected.amountMicros),
        "0.00",
        formatUSDC(match.differenceMicros),
        match.expected.counterparty,
        "",
        match.expected.memoId ?? "",
        "",
        "0.00",
        match.confidence,
        match.reason,
      ]);
      continue;
    }
    for (const payment of match.payments) {
      rows.push([
        "matched",
        match.expected.reference,
        match.status,
        payment.direction,
        formatUSDC(match.expected.amountMicros),
        formatUSDC(payment.amountMicros),
        formatUSDC(match.differenceMicros),
        payment.direction === "inbound" ? payment.from : payment.to,
        payment.transactionHash,
        payment.memoId ?? "",
        payment.memoText ?? "",
        formatUSDC(payment.feeMicros),
        match.confidence,
        match.reason,
      ]);
    }
  }
  for (const payment of report.unmatchedPayments) {
    rows.push([
      "unmatched",
      "",
      "unmatched",
      payment.direction,
      "0.00",
      formatUSDC(payment.amountMicros),
      formatUSDC(payment.amountMicros),
      payment.direction === "inbound" ? payment.from : payment.to,
      payment.transactionHash,
      payment.memoId ?? "",
      payment.memoText ?? "",
      formatUSDC(payment.feeMicros),
      "none",
      "no expected record matched this onchain payment",
    ]);
  }
  return rows.map((row) => row.map(csvCell).join(",")).join("\r\n") + "\r\n";
}

function csvCell(value: string): string {
  const safe = /^[=+\-@]/.test(value.trimStart()) ? `'${value}` : value;
  return `"${safe.replaceAll('"', '""')}"`;
}
