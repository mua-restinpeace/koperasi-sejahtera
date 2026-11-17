package helper

import (
	"math"
	"testing"
)

func TestFormatRupiah(t *testing.T) {
	tests := []struct {
		nama string
		n    int64
		want string
	}{
		{"nol", 0, "Rp0"},
		{"satu digit", 5, "Rp5"},
		{"tiga digit tanpa pemisah", 999, "Rp999"},
		{"ribuan", 1_000, "Rp1.000"},
		{"puluhan ribu", 12_345, "Rp12.345"},
		{"ratusan ribu", 500_000, "Rp500.000"},
		{"jutaan", 1_500_000, "Rp1.500.000"},
		{"puluhan juta", 10_000_000, "Rp10.000.000"},
		{"negatif", -1_500, "-Rp1.500"},
		{"negatif tiga digit", -999, "-Rp999"},
		{"int64 terbesar", math.MaxInt64, "Rp9.223.372.036.854.775.807"},
		{"int64 terkecil tidak overflow", math.MinInt64, "-Rp9.223.372.036.854.775.808"},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			if got := FormatRupiah(tt.n); got != tt.want {
				t.Errorf("FormatRupiah(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}
