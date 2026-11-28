package helper

import (
	"strings"
	"unicode"
)

// Inisial mengambil huruf pertama dari paling banyak dua kata pertama nama,
// huruf besar. "Budi Pratama" menjadi "BP"; nama kosong menjadi "?".
// Hanya untuk tampilan (avatar).
func Inisial(nama string) string {
	var hasil []rune
	for _, kata := range strings.Fields(nama) {
		if len(hasil) == 2 {
			break
		}
		for _, r := range kata {
			if unicode.IsLetter(r) {
				hasil = append(hasil, unicode.ToUpper(r))
				break
			}
		}
	}
	if len(hasil) == 0 {
		return "?"
	}
	return string(hasil)
}
