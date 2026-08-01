package reconcile

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func WriteReportJSON(writer io.Writer, report Report) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func WriteReportCSV(writer io.Writer, report Report) error {
	w := csv.NewWriter(writer)
	defer w.Flush()
	if err := w.Write([]string{
		"record_type", "reference", "status", "direction", "expected_usdc", "paid_usdc",
		"difference_usdc", "counterparty", "transaction_hash", "memo_id", "memo_text",
		"fee_usdc", "confidence", "reason",
	}); err != nil {
		return err
	}
	for _, match := range report.Matches {
		if match.Expected == nil {
			continue
		}
		if len(match.Payments) == 0 {
			if err := w.Write(safeCSVRow([]string{
				"expected", match.Expected.Reference, string(match.Status), string(match.Expected.Direction),
				FormatUSDC(match.Expected.AmountMicros), "0.000000", FormatUSDC(match.DifferenceMicros),
				match.Expected.Counterparty, "", match.Expected.MemoID, "", "0.000000", match.Confidence, match.Reason,
			})); err != nil {
				return err
			}
			continue
		}
		for _, payment := range match.Payments {
			if err := w.Write(safeCSVRow([]string{
				"matched", match.Expected.Reference, string(match.Status), string(payment.Direction),
				FormatUSDC(match.Expected.AmountMicros), FormatUSDC(payment.AmountMicros), FormatUSDC(match.DifferenceMicros),
				paymentCounterparty(payment), payment.TransactionHash, payment.MemoID, payment.MemoText,
				FormatUSDC(payment.FeeMicros), match.Confidence, match.Reason,
			})); err != nil {
				return err
			}
		}
	}
	for _, payment := range report.UnmatchedPayments {
		if err := w.Write(safeCSVRow([]string{
			"unmatched", "", string(StatusUnmatched), string(payment.Direction), "0.000000",
			FormatUSDC(payment.AmountMicros), FormatUSDC(payment.AmountMicros), paymentCounterparty(payment),
			payment.TransactionHash, payment.MemoID, payment.MemoText, FormatUSDC(payment.FeeMicros),
			"none", "no expected record matched this onchain payment",
		})); err != nil {
			return err
		}
	}
	if err := w.Error(); err != nil {
		return fmt.Errorf("write reconciliation CSV: %w", err)
	}
	return nil
}

func safeCSVRow(row []string) []string {
	result := make([]string, len(row))
	for i, value := range row {
		trimmed := strings.TrimLeft(value, " \t\r\n")
		if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "+") || strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, "@") {
			value = "'" + value
		}
		result[i] = value
	}
	return result
}
