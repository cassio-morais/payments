package postgres

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

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

func centsToNumericString(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}

	whole := cents / 100
	frac := cents % 100

	return fmt.Sprintf("%s%d.%02d", sign, whole, frac)
}
