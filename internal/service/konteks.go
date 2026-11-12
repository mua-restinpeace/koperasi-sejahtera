// Package service berisi aturan bisnis aplikasi. Service memanggil repository
// dan package helper; tidak mengenal HTTP.
package service

import (
	"context"
	"errors"
	"sort"
	"strings"
)

// Identitas adalah pengurus yang sedang login, dibawa lewat context.
type Identitas struct {
	IDPengguna  int64
	Username    string
	NamaLengkap string
	CSRFToken   string
}

type kunciIdentitas struct{}

// DenganIdentitas mengembalikan context yang membawa identitas pengurus.
func DenganIdentitas(ctx context.Context, id Identitas) context.Context {
	return context.WithValue(ctx, kunciIdentitas{}, id)
}

// IdentitasDari mengambil identitas dari context; ok=false bila belum login.
func IdentitasDari(ctx context.Context) (Identitas, bool) {
	id, ok := ctx.Value(kunciIdentitas{}).(Identitas)
	return id, ok
}

// Kesalahan umum yang dikenali handler.
var (
	ErrKredensialSalah      = errors.New("username atau kata sandi salah")
	ErrSesiTidakValid       = errors.New("sesi tidak valid atau sudah berakhir")
	ErrBelumLogin           = errors.New("belum login")
	ErrNomorAnggotaSudahAda = errors.New("nomor anggota sudah dipakai")
)

// ValidasiError berisi pesan kesalahan per kolom isian, untuk ditampilkan di form.
type ValidasiError struct {
	Pesan map[string]string
}

func (e *ValidasiError) Error() string {
	kunci := make([]string, 0, len(e.Pesan))
	for k := range e.Pesan {
		kunci = append(kunci, k)
	}
	sort.Strings(kunci)
	bagian := make([]string, 0, len(kunci))
	for _, k := range kunci {
		bagian = append(bagian, k+": "+e.Pesan[k])
	}
	return "validasi gagal: " + strings.Join(bagian, "; ")
}
