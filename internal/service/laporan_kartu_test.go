package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

func (f *fakeLaporanRepo) DaftarPinjaman(_ context.Context, _ database.DBTX) ([]model.PinjamanKartu, error) {
	return f.daftar, f.galat
}
func (f *fakeLaporanRepo) DetailPinjaman(_ context.Context, _ database.DBTX, _ int64) (model.PinjamanKartu, error) {
	if f.galat != nil {
		return model.PinjamanKartu{}, f.galat
	}
	if f.kartu == nil {
		return model.PinjamanKartu{}, repository.ErrTidakDitemukan
	}
	return *f.kartu, nil
}
func (f *fakeLaporanRepo) AngsuranPinjaman(_ context.Context, _ database.DBTX, _ int64) ([]model.Angsuran, error) {
	return f.angsuran, f.galat
}
func (f *fakeLaporanRepo) PembayaranPinjaman(_ context.Context, _ database.DBTX, _ int64) ([]model.Pembayaran, error) {
	return f.bayar, f.galat
}

func pinjamanContohC() model.PinjamanKartu {
	return model.PinjamanKartu{
		ID: 1, IDAnggota: 2, NomorAnggota: "KKS-0002", NamaAnggota: "Rudi Hartono",
		Jumlah: 6_000_000, TenorBulan: 6, JasaBps: 100, DendaPerHari: 2000,
		TanggalPencairan: tgl("2026-11-05"), Status: "lunas", TanggalLunas: tgl("2027-02-20"),
	}
}

func kartuContohC(t *testing.T, asOf string) KartuPinjaman {
	t.Helper()
	jadwal, bayar := contohC(t, 1)
	k, err := HitungKartu(pinjamanContohC(), jadwal, bayar, tgl(asOf), 200)
	if err != nil {
		t.Fatalf("HitungKartu: %v", err)
	}
	return k
}

func TestHitungKartu_SebelumAdaPembayaran(t *testing.T) {
	k := kartuContohC(t, "2026-11-28")
	if len(k.Baris) != 6 || len(k.Pembayaran) != 0 {
		t.Fatalf("baris=%d pembayaran=%d", len(k.Baris), len(k.Pembayaran))
	}
	for _, b := range k.Baris {
		if b.Status != StatusKartuBelum {
			t.Errorf("angsuran %d status %q", b.Urutan, b.Status)
		}
	}
	if k.SisaPokok != 6_000_000 || k.SisaJasa != 360_000 || k.DendaTerutang != 0 || k.TotalDibayar != 0 {
		t.Errorf("sisa pokok=%d jasa=%d denda=%d dibayar=%d", k.SisaPokok, k.SisaJasa, k.DendaTerutang, k.TotalDibayar)
	}
}

// Posisi 2027-02-11 sama dengan angka daftar tunggakan (L-2) untuk Contoh C:
// angsuran ke-2 terlambat 32 hari denda 54.000, ke-3 terlambat 1 hari denda 2.000.
func TestHitungKartu_ContohC_11Februari(t *testing.T) {
	k := kartuContohC(t, "2027-02-11")

	// pembayaran 12 Februari belum terjadi pada 11 Februari, jadi tidak ikut
	if len(k.Pembayaran) != 1 {
		t.Fatalf("pembayaran sampai tanggal itu = %d, want 1", len(k.Pembayaran))
	}
	if b := k.Baris[0]; b.Status != StatusKartuLunas {
		t.Errorf("ke-1 status %q", b.Status)
	}
	b2 := k.Baris[1]
	if b2.Status != StatusKartuMenunggak || b2.HariTerlambat != 32 || b2.SisaPokok != 702_000 || b2.SisaJasa != 0 || b2.DendaTerutang != 54_000 {
		t.Errorf("ke-2 = %+v", b2)
	}
	b3 := k.Baris[2]
	if b3.Status != StatusKartuMenunggak || b3.HariTerlambat != 1 || b3.SisaPokok != 1_000_000 || b3.SisaJasa != 60_000 || b3.DendaTerutang != 2_000 {
		t.Errorf("ke-3 = %+v", b3)
	}
	if k.Baris[3].Status != StatusKartuBelum {
		t.Errorf("ke-4 status %q", k.Baris[3].Status)
	}
	if k.TotalDibayar != 1_500_000 {
		t.Errorf("total dibayar = %d, want 1500000", k.TotalDibayar)
	}
	if want := k.SisaPokok + k.SisaJasa + k.DendaTerutang; k.SisaTagihan() != want {
		t.Errorf("SisaTagihan = %d, want %d", k.SisaTagihan(), want)
	}
}

func TestHitungKartu_ContohC_SetelahPelunasan(t *testing.T) {
	k := kartuContohC(t, "2027-02-20")
	for _, b := range k.Baris {
		if b.Status != StatusKartuLunas {
			t.Errorf("angsuran %d status %q, want lunas", b.Urutan, b.Status)
		}
	}
	if k.SisaPokok != 0 || k.SisaJasa != 0 || k.DendaTerutang != 0 || k.SisaTagihan() != 0 {
		t.Errorf("sisa pokok=%d jasa=%d denda=%d", k.SisaPokok, k.SisaJasa, k.DendaTerutang)
	}
	if len(k.Pembayaran) != 3 {
		t.Errorf("pembayaran = %d, want 3", len(k.Pembayaran))
	}
}

func TestHitungKartu_DataTidakKonsistenGalat(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	bayar[0].Jumlah = 99_000_000 // melebihi seluruh tagihan
	if _, err := HitungKartu(pinjamanContohC(), jadwal, bayar, tgl("2027-03-01"), 200); err == nil {
		t.Error("harus galat bila pembayaran tidak bisa dialokasikan")
	}
}

