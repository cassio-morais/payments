package postgres

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// numericStringToCents converts a PostgreSQL NUMERIC string to int64 cents.
// Examples: "100.50" → 10050, "99.999" → 10000 (rounded), "50" → 5000
func numericStringToCents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty numeric string")
	}

	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("parse numeric %q: %w", s, err)
	}

	return int64(math.Round(f * 100)), nil
}

// centsToNumericString converts int64 cents to a string suitable for PostgreSQL NUMERIC.
// Examples: 10050 → "100.50", 5000 → "50.00", -150 → "-1.50"
func centsToNumericString(cents int64) string {
	whole := cents / 100
	frac := cents % 100
	if frac < 0 {
		frac = -frac
	}
	return fmt.Sprintf("%d.%02d", whole, frac)
}
