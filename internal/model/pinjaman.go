package model

import "time"

const (
	StatusPinjamanAktif      = "aktif"
	StatusPinjamanLunas      = "lunas"
	StatusPinjamanDibatalkan = "dibatalkan"
)

type Pinjaman struct {
	ID               int64
	IDAnggota        int64
	JumlahPinjaman   int64
	TenorBulan       int
	JasaPerBulanBps  int
	DendaPerHari     int64
	TanggalPencairan time.Time
	Status           string
	TanggalLunas     *time.Time
	DibuatOleh       int64
	DibuatPada       time.Time
}

type PinjamanListItem struct {
	Pinjaman
	NomorAnggota string
	NamaAnggota  string
	TotalPokok   int64
	TotalJasa    int64
}

type DetailPinjaman struct {
	Pinjaman
	NomorAnggota    string
	NamaAnggota     string
	Jadwal          []StatusAngsuran
	RincianLunas    *RincianPelunasan
	TotalSudahBayar int64
	SisaKewajiban   int64
}