func TestKartuPinjaman_Service(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	pj := pinjamanContohC()
	repo := &fakeLaporanRepo{kartu: &pj, angsuran: jadwal, bayar: bayar}
	s := svcLaporan(repo, "2027-02-11")

	if _, err := s.KartuPinjaman(context.Background(), KartuInput{ID: 1}); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("tanpa login: %v", err)
	}

	ctx := ctxLogin()
	k, err := s.KartuPinjaman(ctx, KartuInput{ID: 1}) // tanpa tanggal = hari ini (11 Feb 2027)
	if err != nil {
		t.Fatal(err)
	}
	if k.Tanggal != tgl("2027-02-11") || k.Baris[1].DendaTerutang != 54_000 {
		t.Errorf("tanggal=%v denda ke-2=%d", k.Tanggal, k.Baris[1].DendaTerutang)
	}

	k, err = s.KartuPinjaman(ctx, KartuInput{ID: 1, Tanggal: "2027-02-20"})
	if err != nil || k.SisaTagihan() != 0 {
		t.Errorf("per 2027-02-20: err=%v sisa=%d", err, k.SisaTagihan())
	}

	var ve *ValidasiError
	if _, err := s.KartuPinjaman(ctx, KartuInput{ID: 1, Tanggal: "kemarin"}); !errors.As(err, &ve) || ve.Pesan["tanggal"] == "" {
		t.Errorf("tanggal salah: %v", err)
	}

	repo.kartu = nil
	if _, err := s.KartuPinjaman(ctx, KartuInput{ID: 99}); !errors.Is(err, ErrPinjamanTidakDitemukan) {
		t.Errorf("pinjaman tidak ada: %v", err)
	}

	repo.kartu, repo.galat = &pj, errors.New("db mati")
	if _, err := s.KartuPinjaman(ctx, KartuInput{ID: 1}); err == nil || errors.Is(err, ErrPinjamanTidakDitemukan) {
		t.Errorf("galat repo harus diteruskan: %v", err)
	}
}

func TestDaftarPinjaman_Service(t *testing.T) {
	repo := &fakeLaporanRepo{daftar: []model.PinjamanKartu{pinjamanContohC()}}
	s := svcLaporan(repo, "2027-02-11")
	if _, err := s.DaftarPinjaman(context.Background()); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("tanpa login: %v", err)
	}
	d, err := s.DaftarPinjaman(ctxLogin())
	if err != nil || len(d) != 1 {
		t.Errorf("daftar=%d err=%v", len(d), err)
	}
}

// ---------------------------------------------------------------------
// CSV
// ---------------------------------------------------------------------

func barisCSV(t *testing.T, b []byte, err error) []string {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(b, []byte("\xEF\xBB\xBF")) {
		t.Error("CSV harus diawali BOM UTF-8")
	}
	return strings.Split(strings.TrimRight(string(bytes.TrimPrefix(b, []byte("\xEF\xBB\xBF"))), "\n"), "\n")
}

func TestCSV_Pendapatan(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	pb, al := alokasiLaporan(t, jadwal, bayar)
	bulan := HitungPendapatan(pb, al)
	b, err := LaporanPendapatan{Bulan: bulan, Total: JumlahPendapatan(bulan)}.CSV()
	baris := barisCSV(t, b, err)

	want := []string{
		"Bulan;Jasa;Denda;Biaya admin;Total",
		"Januari 2027;120000;82000;0;202000",
		"Februari 2027;120000;76000;80000;276000",
		"Total;240000;158000;80000;478000",
	}
	if strings.Join(baris, "|") != strings.Join(want, "|") {
		t.Errorf("CSV =\n%s\nwant\n%s", strings.Join(baris, "\n"), strings.Join(want, "\n"))
	}
}

func TestCSV_Tunggakan(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	pj := []model.PinjamanLaporan{{ID: 1, IDAnggota: 2, NomorAnggota: "KKS-0002", NamaAnggota: "=HYPERLINK(\"x\")", DendaPerHari: 2000}}
	baris, _ := HitungTunggakan(pj, jadwal, bayar, tgl("2027-02-11"), 200)
	lt := LaporanTunggakan{Tanggal: tgl("2027-02-11"), Baris: baris}
	for _, b := range baris {
		lt.TotalTagihan += b.Tagihan()
		lt.TotalDenda += b.DendaBerjalan
	}

	b, err := lt.CSV()
	out := barisCSV(t, b, err)
	if len(out) != 4 {
		t.Fatalf("baris CSV = %d, want 4: %v", len(out), out)
	}
	if !strings.Contains(out[1], ";32;") || !strings.Contains(out[1], ";54000;") {
		t.Errorf("baris ke-2: %q", out[1])
	}
	if strings.Contains(out[1], ";=HYPERLINK") || !strings.Contains(out[1], "'=HYPERLINK") {
		t.Errorf("nama berawalan = harus dinetralkan: %q", out[1])
	}
	if !strings.HasPrefix(out[3], "Total;") || !strings.Contains(out[3], ";56000;") {
		t.Errorf("baris total: %q", out[3])
	}
}

func TestCSV_Kartu(t *testing.T) {
	k := kartuContohC(t, "2027-02-20")
	b, err := k.CSV()
	out := barisCSV(t, b, err)
	teks := strings.Join(out, "\n")
	for _, w := range []string{"Kartu pinjaman;#1", "Anggota;KKS-0002;Rudi Hartono", "Jumlah pinjaman;6000000", "Total sisa tagihan;0", "2027-02-20;pelunasan;4178000;80000", "Lunas"} {
		if !strings.Contains(teks, w) {
			t.Errorf("CSV tidak memuat %q:\n%s", w, teks)
		}
	}
	if strings.Contains(teks, "Rp") {
		t.Error("CSV tidak boleh memuat format Rp")
	}
}
