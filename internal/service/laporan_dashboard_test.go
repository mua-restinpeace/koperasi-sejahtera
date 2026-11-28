package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

func (f *fakeLaporanRepo) RingkasanAnggota(_ context.Context, _ database.DBTX, _, _ time.Time) (int, int, error) {
	return f.aktif, f.baru, f.galat
}
func (f *fakeLaporanRepo) TotalSimpanan(_ context.Context, _ database.DBTX) (int64, error) {
	return f.simpanan, f.galat
}
func (f *fakeLaporanRepo) WajibBelum(_ context.Context, _ database.DBTX, periode, _ time.Time, batas int) ([]model.AnggotaWajibBelum, int, error) {
	f.periodeWajib, f.batasWajib = periode, batas
	return f.wajib, f.totalWajib, f.galat
}

// Contoh C pada 2027-02-11: hanya pembayaran 15 Januari yang sudah terjadi.
func TestDashboard_ContohC(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	pb, al := alokasiLaporan(t, jadwal, bayar[:1])
	repo := &fakeLaporanRepo{
		aktif: 12, baru: 2, simpanan: 9_300_000,
		wajib: []model.AnggotaWajibBelum{{ID: 5, NomorAnggota: "KKS-0005", NamaAnggota: "Maya Putri"}}, totalWajib: 4,
		pinjaman: []model.PinjamanLaporan{pinjamanUji(1)}, angsuran: jadwal, bayar: bayar,
		pembayaran: pb, alokasi: al,
	}
	s := svcLaporan(repo, "2027-02-11").DenganWajib(100_000)

	d, err := s.Dashboard(ctxLogin())
	if err != nil {
		t.Fatal(err)
	}

	r := d.Ringkasan
	if r.AnggotaAktif != 12 || r.AnggotaBaru != 2 || r.TotalSimpanan != 9_300_000 || r.PinjamanAktif != 1 {
		t.Errorf("ringkasan = %+v", r)
	}
	if r.SisaPinjaman != 4_702_000 { // 6.000.000 - 1.000.000 - 298.000 (angka kartu pinjaman per 11 Feb)
		t.Errorf("sisa pinjaman = %d, want 4702000", r.SisaPinjaman)
	}
	if r.PendapatanJasa != 0 || r.PendapatanDenda != 0 { // pembayaran 15 Jan, bukan Februari
		t.Errorf("pendapatan bulan ini = %d/%d, want 0/0", r.PendapatanJasa, r.PendapatanDenda)
	}

	if d.TotalTelat != 2 || len(d.Terlambat) != 2 || d.Terlambat[0].HariTerlambat != 32 || d.Terlambat[0].DendaBerjalan != 54_000 {
		t.Errorf("terlambat = %d %+v", d.TotalTelat, d.Terlambat)
	}

	if d.TotalWajib != 4 || len(d.WajibBelum) != 1 || d.WajibPerBulan != 100_000 {
		t.Errorf("wajib = %d %+v %d", d.TotalWajib, d.WajibBelum, d.WajibPerBulan)
	}
	if repo.periodeWajib != tgl("2027-02-01") || repo.batasWajib != batasBarisDashboard {
		t.Errorf("periode wajib = %v batas = %d", repo.periodeWajib, repo.batasWajib)
	}

	if d.Nama() != "Februari 2027" {
		t.Errorf("nama = %q", d.Nama())
	}
	if len(d.Grafik) != 6 || d.Grafik[0].Bulan != tgl("2026-09-01") || d.Grafik[5].Bulan != tgl("2027-02-01") {
		t.Fatalf("grafik = %+v", d.Grafik)
	}
	if jan := d.Grafik[4]; jan.Jasa != 120_000 || jan.Denda != 82_000 {
		t.Errorf("Januari = %+v, want jasa 120000 denda 82000", jan)
	}
}

func TestDashboard_TerlambatDibatasi(t *testing.T) {
	var pj []model.PinjamanLaporan
	var jadwal []model.Angsuran
	for i := int64(1); i <= 7; i++ {
		p := pinjamanUji(i)
		pj = append(pj, p)
		j, _ := contohC(t, i)
		jadwal = append(jadwal, j...)
	}
	repo := &fakeLaporanRepo{pinjaman: pj, angsuran: jadwal}
	d, err := svcLaporan(repo, "2027-02-11").Dashboard(ctxLogin())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Terlambat) != batasBarisDashboard || d.TotalTelat <= batasBarisDashboard {
		t.Errorf("tampil %d dari %d, batas %d", len(d.Terlambat), d.TotalTelat, batasBarisDashboard)
	}
}

func TestDashboard_Kosong(t *testing.T) {
	d, err := svcLaporan(&fakeLaporanRepo{}, "2027-02-11").Dashboard(ctxLogin())
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Grafik) != 6 || d.Ringkasan.SisaPinjaman != 0 || len(d.Terlambat) != 0 {
		t.Errorf("dashboard kosong = %+v", d)
	}
}

func TestDashboard_BelumLoginDanGalat(t *testing.T) {
	s := svcLaporan(&fakeLaporanRepo{}, "2027-02-11")
	if _, err := s.Dashboard(context.Background()); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("tanpa login: %v", err)
	}
	s = svcLaporan(&fakeLaporanRepo{galat: errors.New("db mati")}, "2027-02-11")
	if _, err := s.Dashboard(ctxLogin()); err == nil {
		t.Error("galat repo harus diteruskan")
	}
}

func TestIsiBulanGrafik(t *testing.T) {
	data := []PendapatanBulan{{Bulan: tgl("2026-12-01"), Jasa: 5}}
	g := isiBulanGrafik(data, tgl("2027-02-01"), 4) // lintas tahun
	want := []string{"2026-11-01", "2026-12-01", "2027-01-01", "2027-02-01"}
	for i, w := range want {
		if g[i].Bulan != tgl(w) {
			t.Errorf("bulan[%d] = %v, want %s", i, g[i].Bulan, w)
		}
	}
	if g[1].Jasa != 5 || g[0].Jasa != 0 {
		t.Errorf("isi = %+v", g)
	}
}
