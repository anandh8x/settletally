package reconcile

import (
	"testing"
	"time"
)

func TestBuildReportUsesMemoAndAggregatesPartialPayments(t *testing.T) {
	t.Parallel()
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	counterparty := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	expected := []ExpectedRecord{{
		Reference:    "INV-42",
		Direction:    DirectionInbound,
		AmountMicros: 3_000_000,
		Counterparty: counterparty,
		MemoID:       "0x1234",
	}}
	payments := []Payment{
		{TransactionHash: "0x1", Direction: DirectionInbound, From: counterparty, To: wallet, AmountMicros: 1_000_000, MemoID: "0x1234", FeeMicros: 10},
		{TransactionHash: "0x2", Direction: DirectionInbound, From: counterparty, To: wallet, AmountMicros: 2_000_000, MemoID: "0x1234", FeeMicros: 20},
	}
	report := BuildReport(wallet, 1, 2, expected, payments)
	if len(report.Matches) != 1 || report.Matches[0].Status != StatusMatched {
		t.Fatalf("unexpected match: %#v", report.Matches)
	}
	if report.Matches[0].MatchedMicros != 3_000_000 {
		t.Fatalf("matched amount = %d", report.Matches[0].MatchedMicros)
	}
	if report.FeeMicros != 30 {
		t.Fatalf("fee total = %d", report.FeeMicros)
	}
}

func TestBuildReportDoesNotTrustMemoWithWrongCounterparty(t *testing.T) {
	t.Parallel()
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	expected := []ExpectedRecord{{
		Reference:    "INV-42",
		Direction:    DirectionInbound,
		AmountMicros: 1_000_000,
		Counterparty: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		MemoID:       "0x1234",
	}}
	payments := []Payment{{
		TransactionHash: "0x1",
		Direction:       DirectionInbound,
		From:            "0xcccccccccccccccccccccccccccccccccccccccc",
		To:              wallet,
		AmountMicros:    1_000_000,
		MemoID:          "0x1234",
	}}
	report := BuildReport(wallet, 1, 1, expected, payments)
	if report.Matches[0].Status != StatusNeedsReview {
		t.Fatalf("status = %s, want needs_review", report.Matches[0].Status)
	}
	if report.MatchedMicros != 0 || report.ReviewMicros != 1_000_000 {
		t.Fatalf("review payment leaked into matched total: %#v", report)
	}
}

func TestBuildReportCapsReconciledTotalAtExpectedAmount(t *testing.T) {
	t.Parallel()
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	counterparty := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	report := BuildReport(wallet, 1, 1, []ExpectedRecord{{
		Reference:    "INV-OVER",
		Direction:    DirectionInbound,
		AmountMicros: 1_000_000,
		Counterparty: counterparty,
		MemoID:       "0x1234",
	}}, []Payment{{
		TransactionHash: "0x1",
		Direction:       DirectionInbound,
		From:            counterparty,
		To:              wallet,
		AmountMicros:    1_250_000,
		MemoID:          "0x1234",
	}})
	if report.Matches[0].Status != StatusOverpaid || report.MatchedMicros != 1_000_000 {
		t.Fatalf("unexpected overpayment totals: %#v", report)
	}
}

func TestBuildReportLeavesUnknownPaymentUnmatched(t *testing.T) {
	t.Parallel()
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	expected := []ExpectedRecord{{
		Reference:    "INV-42",
		Direction:    DirectionInbound,
		AmountMicros: 2_000_000,
		Counterparty: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}}
	payments := []Payment{{
		TransactionHash: "0x1",
		Direction:       DirectionOutbound,
		From:            wallet,
		To:              "0xcccccccccccccccccccccccccccccccccccccccc",
		AmountMicros:    1_000_000,
	}}
	report := BuildReport(wallet, 1, 1, expected, payments)
	if report.Matches[0].Status != StatusMissing || len(report.UnmatchedPayments) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
}

func TestBuildReportKeepsFutureRecordAwaitingPayment(t *testing.T) {
	t.Parallel()
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	dueDate := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	report := BuildReport(wallet, 1, 1, []ExpectedRecord{{
		Reference:    "INV-FUTURE",
		Direction:    DirectionInbound,
		AmountMicros: 1_000_000,
		Counterparty: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		DueDate:      &dueDate,
	}}, nil)
	if report.Matches[0].Status != StatusAwaiting {
		t.Fatalf("status = %s, want awaiting_payment", report.Matches[0].Status)
	}
	if report.Matches[0].Payments == nil {
		t.Fatal("empty payments must be represented as an array")
	}
}

