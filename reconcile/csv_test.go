package reconcile

import (
	"strings"
	"testing"
)

func TestReadExpectedCSV(t *testing.T) {
	t.Parallel()
	input := `reference,direction,amount,counterparty,due_date,memo_id
INV-101,inbound,12.50,0x1111111111111111111111111111111111111111,2026-08-01,0xabc
`
	records, err := ReadExpectedCSV(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].AmountMicros != 12_500_000 {
		t.Fatalf("unexpected records: %#v", records)
	}
	if records[0].Counterparty != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("address was not normalized: %s", records[0].Counterparty)
	}
}

func TestReadExpectedCSVRejectsInvalidRows(t *testing.T) {
	t.Parallel()
	tests := []string{
		"reference,direction,amount\nINV-1,inbound,1\n",
		"reference,direction,amount,counterparty\nINV-1,sideways,1,0x1111111111111111111111111111111111111111\n",
		"reference,direction,amount,counterparty\nINV-1,inbound,1,not-an-address\n",
		"reference,direction,amount,counterparty\nINV-1,inbound,1,0x1111111111111111111111111111111111111111\ninv-1,inbound,2,0x2222222222222222222222222222222222222222\n",
	}
	for _, input := range tests {
		if _, err := ReadExpectedCSV(strings.NewReader(input)); err == nil {
			t.Fatalf("invalid CSV unexpectedly succeeded: %q", input)
		}
	}
}
