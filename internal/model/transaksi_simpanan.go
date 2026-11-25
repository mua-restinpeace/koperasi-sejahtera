package model

import "time"

// kategoru dan tipe mutasi simpanan
const (
	SimpananPokok    = "pokok"
	SimpananWajib    = "wajib"
	SimpananSukarela = "sukarela"

	MutasiSetor = "setor"
	MutasiTarik = "tarik"
)

type TransaksiSimpanan struct {
	ID               int64
	IDAnggota        int64
	Kategori         string
	TipeMutasi       string
	Jumlah           int64
	Periode          *time.Time // hanya untuk setoran wajib;
	TanggalTransaksi time.Time
	Keterangan       string
	DibuatOleh       int64

	DibatalkanPada   *time.Time
	AlasanPembatalan string
}

// SaldoSimpanan adalah saldo simpanan satu anggota per kategori
// setoran dikurangi penarikan, tidak termasuk transaksi yang sudah dibatalkan
type SaldoSimpanan struct {
	Pokok    int64
	Wajib    int64
	Sukarela int64
}

// Total adalah jumlah saldo ketiga kategori
func (s SaldoSimpanan) Total() int64 {
	return s.Pokok + s.Wajib + s.Sukarela
}
