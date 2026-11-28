package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

// hariJatuhTempoWajib: simpanan wajib dibayar paling lambat tanggal 10 pada bulan periodenya.
const hariJatuhTempoWajib = 10

// =====================================================================
// L-4 Simpanan wajib belum dibayar
// =====================================================================

// WajibInput: Periode berformat YYYY-MM (kosong = bulan berjalan).
type WajibInput struct {
	Periode string
}

// LaporanWajib adalah daftar anggota yang belum membayar simpanan wajib suatu periode.
type LaporanWajib struct {
	Periode         time.Time // tanggal 1 bulan yang dilaporkan
	JatuhTempo      time.Time // tanggal 10 bulan itu
	HariTerlambat   int       // hari sejak jatuh tempo (0 bila belum lewat)
	Baris           []model.AnggotaWajibBelum
	PerAnggota      int64 // nominal simpanan wajib per bulan
	TotalKekurangan int64
}

// Nama menghasilkan teks seperti "Februari 2027".
func (l LaporanWajib) Nama() string {
	return fmt.Sprintf("%s %d", namaBulan[l.Periode.Month()-1], l.Periode.Year())
}

// Terlambat benar bila jatuh tempo periode ini sudah lewat.
func (l LaporanWajib) Terlambat() bool { return l.HariTerlambat > 0 }

// PeriodeInput adalah isi kolom bulan (YYYY-MM).
func (l LaporanWajib) PeriodeInput() string { return l.Periode.Format("2006-01") }

// HitungLaporanWajib menyusun laporan dari daftar anggota yang belum bayar.
// Fungsi murni: tanggal hari ini dan nominal diberikan dari luar.
func HitungLaporanWajib(periode time.Time, belum []model.AnggotaWajibBelum, hariIni time.Time, perAnggota int64) LaporanWajib {
	awal := time.Date(periode.Year(), periode.Month(), 1, 0, 0, 0, 0, time.UTC)
	jt := time.Date(awal.Year(), awal.Month(), hariJatuhTempoWajib, 0, 0, 0, 0, time.UTC)

	l := LaporanWajib{
		Periode:         awal,
		JatuhTempo:      jt,
		Baris:           belum,
		PerAnggota:      perAnggota,
		TotalKekurangan: int64(len(belum)) * perAnggota,
	}
	if hariIni.After(jt) {
		l.HariTerlambat = int(hariIni.Sub(jt) / (24 * time.Hour))
	}
	return l
}

// WajibBelumBayar menyusun L-4 untuk satu periode.
func (s *LaporanService) WajibBelumBayar(ctx context.Context, in WajibInput) (LaporanWajib, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return LaporanWajib{}, ErrBelumLogin
	}

	hari := s.hariIni()
	periode := time.Date(hari.Year(), hari.Month(), 1, 0, 0, 0, 0, time.UTC)
	if p := strings.TrimSpace(in.Periode); p != "" {
		t, err := time.Parse("2006-01", p)
		if err != nil {
			return LaporanWajib{}, &ValidasiError{Pesan: map[string]string{"periode": "Periode harus berformat tahun-bulan."}}
		}
		periode = t
	}

	belum, err := s.repo.WajibBelumPeriode(ctx, s.db, periode)
	if err != nil {
		return LaporanWajib{}, fmt.Errorf("simpanan wajib belum dibayar: %w", err)
	}
	return HitungLaporanWajib(periode, belum, hari, s.wajibPerBulan), nil
}

// CSV L-4: satu baris per anggota, ditutup baris total.
func (l LaporanWajib) CSV() ([]byte, error) {
	baris := [][]string{{"Nomor anggota", "Nama", "Periode", "Jatuh tempo", "Hari terlambat", "Kekurangan"}}
	for _, a := range l.Baris {
		baris = append(baris, []string{
			amanCSV(a.NomorAnggota), amanCSV(a.NamaAnggota), l.PeriodeInput(),
			tanggalCSV(l.JatuhTempo), fmt.Sprint(l.HariTerlambat), angka(l.PerAnggota),
		})
	}
	baris = append(baris, []string{"Total", fmt.Sprintf("%d anggota", len(l.Baris)), "", "", "", angka(l.TotalKekurangan)})
	return tulisCSV(baris)
}
