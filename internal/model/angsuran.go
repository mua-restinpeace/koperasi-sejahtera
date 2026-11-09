package model

import "time"

type Angsuran struct {
	ID int64
	IDPinjaman int64
	Urutan     int
	JatuhTempo time.Time
	Pokok      int64
	Jasa       int64
}

type StatusAngsuran struct {
	Angsuran
	DendaDibayar  int64
	JasaDibayar   int64
	PokokDibayar  int64
	JasaDihapus   int64     // jasa bulan-bulan berikutnya yang tidak ditagihkan lagi karena pelunasan dipercepat
	LunasPada     time.Time // tanggal jasa dan pokok angsuran lunas; nol bila belum lunas
	DendaTerutang int64
}

// Helper
// total adalah jumlah angsuran
func (a Angsuran) Total() int64 { return a.Pokok + a.Jasa }

// pokok yang belum dibayar
func (s StatusAngsuran) SisaPokok() int64 { return s.Pokok - s.PokokDibayar }

// jasa yang masih ditagihkan
func (s StatusAngsuran) SisaJasa() int64 { return s.Jasa - s.JasaDibayar - s.JasaDihapus}

func (s StatusAngsuran) IsLunas() bool { return s.SisaPokok() == 0 && s.SisaJasa() == 0 }
