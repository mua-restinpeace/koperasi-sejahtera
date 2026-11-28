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

// PinjamanKartu adalah data induk satu pinjaman beserta anggotanya, dipakai
// kartu pinjaman (L-1) dan daftar pinjaman.
type PinjamanKartu struct {
	ID               int64
	IDAnggota        int64
	NomorAnggota     string
	NamaAnggota      string
	Jumlah           int64
	TenorBulan       int
	JasaBps          int
	DendaPerHari     int64
	TanggalPencairan time.Time
	Status           string    // aktif, lunas
	TanggalLunas     time.Time // nol bila belum lunas
}

// AnggotaWajibBelum adalah anggota aktif yang belum membayar simpanan wajib
// pada suatu periode, dipakai dashboard.
type AnggotaWajibBelum struct {
	ID           int64
	NomorAnggota string
	NamaAnggota  string
}
