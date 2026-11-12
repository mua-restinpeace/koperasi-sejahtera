package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

type PenggunaRepo interface {
	CariByUsername(ctx context.Context, db database.DBTX, username string) (model.Pengguna, error)
}

type SesiRepo interface {
	Create(ctx context.Context, db database.DBTX, s model.Sesi) error
	CariAktif(ctx context.Context, db database.DBTX, tokenHash string, sekarang time.Time) (model.Sesi, model.Pengguna, error)
	HapusByTokenHash(ctx context.Context, db database.DBTX, tokenHash string) error
	HapusKedaluwarsa(ctx context.Context, db database.DBTX, sekarang time.Time) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Cocok(password, hash string) (bool, error)
}

// HasherPBKDF2 adalah PasswordHasher produksi
type HasherPBKDF2 struct{}

// Hash menghasilkan hash PBKDF2
func (HasherPBKDF2) Hash(p string) (string, error) {
	return helper.HashPassword(p)
}

// Cocok membandingkan kata sandi dengan hash
func (HasherPBKDF2) Cocok(p, h string) (bool, error) {
	return helper.CocokPassword(p, h)
}

type AuthService struct {
	db       database.Sumber
	pengguna PenggunaRepo
	sesiRepo SesiRepo
	hasher   PasswordHasher
	durasi   time.Duration
	now      func() time.Time
	log      *slog.Logger

	dummyOnce sync.Once
	dummyHash string
}

// NewAuthService membuat AuthService, now boleh nil (dipakai time.Now)
func NewAuthService(db database.Sumber, penggunaRepo PenggunaRepo, sesiRepo SesiRepo, hasher PasswordHasher, durasi time.Duration, now func() time.Time, log *slog.Logger) *AuthService {
	if now == nil {
		now = time.Now
	}

	return &AuthService{db: db, pengguna: penggunaRepo, sesiRepo: sesiRepo, hasher: hasher, durasi: durasi, now: now, log: log}
}

// Login memeriksa kredensial dan membuat sesi. Token yang dikemmbalikan hanya
// untuk dipasang ke cookie; yang tersimpan di database adalah hash-nya.
// Kegagalan login selalu ErrKredensialSalah, tanpa membedakan username tidak ada,
// kata sandi salah, atau akun nonaktif
func (s *AuthService) Login(ctx context.Context, username, password string) (token string, sesi model.Sesi, err error) {
	username = strings.TrimSpace(username)

	p, err := s.pengguna.CariByUsername(ctx, s.db, username)
	switch {
	case errors.Is(err, repository.ErrTidakDitemukan):
		// tetap menghitung hash supaya waktu response tidak membocorkan keberadaan usename
		_, _ = s.hasher.Cocok(password, s.hashPalsu())
		s.log.Warn("login gagal", "username", username, "sebab", "username tidak ditemukan")
		return "", model.Sesi{}, ErrKredensialSalah
	case err != nil:
		return "", model.Sesi{}, fmt.Errorf("login: %w", err)
	}

	ok, err := s.hasher.Cocok(password, p.PasswordHash)
	if err != nil {
		return "", model.Sesi{}, fmt.Errorf("login: password tidak cocok. %w", err)
	}

	if !ok || !p.Aktif {
		s.log.Warn("login gagal", "username", username, "sebab", "password salah atau akun nonaktif")
		return "", model.Sesi{}, ErrKredensialSalah
	}

	token, err = tokenAcak(32)
	if err != nil {
		return "", model.Sesi{}, err
	}

	csrf, err := tokenAcak(32)
	if err != nil {
		return "", model.Sesi{}, err
	}

	sekarang := s.now().UTC()
	sesi = model.Sesi{
		IDPengguna:      p.ID,
		TokenHash:       hashToken(token),
		CSRFToken:       csrf,
		DibuatPada:      sekarang,
		KedaluwarsaPada: sekarang.Add(s.durasi),
	}

	if err := s.sesiRepo.Create(ctx, s.db, sesi); err != nil {
		return "", model.Sesi{}, fmt.Errorf("login: %w", err)
	}

	// Pembersihan oportunistik
	if err := s.sesiRepo.HapusKedaluwarsa(ctx, s.db, sekarang); err != nil {
		s.log.Warn("hapus sesi kedaluwarsa", "error", err)
	}

	return token, sesi, nil
}

// Logout menghapus sesi milik token. Token kosong atau tidak dikenal bukan kesalahan.
func (s *AuthService) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.sesiRepo.HapusByTokenHash(ctx, s.db, hashToken(token)); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

// Authenticate mengobah token cookie menjadi Identitas.
// ErrSesiTidakValid bila token tidak dikenal, kedaluwarsa, atau pengguna nonaktif
func (s *AuthService) Authenticate(ctx context.Context, token string) (Identitas, error) {
	if token == "" {
		return Identitas{}, ErrSesiTidakValid
	}

	sesi, p, err := s.sesiRepo.CariAktif(ctx, s.db, hashToken(token), s.now().UTC())
	if errors.Is(err, repository.ErrTidakDitemukan) {
		return Identitas{}, ErrSesiTidakValid
	}

	if err != nil {
		return Identitas{}, fmt.Errorf("authenticate: %w", err)
	}

	return Identitas{IDPengguna: p.ID, Username: p.Username, NamaLengkap: p.NamaLengkap, CSRFToken: sesi.CSRFToken}, nil
}

// HELPER //
func (s *AuthService) hashPalsu() string {
	s.dummyOnce.Do(func() {
		h, err := s.hasher.Hash("palsu-untuk-menyamakan-waktu")
		if err != nil {
			h = ""
		}
		s.dummyHash = h
	})

	return s.dummyHash
}

func tokenAcak(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("error membuat token acak. %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
