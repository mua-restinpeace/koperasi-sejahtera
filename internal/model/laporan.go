package model

import "time"

// PembayaranLaporan adalah satu pembayaran pinjaman yang belum dibatalkan,
// dipakai laporan pendapatan untuk biaya administrasi.
type PembayaranLaporan struct {
	ID           int64
	TanggalBayar time.Time
	BiayaAdmin   int64
}

// AlokasiLaporan adalah pembagian satu pembayaran ke satu angsuran, lengkap dengan
// tanggal bayarnya (basis kas), dipakai laporan pendapatan untuk jasa dan denda.
type AlokasiLaporan struct {
	IDPembayaran int64
	TanggalBayar time.Time
	Denda        int64
	Jasa         int64
	Pokok        int64
}

// PinjamanLaporan adalah pinjaman beserta identitas anggotanya, dipakai laporan
// tunggakan.
type PinjamanLaporan struct {
	ID           int64
	IDAnggota    int64
	NomorAnggota string
	NamaAnggota  string
	DendaPerHari int64
}
