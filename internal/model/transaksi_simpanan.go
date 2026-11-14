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
}
