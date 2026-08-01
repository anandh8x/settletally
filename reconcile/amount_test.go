package reconcile

import "testing"

func TestParseAndFormatUSDC(t *testing.T) {
	t.Parallel()
	tests := []struct {
		input  string
		micros int64
	}{
		{"0", 0},
		{"1", 1_000_000},
		{"1.25", 1_250_000},
		{"0.000001", 1},
		{" 42.000010 ", 42_000_010},
	}
	for _, test := range tests {
		actual, err := ParseUSDC(test.input)
		if err != nil {
			t.Fatalf("ParseUSDC(%q): %v", test.input, err)
		}
		if actual != test.micros {
			t.Fatalf("ParseUSDC(%q) = %d, want %d", test.input, actual, test.micros)
		}
	}
	if got := FormatUSDC(1_250_000); got != "1.250000" {
		t.Fatalf("FormatUSDC = %q", got)
	}
}

func TestParseUSDCRejectsUnsafeValues(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", "-1", "1.0000001", "abc", "1.2.3"} {
		if _, err := ParseUSDC(input); err == nil {
			t.Fatalf("ParseUSDC(%q) unexpectedly succeeded", input)
		}
	}
}
