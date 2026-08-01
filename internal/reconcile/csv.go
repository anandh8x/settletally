package reconcile

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"
)

func ReadExpectedCSV(reader io.Reader) ([]ExpectedRecord, error) {
	r := csv.NewReader(reader)
	r.TrimLeadingSpace = true
	head, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	columns := make(map[string]int, len(head))
	for i, value := range head {
		columns[strings.ToLower(strings.TrimSpace(value))] = i
	}
	for _, required := range []string{"reference", "direction", "amount", "counterparty"} {
		if _, ok := columns[required]; !ok {
			return nil, fmt.Errorf("missing required CSV column %q", required)
		}
	}

	var records []ExpectedRecord
	seenReferences := make(map[string]int)
	for rowNumber := 2; ; rowNumber++ {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row %d: %w", rowNumber, err)
		}
		get := func(name string) string {
			i, ok := columns[name]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}

		reference := get("reference")
		if reference == "" {
			return nil, fmt.Errorf("row %d: reference is required", rowNumber)
		}
		referenceKey := strings.ToLower(reference)
		if previousRow, exists := seenReferences[referenceKey]; exists {
			return nil, fmt.Errorf("row %d: reference %q duplicates row %d", rowNumber, reference, previousRow)
		}
		seenReferences[referenceKey] = rowNumber
		direction := Direction(strings.ToLower(get("direction")))
		if direction != DirectionInbound && direction != DirectionOutbound && direction != DirectionSelf {
			return nil, fmt.Errorf("row %d: direction must be inbound, outbound, or self", rowNumber)
		}
		amount, err := ParseUSDC(get("amount"))
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowNumber, err)
		}
		counterparty := NormalizeAddress(get("counterparty"))
		if !ValidAddress(counterparty) {
			return nil, fmt.Errorf("row %d: counterparty is not a valid EVM address", rowNumber)
		}

		var dueDate *time.Time
		if raw := get("due_date"); raw != "" {
			parsed, err := time.Parse("2006-01-02", raw)
			if err != nil {
				return nil, fmt.Errorf("row %d: due_date must use YYYY-MM-DD", rowNumber)
			}
			dueDate = &parsed
		}

		records = append(records, ExpectedRecord{
			Reference:    reference,
			Direction:    direction,
			AmountMicros: amount,
			Counterparty: counterparty,
			DueDate:      dueDate,
			MemoID:       strings.ToLower(get("memo_id")),
		})
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("CSV contains no expected records")
	}
	return records, nil
}
