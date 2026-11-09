package model

import "time"

// membedakan pembayaran angsuran dengan pelunasan dipercepat
type JenisPembayaran string

const (
	PembayaranAngsuran  JenisPembayaran = "angsuran"
	PembayaranPelunasan JenisPembayaran = "pelunasan"
)

type Pembayaran struct {
	ID           int64
	IDPinjaman   int64
	Jenis        JenisPembayaran
	TanggalBayar time.Time
	Jumlah       int64
	BiayaAdmin   int64 // hanya untuk pelunasan; bukan bagian dari alokasi angsuran
}

type RincianPelunasan struct {
	SisaPokok         int64
	Denda             int64
	JasaJatuhTempo    int64 // jasa angsuran yang sudah jatuh tempo dan belum dibayar
	JasaBulanBerjalan int64 // jasa satu angsuran berikutnya yang belum jatuh tempo
	BiayaAdmin        int64
	Total             int64
}
