package handler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/config"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
	"github.com/mua-restinpeace/koperasi-sejahtera/templates"
)

type fakeLaporan struct {
	pendapatan    service.LaporanPendapatan
	tunggakan     service.LaporanTunggakan
	errPendapatan error
	errTunggakan  error
	inPendapatan  service.PendapatanInput
	inTunggakan   service.TunggakanInput

	daftar   []model.PinjamanKartu
	kartu    service.KartuPinjaman
	errKartu error
	inKartu  service.KartuInput

	dashboard    service.Dashboard
	errDashboard error

	wajib    service.LaporanWajib
	errWajib error
	inWajib  service.WajibInput
}

func (f *fakeLaporan) Pendapatan(_ context.Context, in service.PendapatanInput) (service.LaporanPendapatan, error) {
	f.inPendapatan = in
	return f.pendapatan, f.errPendapatan
}

func (f *fakeLaporan) Tunggakan(_ context.Context, in service.TunggakanInput) (service.LaporanTunggakan, error) {
	f.inTunggakan = in
	return f.tunggakan, f.errTunggakan
}

func (f *fakeLaporan) DaftarPinjaman(context.Context) ([]model.PinjamanKartu, error) {
	return f.daftar, nil
}

func (f *fakeLaporan) KartuPinjaman(_ context.Context, in service.KartuInput) (service.KartuPinjaman, error) {
	f.inKartu = in
	return f.kartu, f.errKartu
}

func (f *fakeLaporan) Dashboard(context.Context) (service.Dashboard, error) {
	return f.dashboard, f.errDashboard
}

func (f *fakeLaporan) WajibBelumBayar(_ context.Context, in service.WajibInput) (service.LaporanWajib, error) {
	f.inWajib = in
	return f.wajib, f.errWajib
}

