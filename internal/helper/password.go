package helper

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	skemaPassword   = "pbkdf2-sha256"
	iterasiPassword = 600_000
	panjangSalt     = 16
	panjangKunci    = 32
	iterasiMaks     = 10_000_000
)

var ErrFormatHashTidakValid = errors.New("helper: format hash kata sandi tidak valid")

// HashPassword menghasilkan hash kata sandi dengan salt acak. Hasilnya berformat
// pbkdf2-sha256$<iterasi>$<salt base64>$<hash base64>
func HashPassword(password string) (string, error) {
	return hashPasswordDenganIterasi(password, iterasiPassword)
}

func hashPasswordDenganIterasi(password string, iterasi int) (string, error) {
	salt := make([]byte, panjangSalt)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("helper: error membuat salt. %w", err)
	}

	kunci, err := pbkdf2.Key(sha256.New, password, salt, iterasi, panjangKunci)
	if err != nil {
		return "", fmt.Errorf("helper: error menurunkan kunci. %w", err)
	}

	return fmt.Sprintf("%s$%d$%s$%s", skemaPassword, iterasi, base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(kunci)), nil
}

// MatchPassword memeriksa apakah password sesuai dengan hash dari HashPassword
func CocokPassword(password, hash string) (bool, error) {
	bagian := strings.Split(hash, "$")
	if len(bagian) != 4 || bagian[0] != skemaPassword {
		return false, ErrFormatHashTidakValid
	}

	iterasi, err := strconv.Atoi(bagian[1])
	if err != nil || iterasi < 1 || iterasi > iterasiMaks {
		return false, ErrFormatHashTidakValid
	}

	salt, err := base64.RawStdEncoding.DecodeString(bagian[2])
	if err != nil || len(salt) == 0 {
		return false, ErrFormatHashTidakValid
	}

	expected, err := base64.RawStdEncoding.DecodeString(bagian[3])
	if err != nil || len(expected) == 0 {
		return false, ErrFormatHashTidakValid
	}

	kunci, err := pbkdf2.Key(sha256.New, password, salt, iterasi, len(expected))
	if err != nil {
		return false, fmt.Errorf("helper: error menurunkan kunci. %w", err)
	}

	return subtle.ConstantTimeCompare(kunci, expected) == 1, nil
}
