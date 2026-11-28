package service

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

func tgl(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

var aturanUji = helper.AturanHitung{DendaPerHari: 2000, BiayaAdminBps: 200}

// contohC membangun skenario Contoh C dari spec-perhitungan.md bagian 6:
// pinjaman Rp6.000.000, tenor 6, cair 2026-11-05, dua pembayaran sebagian,
// lalu pelunasan dipercepat 2027-02-20.
func contohC(t *testing.T, idPinjaman int64) ([]model.Angsuran, []model.Pembayaran) {
	t.Helper()
	jadwal, err := helper.BuatJadwal(6_000_000, 6, 100, tgl("2026-11-05"))
	if err != nil {
		t.Fatal(err)
	}
	for i := range jadwal {
		jadwal[i].IDPinjaman = idPinjaman
	}

	bayar := []model.Pembayaran{
		{ID: 1, IDPinjaman: idPinjaman, Jenis: model.PembayaranAngsuran, TanggalBayar: tgl("2027-01-15"), Jumlah: 1_500_000},
		{ID: 2, IDPinjaman: idPinjaman, Jenis: model.PembayaranAngsuran, TanggalBayar: tgl("2027-02-12"), Jumlah: 800_000},
		{ID: 3, IDPinjaman: idPinjaman, Jenis: model.PembayaranPelunasan, TanggalBayar: tgl("2027-02-20"), Jumlah: 4_178_000, BiayaAdmin: 80_000},
	}
	return jadwal, bayar
}

// alokasiLaporan meniru isi tabel alokasi_pembayaran: alokasi hasil perhitungan
// diberi tanggal bayar pembayarannya.
func alokasiLaporan(t *testing.T, jadwal []model.Angsuran, bayar []model.Pembayaran) ([]model.PembayaranLaporan, []model.AlokasiLaporan) {
	t.Helper()
	alokasi, err := helper.HitungAlokasi(jadwal, bayar, aturanUji)
	if err != nil {
		t.Fatalf("HitungAlokasi: %v", err)
	}

	tanggal := map[int64]time.Time{}
	var pb []model.PembayaranLaporan
	for _, p := range bayar {
		tanggal[p.ID] = p.TanggalBayar
		pb = append(pb, model.PembayaranLaporan{ID: p.ID, TanggalBayar: p.TanggalBayar, BiayaAdmin: p.BiayaAdmin})
	}

	var al []model.AlokasiLaporan
	for _, a := range alokasi {
		al = append(al, model.AlokasiLaporan{IDPembayaran: a.IDPembayaran, TanggalBayar: tanggal[a.IDPembayaran], Denda: a.Denda, Jasa: a.Jasa, Pokok: a.Pokok})
	}
	return pb, al
}

// ---------------------------------------------------------------------
// L-3 Pendapatan
// ---------------------------------------------------------------------

func TestHitungPendapatan_ContohC(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	pb, al := alokasiLaporan(t, jadwal, bayar)

	got := HitungPendapatan(pb, al)

	// spec 6C, "Monthly income report (cash basis)"
	want := []struct {
		bulan              string
		jasa, denda, admin int64
	}{
		{"2027-01", 120_000, 82_000, 0},
		{"2027-02", 120_000, 76_000, 80_000},
	}
	if len(got) != len(want) {
		t.Fatalf("jumlah bulan = %d, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		g := got[i]
		if g.Bulan.Format("2006-01") != w.bulan || g.Jasa != w.jasa || g.Denda != w.denda || g.BiayaAdmin != w.admin {
			t.Errorf("bulan %d = %s jasa %d denda %d admin %d, want %+v", i, g.Bulan.Format("2006-01"), g.Jasa, g.Denda, g.BiayaAdmin, w)
		}
	}

	if tot := JumlahPendapatan(got); tot.Jasa != 240_000 || tot.Denda != 158_000 || tot.BiayaAdmin != 80_000 || tot.Total() != 478_000 {
		t.Errorf("total = %+v (Total %d)", tot, tot.Total())
	}
}

func TestHitungPendapatan_PembatalanDiabaikan(t *testing.T) {
	// Pembayaran yang dibatalkan tidak ikut dimasukkan oleh repository (dibatalkan_pada IS NULL).
	// Spec 6D: pembayaran 1 dibatalkan, pembayaran 2 dihitung ulang sendirian:
	// denda 128.000 + jasa 60.000 + pokok 612.000 = 800.000, jatuh di bulan Februari.
	jadwal, _ := contohC(t, 1)
	sisa := []model.Pembayaran{{ID: 2, IDPinjaman: 1, Jenis: model.PembayaranAngsuran, TanggalBayar: tgl("2027-02-12"), Jumlah: 800_000}}
	pb, al := alokasiLaporan(t, jadwal, sisa)

	got := HitungPendapatan(pb, al)
	if len(got) != 1 || got[0].Bulan.Format("2006-01") != "2027-02" {
		t.Fatalf("hasil = %+v, hanya Februari yang diharapkan", got)
	}
	if got[0].Jasa != 60_000 || got[0].Denda != 128_000 || got[0].BiayaAdmin != 0 {
		t.Errorf("Februari = jasa %d denda %d admin %d, want 60.000 / 128.000 / 0", got[0].Jasa, got[0].Denda, got[0].BiayaAdmin)
	}
}

func TestHitungPendapatan_UrutanDanTahun(t *testing.T) {
	al := []model.AlokasiLaporan{
		{IDPembayaran: 3, TanggalBayar: tgl("2027-01-05"), Jasa: 30},
		{IDPembayaran: 1, TanggalBayar: tgl("2026-12-31"), Jasa: 10},
		{IDPembayaran: 2, TanggalBayar: tgl("2026-12-01"), Denda: 5},
	}
	got := HitungPendapatan(nil, al)
	if len(got) != 2 {
		t.Fatalf("hasil = %+v", got)
	}
	if got[0].Nama() != "Desember 2026" || got[0].Jasa != 10 || got[0].Denda != 5 {
		t.Errorf("baris 0 = %s %+v", got[0].Nama(), got[0])
	}
	if got[1].Nama() != "Januari 2027" || got[1].Jasa != 30 {
		t.Errorf("baris 1 = %s %+v", got[1].Nama(), got[1])
	}
}

func TestHitungPendapatan_Kosong(t *testing.T) {
	if got := HitungPendapatan(nil, nil); len(got) != 0 {
		t.Errorf("hasil = %+v, want kosong", got)
	}
	if tot := JumlahPendapatan(nil); tot.Total() != 0 {
		t.Errorf("total kosong = %d", tot.Total())
	}
}

// ---------------------------------------------------------------------
// L-2 Tunggakan
// ---------------------------------------------------------------------

func pinjamanUji(id int64) model.PinjamanLaporan {
	return model.PinjamanLaporan{ID: id, IDAnggota: 10 + id, NomorAnggota: "KKS-000" + string(rune('0'+id)), NamaAnggota: "Anggota " + string(rune('0'+id)), DendaPerHari: 2000}
}

type barisUji struct {
	urutan, hari       int
	jasa, pokok, denda int64
}

func ringkas(b []BarisTunggakan) []barisUji {
	var r []barisUji
	for _, x := range b {
		r = append(r, barisUji{x.Urutan, x.HariTerlambat, x.SisaJasa, x.SisaPokok, x.DendaBerjalan})
	}
	return r
}

func TestHitungTunggakan_ContohC(t *testing.T) {
	jadwal, bayar := contohC(t, 1)

	tests := []struct {
		nama string
		asOf string
		want []barisUji
	}{
		{
			// spec 6C "Arrears list as of 2027-02-11 (before payment 2)"
			nama: "2027-02-11 sebelum pembayaran 2",
			asOf: "2027-02-11",
			want: []barisUji{
				{urutan: 2, hari: 32, jasa: 0, pokok: 702_000, denda: 54_000},
				{urutan: 3, hari: 1, jasa: 60_000, pokok: 1_000_000, denda: 2_000},
			},
		},
		{
			nama: "tepat jatuh tempo belum terlambat",
			asOf: "2027-02-10",
			want: []barisUji{
				{urutan: 2, hari: 31, jasa: 0, pokok: 702_000, denda: 52_000},
			},
		},
		{
			// pembayaran 2 tanggal 2027-02-12: angsuran 2 lunas, angsuran 3 sisa jasa 22.000 + pokok 1.000.000
			nama: "2027-02-12 setelah pembayaran 2",
			asOf: "2027-02-12",
			want: []barisUji{
				{urutan: 3, hari: 2, jasa: 22_000, pokok: 1_000_000, denda: 0},
			},
		},
		{
			nama: "setelah pelunasan dipercepat tidak ada tunggakan",
			asOf: "2027-02-21",
			want: nil,
		},
		{
			nama: "sebelum jatuh tempo pertama",
			asOf: "2026-12-10",
			want: nil,
		},
		{
			nama: "sehari setelah jatuh tempo pertama",
			asOf: "2026-12-11",
			want: []barisUji{{urutan: 1, hari: 1, jasa: 60_000, pokok: 1_000_000, denda: 2_000}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			baris, dilewati := HitungTunggakan([]model.PinjamanLaporan{pinjamanUji(1)}, jadwal, bayar, tgl(tt.asOf), 200)
			if len(dilewati) != 0 {
				t.Fatalf("dilewati = %v", dilewati)
			}
			if got := ringkas(baris); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("baris = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHitungTunggakan_TagihanDanTotal(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	baris, _ := HitungTunggakan([]model.PinjamanLaporan{pinjamanUji(1)}, jadwal, bayar, tgl("2027-02-11"), 200)
	if len(baris) != 2 {
		t.Fatalf("baris = %+v", baris)
	}
	if baris[0].Tagihan() != 702_000 || baris[0].Total() != 756_000 {
		t.Errorf("baris 0 tagihan %d total %d", baris[0].Tagihan(), baris[0].Total())
	}
	if baris[1].Tagihan() != 1_060_000 || baris[1].Total() != 1_062_000 {
		t.Errorf("baris 1 tagihan %d total %d", baris[1].Tagihan(), baris[1].Total())
	}
}

func TestHitungTunggakan_UrutanPalingLamaDiAtas(t *testing.T) {
	j1, b1 := contohC(t, 1)
	j2, _ := contohC(t, 2)
	// pinjaman 1 sudah membayar 1.500.000 pada 2027-01-15; pinjaman 2 tidak pernah membayar
	semua := append(append([]model.Angsuran{}, j1...), j2...)

	baris, _ := HitungTunggakan([]model.PinjamanLaporan{pinjamanUji(1), pinjamanUji(2)}, semua, b1, tgl("2027-01-20"), 200)

	// paling lama menunggak di atas; sama lama: nomor anggota lebih kecil dulu
	type kunci struct {
		pinjaman int64
		urutan   int
	}
	var got []kunci
	for _, b := range baris {
		got = append(got, kunci{b.IDPinjaman, b.Urutan})
	}
	want := []kunci{{2, 1}, {1, 2}, {2, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("urutan = %+v, want %+v", got, want)
	}
	if baris[0].HariTerlambat != 41 {
		t.Errorf("baris teratas terlambat %d hari, want 41", baris[0].HariTerlambat)
	}
}

func TestHitungTunggakan_DataTidakKonsistenDilewati(t *testing.T) {
	j1, b1 := contohC(t, 1)
	j2, _ := contohC(t, 2)
	// pembayaran melebihi seluruh sisa kewajiban pada pinjaman 2
	rusak := []model.Pembayaran{{ID: 9, IDPinjaman: 2, Jenis: model.PembayaranAngsuran, TanggalBayar: tgl("2027-01-01"), Jumlah: 99_000_000}}
	semua := append(append([]model.Angsuran{}, j1...), j2...)

	baris, dilewati := HitungTunggakan([]model.PinjamanLaporan{pinjamanUji(1), pinjamanUji(2)}, semua, append(b1, rusak...), tgl("2027-02-11"), 200)

	if len(dilewati) != 1 || !strings.Contains(dilewati[0], "#2") {
		t.Fatalf("dilewati = %v, want satu entri untuk pinjaman #2", dilewati)
	}
	if len(baris) != 2 {
		t.Errorf("pinjaman 1 tetap dihitung: baris = %+v", baris)
	}
}

// ---------------------------------------------------------------------
// Service
// ---------------------------------------------------------------------

type fakeLaporanRepo struct {
	dari, sampai *time.Time
	pembayaran   []model.PembayaranLaporan
	alokasi      []model.AlokasiLaporan
	pinjaman     []model.PinjamanLaporan
	angsuran     []model.Angsuran
	bayar        []model.Pembayaran
	asOf         time.Time
	galat        error

	daftar []model.PinjamanKartu
	kartu  *model.PinjamanKartu // nil = pinjaman tidak ada

	aktif, baru  int
	simpanan     int64
	wajib        []model.AnggotaWajibBelum
	totalWajib   int
	periodeWajib time.Time
	batasWajib   int
}

func (f *fakeLaporanRepo) Pembayaran(_ context.Context, _ database.DBTX, d, s *time.Time) ([]model.PembayaranLaporan, error) {
	f.dari, f.sampai = d, s
	return f.pembayaran, f.galat
}
func (f *fakeLaporanRepo) Alokasi(_ context.Context, _ database.DBTX, d, s *time.Time) ([]model.AlokasiLaporan, error) {
	return f.alokasi, f.galat
}
func (f *fakeLaporanRepo) PinjamanBerjalan(_ context.Context, _ database.DBTX, asOf time.Time) ([]model.PinjamanLaporan, error) {
	f.asOf = asOf
	return f.pinjaman, f.galat
}
func (f *fakeLaporanRepo) AngsuranBerjalan(_ context.Context, _ database.DBTX, _ time.Time) ([]model.Angsuran, error) {
	return f.angsuran, f.galat
}
func (f *fakeLaporanRepo) PembayaranBerjalan(_ context.Context, _ database.DBTX, _ time.Time) ([]model.Pembayaran, error) {
	return f.bayar, f.galat
}

func svcLaporan(repo *fakeLaporanRepo, sekarang string) *LaporanService {
	return NewLaporanService(&fakeDB{}, repo, 200, time.UTC, func() time.Time { return tgl(sekarang).Add(10 * time.Hour) })
}

func TestLaporan_BelumLogin(t *testing.T) {
	s := svcLaporan(&fakeLaporanRepo{}, "2027-02-11")
	if _, err := s.Pendapatan(context.Background(), PendapatanInput{}); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("Pendapatan err = %v, want ErrBelumLogin", err)
	}
	if _, err := s.Tunggakan(context.Background(), TunggakanInput{}); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("Tunggakan err = %v, want ErrBelumLogin", err)
	}
}

func TestLaporan_PendapatanValidasi(t *testing.T) {
	tests := []struct {
		nama  string
		in    PendapatanInput
		kolom string
	}{
		{"format dari salah", PendapatanInput{Dari: "Jan 2027"}, "dari"},
		{"format sampai salah", PendapatanInput{Sampai: "2027-13"}, "sampai"},
		{"sampai sebelum dari", PendapatanInput{Dari: "2027-03", Sampai: "2027-02"}, "sampai"},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			_, err := svcLaporan(&fakeLaporanRepo{}, "2027-02-11").Pendapatan(ctxLogin(), tt.in)
			var ve *ValidasiError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want ValidasiError", err)
			}
			if _, ok := ve.Pesan[tt.kolom]; !ok {
				t.Errorf("pesan = %v, want kolom %q", ve.Pesan, tt.kolom)
			}
		})
	}
}

func TestLaporan_PendapatanRentang(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	pb, al := alokasiLaporan(t, jadwal, bayar)
	repo := &fakeLaporanRepo{pembayaran: pb, alokasi: al}

	hasil, err := svcLaporan(repo, "2027-03-01").Pendapatan(ctxLogin(), PendapatanInput{Dari: "2027-01", Sampai: "2027-02"})
	if err != nil {
		t.Fatal(err)
	}
	if repo.dari == nil || repo.dari.Format("2006-01-02") != "2027-01-01" {
		t.Errorf("dari = %v, want 2027-01-01", repo.dari)
	}
	if repo.sampai == nil || repo.sampai.Format("2006-01-02") != "2027-02-28" {
		t.Errorf("sampai = %v, want 2027-02-28 (hari terakhir bulan)", repo.sampai)
	}
	if len(hasil.Bulan) != 2 || hasil.Total.Total() != 478_000 {
		t.Errorf("hasil = %+v", hasil)
	}

	// tanpa filter: tanpa batas
	repo2 := &fakeLaporanRepo{}
	if _, err := svcLaporan(repo2, "2027-03-01").Pendapatan(ctxLogin(), PendapatanInput{}); err != nil {
		t.Fatal(err)
	}
	if repo2.dari != nil || repo2.sampai != nil {
		t.Errorf("tanpa filter harus tanpa batas, dapat %v %v", repo2.dari, repo2.sampai)
	}
}

func TestLaporan_TunggakanTanggal(t *testing.T) {
	jadwal, bayar := contohC(t, 1)
	repo := &fakeLaporanRepo{pinjaman: []model.PinjamanLaporan{pinjamanUji(1)}, angsuran: jadwal, bayar: bayar}
	s := svcLaporan(repo, "2027-02-20")

	// bawaan: hari ini (2027-02-20, pinjaman baru lunas hari itu)
	hasil, err := s.Tunggakan(ctxLogin(), TunggakanInput{})
	if err != nil {
		t.Fatal(err)
	}
	if hasil.Tanggal.Format("2006-01-02") != "2027-02-20" || repo.asOf.Format("2006-01-02") != "2027-02-20" {
		t.Errorf("tanggal bawaan = %v / %v", hasil.Tanggal, repo.asOf)
	}
	if len(hasil.Baris) != 0 {
		t.Errorf("baris = %+v, pelunasan sudah dilakukan", hasil.Baris)
	}

	// tanggal pilihan: angka spec 6C
	hasil, err = s.Tunggakan(ctxLogin(), TunggakanInput{Tanggal: "2027-02-11"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hasil.Baris) != 2 || hasil.JumlahAnggota != 1 {
		t.Fatalf("hasil = %+v", hasil)
	}
	if hasil.TotalTagihan != 702_000+1_060_000 || hasil.TotalDenda != 56_000 || hasil.TotalSemua() != 1_818_000 {
		t.Errorf("total tagihan %d denda %d semua %d", hasil.TotalTagihan, hasil.TotalDenda, hasil.TotalSemua())
	}

	// format salah
	_, err = s.Tunggakan(ctxLogin(), TunggakanInput{Tanggal: "11/02/2027"})
	var ve *ValidasiError
	if !errors.As(err, &ve) || ve.Pesan["tanggal"] == "" {
		t.Errorf("err = %v, want ValidasiError kolom tanggal", err)
	}
}

func TestLaporan_GalatRepository(t *testing.T) {
	boom := errors.New("db mati")
	s := svcLaporan(&fakeLaporanRepo{galat: boom}, "2027-02-11")
	if _, err := s.Pendapatan(ctxLogin(), PendapatanInput{}); !errors.Is(err, boom) {
		t.Errorf("Pendapatan err = %v", err)
	}
	if _, err := s.Tunggakan(ctxLogin(), TunggakanInput{}); !errors.Is(err, boom) {
		t.Errorf("Tunggakan err = %v", err)
	}
}
