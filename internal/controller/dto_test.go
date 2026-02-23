package controller

import (
	"fmt"
	"math"
	"testing"
)

func TestFloatToCents(t *testing.T) {
	tests := []struct {
		name    string
		input   float64
		want    int64
		wantErr bool
	}{
		{"normal", 123.45, 12345, false},
		{"zero", 0, 0, true},
		{"negative", -10.00, 0, true},
		{"max valid", 922337203685477.0, 92233720368547696, false}, // Actual result due to float64 precision
		{"overflow", 922337203685478.0, 0, true},
		{"huge overflow", 9999999999999999.99, 0, true},
		{"NaN", math.NaN(), 0, true},
		{"positive infinity", math.Inf(1), 0, true},
		{"negative infinity", math.Inf(-1), 0, true},
		{"min valid", 0.01, 1, false},
		{"rounding", 10.999, 1100, false},
		{"small amount", 1.00, 100, false},
		{"large amount", 100000.00, 10000000, false},
		{"exact max", maxAmountFloat, 92233720368547696, false}, // Actual result due to float64 precision
		{"just over max", maxAmountFloat + 1.0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := floatToCents(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("floatToCents() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("floatToCents() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCentsToFloat(t *testing.T) {
	tests := []struct {
		cents int64
		want  float64
	}{
		{12345, 123.45},
		{1, 0.01},
		{99, 0.99},
		{-99, -0.99}, // Negative display correct
		{92233720368547696, 922337203685476.96}, // Actual max that can be precisely represented
		{100, 1.00},
		{10000000, 100000.00},
		{0, 0.00},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d", tt.cents), func(t *testing.T) {
			got := centsToFloat(tt.cents)
			if got != tt.want {
				t.Errorf("centsToFloat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFloatToCentsRoundTrip(t *testing.T) {
	// Test that converting back and forth preserves values for valid amounts
	testValues := []float64{
		1.00,
		10.50,
		100.99,
		1000.01,
		12345.67,
	}

	for _, original := range testValues {
		t.Run(fmt.Sprintf("%.2f", original), func(t *testing.T) {
			cents, err := floatToCents(original)
			if err != nil {
				t.Fatalf("floatToCents() error = %v", err)
			}
			result := centsToFloat(cents)
			// Allow small floating point differences
			if math.Abs(result-original) > 0.01 {
				t.Errorf("Round trip failed: original=%v, result=%v", original, result)
			}
		})
	}
}
