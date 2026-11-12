package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

func siapAuth(t *testing.T) (*AuthService, *fakeHasher, *fakeSesi, *time.Time) {
	t.Helper()
	sekarang := time.Date(2026, 11, 10, 3, 0, 0, 0, time.UTC)
	h := &fakeHasher{}
	pg := &fakePengguna{data: map[string]model.Pengguna{
		"ani":  {ID: 1, Username: "ani", PasswordHash: "hash:rahasia-panjang", NamaLengkap: "Ani", Aktif: true},
		"budi": {ID: 2, Username: "budi", PasswordHash: "hash:rahasia-panjang", NamaLengkap: "Budi", Aktif: false},
	}}
	ss := &fakeSesi{user: map[int64]model.Pengguna{1: pg.data["ani"], 2: pg.data["budi"]}}
	s := NewAuthService(&fakeDB{}, pg, ss, h, 12*time.Hour, func() time.Time { return sekarang }, logDiam())
	return s, h, ss, &sekarang
}

func TestAuth_LoginBerhasilLaluAuthenticate(t *testing.T) {
	s, _, ss, _ := siapAuth(t)
	ctx := context.Background()

	token, sesi, err := s.Login(ctx, " ani ", "rahasia-panjang")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if token == "" || sesi.TokenHash == token {
		t.Errorf("token harus ada dan yang tersimpan harus hash-nya")
	}
	if _, ada := ss.data[hashToken(token)]; !ada {
		t.Error("sesi tidak tersimpan dengan hash token")
	}
	if got := sesi.KedaluwarsaPada.Sub(sesi.DibuatPada); got != 12*time.Hour {
		t.Errorf("durasi sesi = %v", got)
	}

	id, err := s.Authenticate(ctx, token)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	if id.IDPengguna != 1 || id.CSRFToken == "" {
		t.Errorf("identitas = %+v", id)
	}
}

func TestAuth_LoginGagal(t *testing.T) {
	tests := []struct {
		nama, user, pass string
		hashDipanggil    int
	}{
		{"password salah", "ani", "salah", 1},
		{"username tidak ada", "zzz", "apa-saja", 1}, // tetap menghitung hash palsu
		{"akun nonaktif", "budi", "rahasia-panjang", 1},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			s, h, ss, _ := siapAuth(t)
			_, _, err := s.Login(context.Background(), tt.user, tt.pass)
			if !errors.Is(err, ErrKredensialSalah) {
				t.Fatalf("err = %v, want ErrKredensialSalah", err)
			}
			if len(ss.data) != 0 {
				t.Error("sesi tidak boleh dibuat")
			}
			if h.dipanggil != tt.hashDipanggil {
				t.Errorf("Cocok dipanggil %d kali, want %d", h.dipanggil, tt.hashDipanggil)
			}
		})
	}
}

func TestAuth_SesiKedaluwarsaDanLogout(t *testing.T) {
	s, _, _, sekarang := siapAuth(t)
	ctx := context.Background()
	token, _, err := s.Login(ctx, "ani", "rahasia-panjang")
	if err != nil {
		t.Fatal(err)
	}

	*sekarang = sekarang.Add(12*time.Hour + time.Second)
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrSesiTidakValid) {
		t.Errorf("sesi kedaluwarsa: err = %v", err)
	}

	*sekarang = sekarang.Add(-time.Hour)
	if _, err := s.Authenticate(ctx, token); err != nil {
		t.Fatalf("sesi masih berlaku: %v", err)
	}
	if err := s.Logout(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrSesiTidakValid) {
		t.Errorf("setelah logout: err = %v", err)
	}
	if _, err := s.Authenticate(ctx, ""); !errors.Is(err, ErrSesiTidakValid) {
		t.Errorf("token kosong: err = %v", err)
	}
	if err := s.Logout(ctx, ""); err != nil {
		t.Errorf("logout tanpa token: %v", err)
	}
}