func tanggalUji(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

// dataContohC menghasilkan laporan dari skenario Contoh C (spec bagian 6),
// memakai fungsi perhitungan asli, bukan angka yang diketik tangan.
func dataContohC(t *testing.T) (service.LaporanPendapatan, service.LaporanTunggakan) {
	t.Helper()
	jadwal, err := helper.BuatJadwal(6_000_000, 6, 100, tanggalUji("2026-11-05"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range jadwal {
		jadwal[i].IDPinjaman = 1
	}

	bayar := []model.Pembayaran{
		{ID: 1, IDPinjaman: 1, Jenis: model.PembayaranAngsuran, TanggalBayar: tanggalUji("2027-01-15"), Jumlah: 1_500_000},
		{ID: 2, IDPinjaman: 1, Jenis: model.PembayaranAngsuran, TanggalBayar: tanggalUji("2027-02-12"), Jumlah: 800_000},
		{ID: 3, IDPinjaman: 1, Jenis: model.PembayaranPelunasan, TanggalBayar: tanggalUji("2027-02-20"), Jumlah: 4_178_000, BiayaAdmin: 80_000},
	}
	aturan := helper.AturanHitung{DendaPerHari: 2000, BiayaAdminBps: 200}
	alokasi, err := helper.HitungAlokasi(jadwal, bayar, aturan)
	if err != nil {
		t.Fatal(err)
	}

	tgl := map[int64]time.Time{}
	var pb []model.PembayaranLaporan
	for _, p := range bayar {
		tgl[p.ID] = p.TanggalBayar
		pb = append(pb, model.PembayaranLaporan{ID: p.ID, TanggalBayar: p.TanggalBayar, BiayaAdmin: p.BiayaAdmin})
	}
	var al []model.AlokasiLaporan
	for _, a := range alokasi {
		al = append(al, model.AlokasiLaporan{IDPembayaran: a.IDPembayaran, TanggalBayar: tgl[a.IDPembayaran], Denda: a.Denda, Jasa: a.Jasa, Pokok: a.Pokok})
	}
	bulan := service.HitungPendapatan(pb, al)

	pj := []model.PinjamanLaporan{{ID: 1, IDAnggota: 2, NomorAnggota: "KKS-0002", NamaAnggota: "Rudi Hartono", DendaPerHari: 2000}}
	baris, _ := service.HitungTunggakan(pj, jadwal, bayar, tanggalUji("2027-02-11"), 200)
	lt := service.LaporanTunggakan{Tanggal: tanggalUji("2027-02-11"), Baris: baris, JumlahAnggota: 1}
	for _, b := range baris {
		lt.TotalTagihan += b.Tagihan()
		lt.TotalDenda += b.DendaBerjalan
	}

	return service.LaporanPendapatan{Bulan: bulan, Total: service.JumlahPendapatan(bulan)}, lt
}

func siapLaporan(t *testing.T, f *fakeLaporan) *Handler {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h, err := NewHandler(config.Config{}, log, templates.FS, Service{Laporan: f})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	return h
}

func reqLogin(path string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	return r.WithContext(service.DenganIdentitas(r.Context(), service.Identitas{IDPengguna: 1, Username: "admin", NamaLengkap: "Budi Pratama", CSRFToken: "tok"}))
}

func simpanPratinjau(t *testing.T, env string, body string) {
	t.Helper()
	if out := os.Getenv(env); out != "" {
		_ = os.WriteFile(out, []byte(body), 0o644)
	}
}

func wajibMemuat(t *testing.T, body string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("body tidak memuat %q", w)
		}
	}
}

func TestLaporan_Indeks(t *testing.T) {
	h := siapLaporan(t, &fakeLaporan{})
	rec := httptest.NewRecorder()
	h.Laporan(rec, reqLogin("/laporan"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	wajibMemuat(t, rec.Body.String(), "/laporan/pendapatan", "/laporan/tunggakan", "Pendapatan bulanan", "Daftar tunggakan")
	simpanPratinjau(t, "KOPERASI_PREVIEW_LAPORAN", rec.Body.String())
}

func TestLaporanPendapatan_AngkaContohC(t *testing.T) {
	pend, _ := dataContohC(t)
	f := &fakeLaporan{pendapatan: pend}
	h := siapLaporan(t, f)

	rec := httptest.NewRecorder()
	h.LaporanPendapatan(rec, reqLogin("/laporan/pendapatan?dari=2027-01&sampai=2027-02"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if f.inPendapatan.Dari != "2027-01" || f.inPendapatan.Sampai != "2027-02" {
		t.Errorf("input = %+v", f.inPendapatan)
	}

	body := rec.Body.String()
	// spec 6C: Jan jasa 120.000 denda 82.000 admin 0; Feb jasa 120.000 denda 76.000 admin 80.000
	wajibMemuat(t, body, "Januari 2027", "Februari 2027", "Rp120.000", "Rp82.000", "Rp76.000", "Rp80.000", "Rp478.000")
	simpanPratinjau(t, "KOPERASI_PREVIEW_PENDAPATAN", body)
}

func TestLaporanPendapatan_Kosong(t *testing.T) {
	h := siapLaporan(t, &fakeLaporan{})
	rec := httptest.NewRecorder()
	h.LaporanPendapatan(rec, reqLogin("/laporan/pendapatan"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	wajibMemuat(t, rec.Body.String(), "Belum ada pembayaran pada periode ini")
}

func TestLaporanPendapatan_Validasi(t *testing.T) {
	f := &fakeLaporan{errPendapatan: &service.ValidasiError{Pesan: map[string]string{"sampai": "Bulan akhir tidak boleh sebelum bulan awal."}}}
	h := siapLaporan(t, f)
	rec := httptest.NewRecorder()
	h.LaporanPendapatan(rec, reqLogin("/laporan/pendapatan?dari=2027-03&sampai=2027-02"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	wajibMemuat(t, rec.Body.String(), "Bulan akhir tidak boleh sebelum bulan awal.", `value="2027-03"`, `value="2027-02"`)
}

func TestLaporanTunggakan_AngkaContohC(t *testing.T) {
	_, tung := dataContohC(t)
	f := &fakeLaporan{tunggakan: tung}
	h := siapLaporan(t, f)

	rec := httptest.NewRecorder()
	h.LaporanTunggakan(rec, reqLogin("/laporan/tunggakan?tanggal=2027-02-11"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if f.inTunggakan.Tanggal != "2027-02-11" {
		t.Errorf("input = %+v", f.inTunggakan)
	}

	body := rec.Body.String()
	// spec 6C arrears list per 2027-02-11: angsuran 2 pokok 702.000 denda 54.000 (32 hari); angsuran 3 jasa 60.000 pokok 1.000.000 denda 2.000
	wajibMemuat(t, body, "Rudi Hartono", "KKS-0002", "32 hari", "1 hari", "Rp702.000", "Rp54.000", "Rp1.060.000", "Rp2.000", `value="2027-02-11"`, "10-01-2027", "10-02-2027")
	simpanPratinjau(t, "KOPERASI_PREVIEW_TUNGGAKAN", body)
}

func TestLaporanTunggakan_Kosong(t *testing.T) {
	h := siapLaporan(t, &fakeLaporan{tunggakan: service.LaporanTunggakan{Tanggal: tanggalUji("2027-02-21")}})
	rec := httptest.NewRecorder()
	h.LaporanTunggakan(rec, reqLogin("/laporan/tunggakan?tanggal=2027-02-21"))
	wajibMemuat(t, rec.Body.String(), "Tidak ada angsuran yang menunggak")
}

func TestLaporanTunggakan_DataDilewatiTampil(t *testing.T) {
	f := &fakeLaporan{tunggakan: service.LaporanTunggakan{Tanggal: tanggalUji("2027-02-11"), Dilewati: []string{"Pinjaman #2 (KKS-0003): data tidak konsisten"}}}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanTunggakan(rec, reqLogin("/laporan/tunggakan"))
	wajibMemuat(t, rec.Body.String(), "Data dilewati", "Pinjaman #2 (KKS-0003)")
}

func TestLaporanTunggakan_Validasi(t *testing.T) {
	f := &fakeLaporan{errTunggakan: &service.ValidasiError{Pesan: map[string]string{"tanggal": "Tanggal harus berformat tahun-bulan-hari."}}}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanTunggakan(rec, reqLogin("/laporan/tunggakan?tanggal=kemarin"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	wajibMemuat(t, rec.Body.String(), "Tanggal harus berformat tahun-bulan-hari.")
}

func TestLaporan_GalatTakTerdugaTidakBocor(t *testing.T) {
	boom := errors.New("rahasia: password db salah")
	f := &fakeLaporan{errPendapatan: boom, errTunggakan: boom}
	h := siapLaporan(t, f)

	for _, fn := range []func(http.ResponseWriter, *http.Request){h.LaporanPendapatan, h.LaporanTunggakan} {
		rec := httptest.NewRecorder()
		fn(rec, reqLogin("/laporan/x"))
		if rec.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want 500", rec.Code)
		}
		if strings.Contains(rec.Body.String(), "rahasia") {
			t.Error("detail galat bocor ke pengguna")
		}
	}
}

// ---------------------------------------------------------------------
// L-1 kartu pinjaman dan ekspor CSV
// ---------------------------------------------------------------------

func kartuUji(t *testing.T, asOf string) service.KartuPinjaman {
	t.Helper()
	jadwal, err := helper.BuatJadwal(6_000_000, 6, 100, tanggalUji("2026-11-05"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range jadwal {
		jadwal[i].IDPinjaman = 1
	}
	bayar := []model.Pembayaran{
		{ID: 1, IDPinjaman: 1, Jenis: model.PembayaranAngsuran, TanggalBayar: tanggalUji("2027-01-15"), Jumlah: 1_500_000},
		{ID: 2, IDPinjaman: 1, Jenis: model.PembayaranAngsuran, TanggalBayar: tanggalUji("2027-02-12"), Jumlah: 800_000},
		{ID: 3, IDPinjaman: 1, Jenis: model.PembayaranPelunasan, TanggalBayar: tanggalUji("2027-02-20"), Jumlah: 4_178_000, BiayaAdmin: 80_000},
	}
	pj := model.PinjamanKartu{ID: 1, IDAnggota: 2, NomorAnggota: "KKS-0002", NamaAnggota: "Rudi Hartono", Jumlah: 6_000_000, TenorBulan: 6, JasaBps: 100, DendaPerHari: 2000, TanggalPencairan: tanggalUji("2026-11-05"), Status: "lunas", TanggalLunas: tanggalUji("2027-02-20")}
	k, err := service.HitungKartu(pj, jadwal, bayar, tanggalUji(asOf), 200)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func reqKartu(path string) *http.Request {
	r := reqLogin(path)
	r.SetPathValue("id", "1")
	return r
}

func TestLaporanPinjaman_Daftar(t *testing.T) {
	f := &fakeLaporan{daftar: []model.PinjamanKartu{{ID: 7, IDAnggota: 2, NomorAnggota: "KKS-0002", NamaAnggota: "Rudi Hartono", Jumlah: 6_000_000, TenorBulan: 6, TanggalPencairan: tanggalUji("2026-11-05"), Status: "aktif"}}}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanPinjaman(rec, reqLogin("/laporan/pinjaman"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	wajibMemuat(t, rec.Body.String(), "Rudi Hartono", "KKS-0002", "Rp6.000.000", "/laporan/pinjaman/7", "Aktif")
}

func TestLaporanPinjaman_DaftarKosong(t *testing.T) {
	rec := httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{}).LaporanPinjaman(rec, reqLogin("/laporan/pinjaman"))
	wajibMemuat(t, rec.Body.String(), "Belum ada pinjaman")
}

func TestLaporanKartu_ContohC(t *testing.T) {
	f := &fakeLaporan{kartu: kartuUji(t, "2027-02-11")}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanKartu(rec, reqKartu("/laporan/pinjaman/1?tanggal=2027-02-11"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if f.inKartu.ID != 1 || f.inKartu.Tanggal != "2027-02-11" {
		t.Errorf("input = %+v", f.inKartu)
	}
	body := rec.Body.String()
	wajibMemuat(t, body, "Kartu Pinjaman #1", "Rudi Hartono", "Rp6.000.000", "Lunas", "Menunggak 32 hari", "Menunggak 1 hari", "Rp54.000", "Rp702.000",
		"Belum jatuh tempo", "15-01-2027", "/laporan/pinjaman/1/csv?tanggal=2027-02-11", `value="2027-02-11"`)
	simpanPratinjau(t, "KOPERASI_PREVIEW_KARTU", body)
}

func TestLaporanKartu_SetelahPelunasan(t *testing.T) {
	f := &fakeLaporan{kartu: kartuUji(t, "2027-02-20")}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanKartu(rec, reqKartu("/laporan/pinjaman/1?tanggal=2027-02-20"))
	body := rec.Body.String()
	wajibMemuat(t, body, "Pelunasan dipercepat", "Rp4.178.000", "Rp80.000")
	if strings.Contains(body, "Menunggak") {
		t.Error("setelah pelunasan tidak boleh ada angsuran menunggak")
	}
}

func TestLaporanKartu_TidakAdaDan404(t *testing.T) {
	f := &fakeLaporan{errKartu: service.ErrKartuPinjamanTidakAda}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanKartu(rec, reqKartu("/laporan/pinjaman/1"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	r := reqLogin("/laporan/pinjaman/abc")
	r.SetPathValue("id", "abc")
	siapLaporan(t, &fakeLaporan{}).LaporanKartu(rec, r)
	if rec.Code != http.StatusNotFound {
		t.Errorf("id bukan angka: status = %d, want 404", rec.Code)
	}
}

func TestLaporanKartu_Validasi(t *testing.T) {
	f := &fakeLaporan{errKartu: &service.ValidasiError{Pesan: map[string]string{"tanggal": "Tanggal harus berformat tahun-bulan-hari."}}}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanKartu(rec, reqKartu("/laporan/pinjaman/1?tanggal=kemarin"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	wajibMemuat(t, rec.Body.String(), "Tanggal harus berformat tahun-bulan-hari.")
}

func cekUnduhan(t *testing.T, rec *httptest.ResponseRecorder, namaBerkas string) string {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q", ct)
	}
	if cd := rec.Header().Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, namaBerkas) {
		t.Errorf("Content-Disposition = %q", cd)
	}
	if !strings.HasPrefix(rec.Body.String(), "\xEF\xBB\xBF") {
		t.Error("CSV harus diawali BOM UTF-8")
	}
	return rec.Body.String()
}

func TestLaporanCSV_Pendapatan(t *testing.T) {
	pend, _ := dataContohC(t)
	f := &fakeLaporan{pendapatan: pend}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanPendapatanCSV(rec, reqLogin("/laporan/pendapatan/csv?dari=2027-01&sampai=2027-02"))
	body := cekUnduhan(t, rec, "pendapatan-bulanan.csv")
	wajibMemuat(t, body, "Januari 2027;120000;82000;0;202000", "Total;240000;158000;80000;478000")
	if f.inPendapatan.Dari != "2027-01" {
		t.Errorf("filter tidak diteruskan: %+v", f.inPendapatan)
	}

	rec = httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{errPendapatan: &service.ValidasiError{Pesan: map[string]string{"dari": "x"}}}).LaporanPendapatanCSV(rec, reqLogin("/laporan/pendapatan/csv?dari=salah"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("filter salah: status = %d, want 400", rec.Code)
	}
}

func TestLaporanCSV_Tunggakan(t *testing.T) {
	_, tung := dataContohC(t)
	rec := httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{tunggakan: tung}).LaporanTunggakanCSV(rec, reqLogin("/laporan/tunggakan/csv?tanggal=2027-02-11"))
	body := cekUnduhan(t, rec, "tunggakan-2027-02-11.csv")
	wajibMemuat(t, body, "KKS-0002;Rudi Hartono;1;2;2027-01-10;32;0;702000;54000;756000")
}

func TestLaporanCSV_Kartu(t *testing.T) {
	rec := httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{kartu: kartuUji(t, "2027-02-20")}).LaporanKartuCSV(rec, reqKartu("/laporan/pinjaman/1/csv"))
	body := cekUnduhan(t, rec, "kartu-pinjaman-1.csv")
	wajibMemuat(t, body, "Kartu pinjaman;#1", "Total sisa tagihan;0", "2027-02-20;pelunasan;4178000;80000")

	rec = httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{errKartu: service.ErrKartuPinjamanTidakAda}).LaporanKartuCSV(rec, reqKartu("/laporan/pinjaman/1/csv"))
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// ---------------------------------------------------------------------
// L-4 simpanan wajib belum dibayar
// ---------------------------------------------------------------------

func wajibUji() service.LaporanWajib {
	return service.HitungLaporanWajib(tanggalUji("2027-02-01"), []model.AnggotaWajibBelum{
		{ID: 2, NomorAnggota: "KKS-0002", NamaAnggota: "Dewi Anggraini"},
		{ID: 5, NomorAnggota: "KKS-0005", NamaAnggota: "Maya Putri"},
	}, tanggalUji("2027-02-15"), 100_000)
}

func TestLaporanWajib_Halaman(t *testing.T) {
	f := &fakeLaporan{wajib: wajibUji()}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanWajib(rec, reqLogin("/laporan/simpanan-wajib?periode=2027-02"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if f.inWajib.Periode != "2027-02" {
		t.Errorf("filter tidak diteruskan: %+v", f.inWajib)
	}
	wajibMemuat(t, rec.Body.String(), "Simpanan Wajib Belum Dibayar", "Februari 2027", "10-02-2027", "2 anggota", "Rp200.000",
		"Dewi Anggraini", "KKS-0005", "Terlambat 5 hari", "/simpanan/2", "/anggota/5", `value="2027-02"`,
		"/laporan/simpanan-wajib/csv?periode=2027-02")
}

func TestLaporanWajib_BelumJatuhTempoDanKosong(t *testing.T) {
	l := service.HitungLaporanWajib(tanggalUji("2027-02-01"), nil, tanggalUji("2027-02-05"), 100_000)
	rec := httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{wajib: l}).LaporanWajib(rec, reqLogin("/laporan/simpanan-wajib"))
	body := rec.Body.String()
	wajibMemuat(t, body, "Belum jatuh tempo", "Semua anggota sudah membayar")
	if strings.Contains(body, "Terlambat") {
		t.Error("belum jatuh tempo tidak boleh menampilkan Terlambat")
	}
}

func TestLaporanWajib_Validasi(t *testing.T) {
	f := &fakeLaporan{errWajib: &service.ValidasiError{Pesan: map[string]string{"periode": "Periode harus berformat tahun-bulan."}}}
	rec := httptest.NewRecorder()
	siapLaporan(t, f).LaporanWajib(rec, reqLogin("/laporan/simpanan-wajib?periode=x"))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
	wajibMemuat(t, rec.Body.String(), "Periode harus berformat tahun-bulan.")
}

func TestLaporanWajib_CSV(t *testing.T) {
	rec := httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{wajib: wajibUji()}).LaporanWajibCSV(rec, reqLogin("/laporan/simpanan-wajib/csv?periode=2027-02"))
	body := cekUnduhan(t, rec, "simpanan-wajib-belum-2027-02.csv")
	wajibMemuat(t, body, "KKS-0002;Dewi Anggraini;2027-02;2027-02-10;5;100000", "Total;2 anggota;;;;200000")

	rec = httptest.NewRecorder()
	siapLaporan(t, &fakeLaporan{errWajib: &service.ValidasiError{Pesan: map[string]string{"periode": "x"}}}).LaporanWajibCSV(rec, reqLogin("/laporan/simpanan-wajib/csv?periode=x"))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("periode salah: status = %d, want 400", rec.Code)
	}
}
