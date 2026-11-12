package helper

import (
	"errors"
	"strings"
	"testing"
)

const iterasiUji = 1_000 // cukup untuk tes; iterasi sebenarnya (600.000) lambat

func TestPassword_HashDanCocok(t *testing.T) {
	hash, err := hashPasswordDenganIterasi("rahasia-123", iterasiUji)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(hash, "pbkdf2-sha256$1000$") {
		t.Errorf("format hash = %q", hash)
	}

	tests := []struct {
		nama     string
		password string
		want     bool
	}{
		{"password benar", "rahasia-123", true},
		{"password salah", "rahasia-124", false},
		{"huruf besar kecil dibedakan", "Rahasia-123", false},
		{"kosong", "", false},
		{"awalan saja", "rahasia", false},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			got, err := CocokPassword(tt.password, hash)
			if err != nil {
				t.Fatalf("CocokPassword: %v", err)
			}
			if got != tt.want {
				t.Errorf("cocok = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPassword_SaltAcak(t *testing.T) {
	a, _ := hashPasswordDenganIterasi("sama", iterasiUji)
	b, _ := hashPasswordDenganIterasi("sama", iterasiUji)
	if a == b {
		t.Error("dua hash dari password yang sama harus berbeda (salt acak)")
	}
}

func TestPassword_FormatTidakValid(t *testing.T) {
	valid, _ := hashPasswordDenganIterasi("x", iterasiUji)
	bagian := strings.Split(valid, "$")
	tests := []struct {
		nama string
		hash string
	}{
		{"kosong", ""},
		{"bukan hash", "rahasia"},
		{"skema lain", "bcrypt$1000$" + bagian[2] + "$" + bagian[3]},
		{"terlalu sedikit bagian", "pbkdf2-sha256$1000$" + bagian[2]},
		{"iterasi bukan angka", "pbkdf2-sha256$banyak$" + bagian[2] + "$" + bagian[3]},
		{"iterasi nol", "pbkdf2-sha256$0$" + bagian[2] + "$" + bagian[3]},
		{"iterasi berlebihan", "pbkdf2-sha256$99999999999$" + bagian[2] + "$" + bagian[3]},
		{"salt bukan base64", "pbkdf2-sha256$1000$***$" + bagian[3]},
		{"hash bukan base64", "pbkdf2-sha256$1000$" + bagian[2] + "$***"},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			if ok, err := CocokPassword("x", tt.hash); !errors.Is(err, ErrFormatHashTidakValid) || ok {
				t.Errorf("CocokPassword = %v, %v; want false dan ErrFormatHashTidakValid", ok, err)
			}
		})
	}
}

// Satu tes dengan iterasi sebenarnya, agar perubahan konstanta yang tidak sengaja ketahuan.
func TestHashPassword_IterasiSebenarnya(t *testing.T) {
	hash, err := HashPassword("kata-sandi-panjang")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if !strings.HasPrefix(hash, "pbkdf2-sha256$600000$") {
		t.Errorf("iterasi bukan 600000: %q", hash)
	}
	if ok, err := CocokPassword("kata-sandi-panjang", hash); err != nil || !ok {
		t.Errorf("CocokPassword = %v, %v", ok, err)
	}
}
