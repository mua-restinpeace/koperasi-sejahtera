package service

import (
	"context"
	"fmt"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

const (
	batasBarisDashboard = 5 // baris yang ditampilkan per tabel di dashboard
	bulanGrafik         = 6 // jumlah bulan pada grafik pendapatan
)

// DenganWajib mengisi nominal simpanan wajib per bulan (kolom "kekurangan" di dashboard).
func (s *LaporanService) DenganWajib(jumlah int64) *LaporanService {
	s.wajibPerBulan = jumlah
	return s
}

// RingkasanDashboard adalah angka-angka di baris atas dashboard.
type RingkasanDashboard struct {
	AnggotaAktif    int
	AnggotaBaru     int // bergabung pada bulan berjalan
	PinjamanAktif   int
	SisaPinjaman    int64 // sisa pokok seluruh pinjaman berjalan
	TotalSimpanan   int64
	PendapatanJasa  int64 // bulan berjalan
	PendapatanDenda int64 // bulan berjalan
}

// Dashboard adalah seluruh isi halaman beranda, dihitung pada satu tanggal.
type Dashboard struct {
	Tanggal       time.Time
	Bulan         time.Time // tanggal 1 bulan berjalan
	Ringkasan     RingkasanDashboard
	Terlambat     []BarisTunggakan // paling lama menunggak, dibatasi
	TotalTelat    int              // seluruh angsuran terlambat
	WajibBelum    []model.AnggotaWajibBelum
	TotalWajib    int
	WajibPerBulan int64
	Grafik        []PendapatanBulan // enam bulan terakhir, bulan kosong berisi nol
}

// Nama menghasilkan teks seperti "November 2026" untuk bulan berjalan.
func (d Dashboard) Nama() string {
	return fmt.Sprintf("%s %d", namaBulan[d.Bulan.Month()-1], d.Bulan.Year())
}

// SisaPokokPinjaman menjumlahkan sisa pokok semua pinjaman pada tanggal asOf memakai
// helper.HitungStatus (sama dengan kartu pinjaman dan daftar tunggakan). Pinjaman yang
// datanya tidak konsisten dilewati dan tidak membuat dashboard gagal.
func SisaPokokPinjaman(pinjaman []model.PinjamanLaporan, angsuran []model.Angsuran, bayar []model.Pembayaran, asOf time.Time, biayaAdminBps int) int64 {
	jadwal := map[int64][]model.Angsuran{}
	for _, a := range angsuran {
		jadwal[a.IDPinjaman] = append(jadwal[a.IDPinjaman], a)
	}
	pembayaran := map[int64][]model.Pembayaran{}
	for _, p := range bayar {
		pembayaran[p.IDPinjaman] = append(pembayaran[p.IDPinjaman], p)
	}

	var sisa int64
	for _, pj := range pinjaman {
		status, err := helper.HitungStatus(jadwal[pj.ID], pembayaran[pj.ID], asOf, helper.AturanHitung{DendaPerHari: pj.DendaPerHari, BiayaAdminBps: biayaAdminBps})
		if err != nil {
			continue
		}
		for _, st := range status {
			sisa += st.SisaPokok()
		}
	}
	return sisa
}

// isiBulanGrafik menghasilkan tepat n bulan berurutan yang berakhir di akhir;
// bulan tanpa pendapatan diisi nol supaya grafik tidak melompat.
func isiBulanGrafik(data []PendapatanBulan, akhir time.Time, n int) []PendapatanBulan {
	ada := map[time.Time]PendapatanBulan{}
	for _, p := range data {
		ada[p.Bulan] = p
	}

	hasil := make([]PendapatanBulan, 0, n)
	for i := n - 1; i >= 0; i-- {
		b := time.Date(akhir.Year(), akhir.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		p, ok := ada[b]
		if !ok {
			p = PendapatanBulan{Bulan: b}
		}
		hasil = append(hasil, p)
	}
	return hasil
}

// Dashboard menyusun ringkasan keuangan koperasi pada hari ini.
func (s *LaporanService) Dashboard(ctx context.Context) (Dashboard, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return Dashboard{}, ErrBelumLogin
	}

	hari := s.hariIni()
	awalBulan := time.Date(hari.Year(), hari.Month(), 1, 0, 0, 0, 0, time.UTC)
	awalGrafik := awalBulan.AddDate(0, -(bulanGrafik - 1), 0)
	akhirBulan := time.Date(hari.Year(), hari.Month()+1, 0, 0, 0, 0, 0, time.UTC)

	d := Dashboard{Tanggal: hari, Bulan: awalBulan, WajibPerBulan: s.wajibPerBulan}

	galat := func(err error) (Dashboard, error) { return Dashboard{}, fmt.Errorf("dashboard: %w", err) }

	var err error
	d.Ringkasan.AnggotaAktif, d.Ringkasan.AnggotaBaru, err = s.repo.RingkasanAnggota(ctx, s.db, awalBulan, hari)
	if err != nil {
		return galat(err)
	}

	if d.Ringkasan.TotalSimpanan, err = s.repo.TotalSimpanan(ctx, s.db); err != nil {
		return galat(err)
	}

	if d.WajibBelum, d.TotalWajib, err = s.repo.WajibBelum(ctx, s.db, awalBulan, hari, batasBarisDashboard); err != nil {
		return galat(err)
	}

	pinjaman, err := s.repo.PinjamanBerjalan(ctx, s.db, hari)
	if err != nil {
		return galat(err)
	}
	angsuran, err := s.repo.AngsuranBerjalan(ctx, s.db, hari)
	if err != nil {
		return galat(err)
	}
	bayar, err := s.repo.PembayaranBerjalan(ctx, s.db, hari)
	if err != nil {
		return galat(err)
	}

	d.Ringkasan.PinjamanAktif = len(pinjaman)
	d.Ringkasan.SisaPinjaman = SisaPokokPinjaman(pinjaman, angsuran, bayar, hari, s.biayaAdminBps)

	telat, _ := HitungTunggakan(pinjaman, angsuran, bayar, hari, s.biayaAdminBps)
	d.TotalTelat = len(telat)
	d.Terlambat = telat
	if len(d.Terlambat) > batasBarisDashboard {
		d.Terlambat = d.Terlambat[:batasBarisDashboard]
	}

	pb, err := s.repo.Pembayaran(ctx, s.db, &awalGrafik, &akhirBulan)
	if err != nil {
		return galat(err)
	}
	al, err := s.repo.Alokasi(ctx, s.db, &awalGrafik, &akhirBulan)
	if err != nil {
		return galat(err)
	}
	d.Grafik = isiBulanGrafik(HitungPendapatan(pb, al), awalBulan, bulanGrafik)

	bulanIni := d.Grafik[len(d.Grafik)-1]
	d.Ringkasan.PendapatanJasa = bulanIni.Jasa
	d.Ringkasan.PendapatanDenda = bulanIni.Denda

	return d, nil
}
