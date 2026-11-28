package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func ctxPembayaranTest() context.Context {
	return DenganIdentitas(context.Background(), Identitas{
		IDPengguna: 7,
		Username:   "uji",
	})
}

func TestPembayaran_CatatPembayaranValidasi(t *testing.T) {
	tests := []struct {
		nama  string
		id    int64
		input CatatPembayaranInput
		kolom string
	}{
		{
			nama:  "pinjaman tidak valid",
			id:    0,
			input: CatatPembayaranInput{TanggalBayar: time.Now(), Jumlah: 1000},
			kolom: "pinjaman",
		},
		{
			nama:  "jumlah nol",
			id:    1,
			input: CatatPembayaranInput{TanggalBayar: time.Now(), Jumlah: 0},
			kolom: "jumlah",
		},
		{
			nama:  "jumlah negatif",
			id:    1,
			input: CatatPembayaranInput{TanggalBayar: time.Now(), Jumlah: -1},
			kolom: "jumlah",
		},
		{
			nama:  "jumlah melebihi batas",
			id:    1,
			input: CatatPembayaranInput{TanggalBayar: time.Now(), Jumlah: maksPembayaran + 1},
			kolom: "jumlah",
		},
		{
			nama:  "tanggal kosong",
			id:    1,
			input: CatatPembayaranInput{Jumlah: 1000},
			kolom: "tanggal_bayar",
		},
		{
			nama: "keterangan melebihi 255 karakter",
			id:   1,
			input: CatatPembayaranInput{
				TanggalBayar: time.Now(),
				Jumlah:       1000,
				Keterangan:   strings.Repeat("a", 256),
			},
			kolom: "keterangan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			svc := &PembayaranService{
				zona: time.Local,
			}

			_, err := svc.CatatPembayaran(ctxPembayaranTest(), tt.id, tt.input)

			var ve *ValidasiError
			if !errors.As(err, &ve) || ve.Pesan[tt.kolom] == "" {
				t.Fatalf("error = %v, ingin ValidasiError kolom %q", err, tt.kolom)
			}
		})
	}
}

func TestPembayaran_BatalkanPembayaranValidasi(t *testing.T) {
	tests := []struct {
		nama   string
		id     int64
		alasan string
		kolom  string
	}{
		{
			nama:   "alasan kosong",
			id:     1,
			alasan: "",
			kolom:  "alasan",
		},
		{
			nama:   "alasan hanya spasi",
			id:     1,
			alasan: "   ",
			kolom:  "alasan",
		},
		{
			nama:   "alasan melebihi 255 karakter",
			id:     1,
			alasan: strings.Repeat("a", 256),
			kolom:  "alasan",
		},
		{
			nama:   "id pembayaran nol",
			id:     0,
			alasan: "Kesalahan input",
			kolom:  "pembayaran",
		},
		{
			nama:   "id pembayaran negatif",
			id:     -1,
			alasan: "Kesalahan input",
			kolom:  "pembayaran",
		},
	}

	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			svc := &PembayaranService{
				zona: time.Local,
			}

			_, err := svc.BatalkanPembayaran(
				ctxPembayaranTest(),
				tt.id,
				tt.alasan,
			)

			var ve *ValidasiError
			if !errors.As(err, &ve) || ve.Pesan[tt.kolom] == "" {
				t.Fatalf("error = %v, ingin ValidasiError kolom %q", err, tt.kolom)
			}
		})
	}
}