func TestBuildReportDoesNotAutoAcceptCounterpartyOnlyPartials(t *testing.T) {
	t.Parallel()
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	counterparty := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	report := BuildReport(wallet, 1, 2, []ExpectedRecord{{
		Reference:    "INV-AMBIGUOUS",
		Direction:    DirectionInbound,
		AmountMicros: 3_000_000,
		Counterparty: counterparty,
	}}, []Payment{
		{TransactionHash: "0x1", Direction: DirectionInbound, From: counterparty, To: wallet, AmountMicros: 1_000_000},
		{TransactionHash: "0x2", Direction: DirectionInbound, From: counterparty, To: wallet, AmountMicros: 1_000_000},
	})
	if report.Matches[0].Status != StatusNeedsReview || report.MatchedMicros != 0 || report.ReviewMicros != 2_000_000 {
		t.Fatalf("ambiguous partials were presented as reconciled: %#v", report)
	}
}

func TestTenRecordValidationMatrix(t *testing.T) {
	t.Parallel()
	wallet := "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	b := "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	c := "0xcccccccccccccccccccccccccccccccccccccccc"
	d := "0xdddddddddddddddddddddddddddddddddddddddd"
	records := []ExpectedRecord{
		{Reference: "R1", Direction: DirectionInbound, AmountMicros: 1_000_000, Counterparty: b, MemoID: "0x01"},
		{Reference: "R2", Direction: DirectionInbound, AmountMicros: 2_000_000, Counterparty: b, MemoID: "0x02"},
		{Reference: "R3", Direction: DirectionInbound, AmountMicros: 3_000_000, Counterparty: b, MemoID: "0x03"},
		{Reference: "R4", Direction: DirectionInbound, AmountMicros: 4_000_000, Counterparty: b},
		{Reference: "R5", Direction: DirectionInbound, AmountMicros: 5_000_000, Counterparty: b},
		{Reference: "R6", Direction: DirectionInbound, AmountMicros: 6_000_000, Counterparty: b},
		{Reference: "R7", Direction: DirectionInbound, AmountMicros: 7_000_000, Counterparty: b, MemoID: "0x07"},
		{Reference: "R8", Direction: DirectionInbound, AmountMicros: 8_000_000, Counterparty: b},
		{Reference: "R9", Direction: DirectionOutbound, AmountMicros: 9_000_000, Counterparty: c},
		{Reference: "R10", Direction: DirectionSelf, AmountMicros: 10_000_000, Counterparty: wallet, MemoID: "0x10"},
	}
	payments := []Payment{
		{TransactionHash: "0x01", Direction: DirectionInbound, From: b, To: wallet, AmountMicros: 1_000_000, MemoID: "0x01"},
		{TransactionHash: "0x02", Direction: DirectionInbound, From: b, To: wallet, AmountMicros: 1_250_000, MemoID: "0x02"},
		{TransactionHash: "0x03", Direction: DirectionInbound, From: b, To: wallet, AmountMicros: 3_250_000, MemoID: "0x03"},
		{TransactionHash: "0x04", Direction: DirectionInbound, From: b, To: wallet, AmountMicros: 4_000_000},
		{TransactionHash: "0x05", Direction: DirectionInbound, From: c, To: wallet, AmountMicros: 5_000_000},
		{TransactionHash: "0x07", Direction: DirectionInbound, From: d, To: wallet, AmountMicros: 7_000_000, MemoID: "0x07"},
		{TransactionHash: "0x08a", Direction: DirectionInbound, From: b, To: wallet, AmountMicros: 8_000_000},
		{TransactionHash: "0x08b", Direction: DirectionInbound, From: b, To: wallet, AmountMicros: 8_000_000},
		{TransactionHash: "0x09", Direction: DirectionOutbound, From: wallet, To: c, AmountMicros: 9_000_000},
		{TransactionHash: "0x10", Direction: DirectionSelf, From: wallet, To: wallet, AmountMicros: 10_000_000, MemoID: "0x10"},
	}
	report := BuildReport(wallet, 1, 10, records, payments)
	want := []MatchStatus{
		StatusMatched,
		StatusPartial,
		StatusOverpaid,
		StatusMatched,
		StatusNeedsReview,
		StatusMissing,
		StatusNeedsReview,
		StatusNeedsReview,
		StatusMatched,
		StatusMatched,
	}
	if len(report.Matches) != len(want) {
		t.Fatalf("match count = %d, want %d", len(report.Matches), len(want))
	}
	for i, status := range want {
		if report.Matches[i].Status != status {
			t.Fatalf("record %d status = %s, want %s", i+1, report.Matches[i].Status, status)
		}
	}
	if len(report.UnmatchedPayments) != 1 || report.UnmatchedPayments[0].TransactionHash != "0x08b" {
		t.Fatalf("unexpected unmatched payments: %#v", report.UnmatchedPayments)
	}
}
