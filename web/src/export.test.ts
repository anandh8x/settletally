import { describe, expect, it } from "vitest";
import { reportToCSV } from "./export";
import type { Report } from "./types";

describe("reportToCSV", () => {
  it("exports matched and unmatched payments", () => {
    const report: Report = {
      generatedAt: "2026-08-01T00:00:00Z",
      wallet: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
      fromBlock: 1,
      toBlock: 2,
      expectedCount: 1,
      paymentCount: 2,
      expectedMicros: 1_000_000,
      matchedMicros: 1_000_000,
      reviewMicros: 0,
      unmatchedMicros: 2_000_000,
      feeMicros: 0,
      matches: [{
        expected: { reference: "INV-1", direction: "inbound", amountMicros: 1_000_000, counterparty: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" },
        payments: [{ transactionHash: "0x1", blockNumber: 1, logIndex: 0, from: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", to: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", direction: "inbound", amountMicros: 1_000_000, feeMicros: 0 }],
        status: "matched",
        matchedMicros: 1_000_000,
        differenceMicros: 0,
        confidence: "high",
        reason: "unique exact amount and counterparty",
      }],
      unmatchedPayments: [{ transactionHash: "0x2", blockNumber: 2, logIndex: 0, from: "0xcccccccccccccccccccccccccccccccccccccccc", to: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", direction: "inbound", amountMicros: 2_000_000, feeMicros: 0, memoText: "unknown" }],
    };
    const csv = reportToCSV(report);
    expect(csv).toContain('"matched","INV-1"');
    expect(csv).toContain('"unmatched","","unmatched"');
  });

  it("neutralizes spreadsheet formulas", () => {
    const report = {
      generatedAt: "2026-08-01T00:00:00Z", wallet: "0x0", fromBlock: 1, toBlock: 1,
      expectedCount: 1, paymentCount: 0, expectedMicros: 1, matchedMicros: 0, reviewMicros: 0,
      unmatchedMicros: 0, feeMicros: 0, unmatchedPayments: [],
      matches: [{ expected: { reference: "=IMPORTXML(1)", direction: "inbound" as const, amountMicros: 1, counterparty: "0x0" }, payments: [], status: "missing_payment" as const, matchedMicros: 0, differenceMicros: 0, reason: "+unsafe", confidence: "none" as const }],
    };
    const csv = reportToCSV(report);
    expect(csv).toContain("'=IMPORTXML");
    expect(csv).toContain("'+unsafe");
  });
});
