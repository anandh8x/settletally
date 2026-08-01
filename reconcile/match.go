package reconcile

import (
	"strings"
	"time"
)

func BuildReport(wallet string, fromBlock, toBlock uint64, expected []ExpectedRecord, payments []Payment) Report {
	report := Report{
		GeneratedAt:       time.Now().UTC(),
		Wallet:            NormalizeAddress(wallet),
		FromBlock:         fromBlock,
		ToBlock:           toBlock,
		ExpectedCount:     len(expected),
		PaymentCount:      len(payments),
		Matches:           make([]Match, 0, len(expected)),
		UnmatchedPayments: make([]Payment, 0),
	}
	used := make([]bool, len(payments))

	for i := range expected {
		record := expected[i]
		report.ExpectedMicros += record.AmountMicros
		indices, reason, confidence := candidatesFor(record, payments, used)
		if len(indices) == 0 {
			status := StatusMissing
			reason := "no onchain payment satisfied the matching rules"
			if record.DueDate != nil && report.GeneratedAt.Before(record.DueDate.Add(24*time.Hour)) {
				status = StatusAwaiting
				reason = "no matching payment yet; the expected record is not past due"
			}
			report.Matches = append(report.Matches, Match{
				Expected:   &record,
				Payments:   make([]Payment, 0),
				Status:     status,
				Reason:     reason,
				Confidence: "none",
			})
			continue
		}

		match := Match{
			Expected:   &record,
			Payments:   make([]Payment, 0, len(indices)),
			Reason:     reason,
			Confidence: confidence,
		}
		counterpartyMismatch := false
		for _, index := range indices {
			used[index] = true
			payment := payments[index]
			match.Payments = append(match.Payments, payment)
			match.MatchedMicros += payment.AmountMicros
			if paymentCounterparty(payment) != record.Counterparty {
				counterpartyMismatch = true
			}
		}
		match.DifferenceMicros = match.MatchedMicros - record.AmountMicros
		switch {
		case counterpartyMismatch:
			match.Status = StatusNeedsReview
			match.Reason += "; memo matched but counterparty differs"
		case confidence == "low":
			match.Status = StatusNeedsReview
		case match.MatchedMicros < record.AmountMicros:
			match.Status = StatusPartial
		case match.MatchedMicros > record.AmountMicros:
			match.Status = StatusOverpaid
		default:
			match.Status = StatusMatched
		}
		switch match.Status {
		case StatusMatched, StatusPartial, StatusOverpaid:
			reconciledAmount := match.MatchedMicros
			if reconciledAmount > record.AmountMicros {
				reconciledAmount = record.AmountMicros
			}
			report.MatchedMicros += reconciledAmount
		case StatusNeedsReview:
			report.ReviewMicros += match.MatchedMicros
		}
		report.Matches = append(report.Matches, match)
	}

	for i, payment := range payments {
		report.FeeMicros += payment.FeeMicros
		if !used[i] {
			report.UnmatchedPayments = append(report.UnmatchedPayments, payment)
			report.UnmatchedMicros += payment.AmountMicros
		}
	}
	return report
}

func candidatesFor(record ExpectedRecord, payments []Payment, used []bool) ([]int, string, string) {
	if record.MemoID != "" {
		var indices []int
		for i, payment := range payments {
			if !used[i] && payment.Direction == record.Direction && strings.EqualFold(payment.MemoID, record.MemoID) {
				indices = append(indices, i)
			}
		}
		if len(indices) > 0 {
			return indices, "exact Arc memo ID", "high"
		}
	}

	var memoText []int
	for i, payment := range payments {
		if !used[i] && payment.Direction == record.Direction && normalizedText(payment.MemoText) == normalizedText(record.Reference) {
			memoText = append(memoText, i)
		}
	}
	if len(memoText) > 0 {
		return memoText, "exact memo text reference", "high"
	}

	var exact []int
	for i, payment := range payments {
		if !used[i] && payment.Direction == record.Direction && payment.AmountMicros == record.AmountMicros && paymentCounterparty(payment) == record.Counterparty {
			exact = append(exact, i)
		}
	}
	if len(exact) == 1 {
		return exact, "unique exact amount and counterparty", "high"
	}
	if len(exact) > 1 {
		return exact[:1], "multiple exact payments require duplicate review", "low"
	}

	var partial []int
	var partialTotal int64
	for i, payment := range payments {
		if !used[i] && payment.Direction == record.Direction && paymentCounterparty(payment) == record.Counterparty && payment.AmountMicros < record.AmountMicros {
			partial = append(partial, i)
			partialTotal += payment.AmountMicros
		}
	}
	if len(partial) > 0 && partialTotal <= record.AmountMicros {
		return partial, "counterparty-only partial candidates require manual confirmation", "low"
	}

	var amountOnly []int
	for i, payment := range payments {
		if !used[i] && payment.Direction == record.Direction && payment.AmountMicros == record.AmountMicros {
			amountOnly = append(amountOnly, i)
		}
	}
	if len(amountOnly) == 1 {
		return amountOnly, "amount-only candidate requires manual confirmation", "low"
	}
	return nil, "", "none"
}

func paymentCounterparty(payment Payment) string {
	switch payment.Direction {
	case DirectionInbound:
		return NormalizeAddress(payment.From)
	case DirectionOutbound:
		return NormalizeAddress(payment.To)
	default:
		return NormalizeAddress(payment.To)
	}
}

func normalizedText(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
