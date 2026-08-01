package reconcile

import "strings"

func NormalizeAddress(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func ValidAddress(value string) bool {
	value = NormalizeAddress(value)
	if len(value) != 42 || !strings.HasPrefix(value, "0x") {
		return false
	}
	for _, r := range value[2:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
