export type Direction = "inbound" | "outbound" | "self";

export type MatchStatus =
  | "matched"
  | "partial"
  | "overpaid"
  | "needs_review"
  | "unmatched"
  | "missing_payment"
  | "awaiting_payment";

export interface ExpectedInput {
  reference: string;
  direction: Direction;
  amount: string;
  counterparty: string;
  dueDate?: string;
  memoId?: string;
}

export interface ExpectedRecord {
  reference: string;
  direction: Direction;
  amountMicros: number;
  counterparty: string;
  dueDate?: string;
  memoId?: string;
}

export interface Payment {
  transactionHash: string;
  blockNumber: number;
  logIndex: number;
  from: string;
  to: string;
  direction: Direction;
  amountMicros: number;
  feeMicros: number;
  memoId?: string;
  memoText?: string;
}

export interface ReconciliationMatch {
  expected: ExpectedRecord;
  payments: Payment[];
  status: MatchStatus;
  matchedMicros: number;
  differenceMicros: number;
  reason: string;
  confidence: "high" | "medium" | "low" | "none";
}

export interface Report {
  generatedAt: string;
  wallet: string;
  fromBlock: number;
  toBlock: number;
  expectedCount: number;
  paymentCount: number;
  expectedMicros: number;
  matchedMicros: number;
  reviewMicros: number;
  unmatchedMicros: number;
  feeMicros: number;
  matches: ReconciliationMatch[];
  unmatchedPayments: Payment[];
}
