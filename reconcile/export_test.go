package reconcile

import (
	"bytes"
	"strings"
	"testing"
)

func TestCSVExportPreventsFormulaInjection(t *testing.T) {
	t.Parallel()
	report := Report{Matches: []Match{{
		Expected: &ExpectedRecord{
			Reference:    "=HYPERLINK(\"https://example.test\")",
			Direction:    DirectionInbound,
			AmountMicros: 1_000_000,
			Counterparty: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
		Status: StatusMissing,
	}}}
	var output bytes.Buffer
	if err := WriteReportCSV(&output, report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "'=HYPERLINK") {
		t.Fatalf("formula was not neutralized: %s", output.String())
	}
}
