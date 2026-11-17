package helper

import (
	"strconv"
	"strings"
)

// FormatRupiah memformat rupiah utuh untuk ditampilkan, dengan titik pemisah
// ribuan: 1500000 menjadi "Rp1.500.000". Nilai negatif menjadi "-Rp1.500".
// Hanya untuk tampilan; perhitungan tetap memakai int64 apa adanya.
func FormatRupiah(n int64) string {
	u := uint64(n)
	if n < 0 {
		u = -u // aman juga untuk math.MinInt64
	}

	digit := strconv.FormatUint(u, 10)

	var b strings.Builder
	if n < 0 {
		b.WriteByte('-')
	}
	b.WriteString("Rp")

	for i, r := range digit {
		if i > 0 && (len(digit)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}

	return b.String()
}
