package reconcile

import (
	"fmt"
	"strconv"
	"strings"
)

func ParseUSDC(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("amount is required")
	}
	if strings.HasPrefix(value, "-") {
		return 0, fmt.Errorf("amount must not be negative")
	}

	parts := strings.Split(value, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid USDC amount %q", value)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid USDC amount %q: %w", value, err)
	}
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if len(fraction) > 6 {
		return 0, fmt.Errorf("USDC amount %q has more than 6 decimal places", value)
	}
	fraction += strings.Repeat("0", 6-len(fraction))
	fracMicros := int64(0)
	if fraction != "" {
		fracMicros, err = strconv.ParseInt(fraction, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid USDC amount %q: %w", value, err)
		}
	}
	if whole > (int64(^uint64(0)>>1)-fracMicros)/1_000_000 {
		return 0, fmt.Errorf("USDC amount %q is too large", value)
	}
	return whole*1_000_000 + fracMicros, nil
}

func FormatUSDC(micros int64) string {
	sign := ""
	if micros < 0 {
		sign = "-"
		micros = -micros
	}
	return fmt.Sprintf("%s%d.%06d", sign, micros/1_000_000, micros%1_000_000)
}
