package handler

import (
	"fmt"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

// Model tampilan Dashboard: mengubah service.Dashboard menjadi isian template.
// Semua uang int64 rupiah, tanggal sudah berupa teks.

type dataDashboard struct {
	Halaman
	Ringkasan  ringkasanDashboard
	Terlambat  []barisTerlambat
	TotalTelat int
	WajibBelum []barisWajibBelum
	TotalWajib int
	Periode    string // "November 2026"
	Grafik     []titikGrafik
}

type ringkasanDashboard struct {
	AnggotaAktif    int
	AnggotaBaru     int // bulan ini
	PinjamanAktif   int
	SisaPinjaman    int64
	TotalSimpanan   int64
	PendapatanJasa  int64
	PendapatanDenda int64
}

type barisTerlambat struct {
	IDAnggota  int64
	Nama       string
	NoAnggota  string
	IDPinjaman int64
	NoPinjaman string
	JatuhTempo string
	Tagihan    int64
	HariTelat  int
}

type barisWajibBelum struct {
	IDAnggota  int64
	Nama       string
	NoAnggota  string
	Kekurangan int64
	Periode    string
}

// titikGrafik: tinggi batang dalam persen (0-100), dihitung di server.
type titikGrafik struct {
	Bulan       string
	Jasa, Denda int64
	JasaPct     int
	DendaPct    int
}

var singkatanBulan = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

// tanggalPendek menulis tanggal seperti "10 Okt 2026".
func tanggalPendek(t time.Time) string {
	return fmt.Sprintf("%d %s %d", t.Day(), singkatanBulan[t.Month()-1], t.Year())
}

func tampilanDashboard(s service.Dashboard) dataDashboard {
	d := dataDashboard{
		Periode:    s.Nama(),
		TotalTelat: s.TotalTelat,
		TotalWajib: s.TotalWajib,
		Ringkasan: ringkasanDashboard{
			AnggotaAktif:    s.Ringkasan.AnggotaAktif,
			AnggotaBaru:     s.Ringkasan.AnggotaBaru,
			PinjamanAktif:   s.Ringkasan.PinjamanAktif,
			SisaPinjaman:    s.Ringkasan.SisaPinjaman,
			TotalSimpanan:   s.Ringkasan.TotalSimpanan,
			PendapatanJasa:  s.Ringkasan.PendapatanJasa,
			PendapatanDenda: s.Ringkasan.PendapatanDenda,
		},
	}

	for _, b := range s.Terlambat {
		d.Terlambat = append(d.Terlambat, barisTerlambat{
			IDAnggota:  b.IDAnggota,
			Nama:       b.NamaAnggota,
			NoAnggota:  b.NomorAnggota,
			IDPinjaman: b.IDPinjaman,
			NoPinjaman: fmt.Sprintf("Pinjaman #%d · ke-%d", b.IDPinjaman, b.Urutan),
			JatuhTempo: tanggalPendek(b.JatuhTempo),
			Tagihan:    b.Total(),
			HariTelat:  b.HariTerlambat,
		})
	}

	periode := fmt.Sprintf("%s %d", singkatanBulan[s.Bulan.Month()-1], s.Bulan.Year())
	for _, a := range s.WajibBelum {
		d.WajibBelum = append(d.WajibBelum, barisWajibBelum{
			IDAnggota: a.ID, Nama: a.NamaAnggota, NoAnggota: a.NomorAnggota,
			Kekurangan: s.WajibPerBulan, Periode: periode,
		})
	}

	var maks int64 = 1
	for _, p := range s.Grafik {
		maks = max(maks, p.Jasa, p.Denda)
	}
	for _, p := range s.Grafik {
		d.Grafik = append(d.Grafik, titikGrafik{
			Bulan: singkatanBulan[p.Bulan.Month()-1], Jasa: p.Jasa, Denda: p.Denda,
			JasaPct: int(p.Jasa * 100 / maks), DendaPct: int(p.Denda * 100 / maks),
		})
	}
	return d
}
