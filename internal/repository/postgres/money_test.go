package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNumericStringToCents_Success(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"whole dollars", "100", 10000},
		{"dollars with cents", "100.50", 10050},
		{"cents only", "0.99", 99},
		{"zero", "0", 0},
		{"zero with decimals", "0.00", 0},
		{"small amount", "1.23", 123},
		{"large amount", "9999.99", 999999},
		{"rounding needed", "99.999", 10000},
		{"rounding down", "99.994", 9999},
		{"with whitespace", "  50.25  ", 5025},
		{"negative amount", "-10.50", -1050},
		{"single decimal", "5.5", 550},
		{"three decimals", "5.555", 556},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := numericStringToCents(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNumericStringToCents_Errors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"invalid format", "abc"},
		{"special characters", "$100.00"},
		{"multiple decimals", "10.5.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := numericStringToCents(tt.input)
			assert.Error(t, err)
		})
	}
}

func TestCentsToNumericString_Success(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected string
	}{
		{"whole dollars", 10000, "100.00"},
		{"dollars with cents", 10050, "100.50"},
		{"cents only", 99, "0.99"},
		{"zero", 0, "0.00"},
		{"small amount", 123, "1.23"},
		{"large amount", 999999, "9999.99"},
		{"negative amount", -1050, "-10.50"},
		{"negative cents", -99, "-0.99"},
		{"single cent", 1, "0.01"},
		{"ten cents", 10, "0.10"},
		{"exact dollar", 5000, "50.00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := centsToNumericString(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMoneyConversion_RoundTrip(t *testing.T) {
	// Test that converting cents -> string -> cents produces the same value
	tests := []int64{
		0,
		1,
		10,
		100,
		999,
		1000,
		10000,
		12345,
		999999,
		-100,
		-12345,
	}

	for _, original := range tests {
		t.Run("roundtrip", func(t *testing.T) {
			str := centsToNumericString(original)
			cents, err := numericStringToCents(str)
			require.NoError(t, err)
			assert.Equal(t, original, cents, "cents=%d, str=%s, back=%d", original, str, cents)
		})
	}
}

func TestMoneyConversion_EdgeCases(t *testing.T) {
	t.Run("very large amount", func(t *testing.T) {
		cents := int64(999999999999) // $9,999,999,999.99
		str := centsToNumericString(cents)
		back, err := numericStringToCents(str)
		require.NoError(t, err)
		assert.Equal(t, cents, back)
	})

	t.Run("very small negative", func(t *testing.T) {
		cents := int64(-1)
		str := centsToNumericString(cents)
		assert.Equal(t, "-0.01", str)
	})
}
