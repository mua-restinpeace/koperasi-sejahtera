package helper

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

var aturanUji = AturanHitung{DendaPerHari: 2_000, BiayaAdminBps: 200}

// jadwalC adalah pinjaman contoh C: Rp6.000.000, tenor 6, dicairkan 2026-11-05.
// Setiap angsuran: pokok 1.000.000, jasa 60.000. Jatuh tempo 2026-12-10, 2027-01-10,
// 2027-02-10, 2027-03-10, 2027-04-10, 2027-05-10.
func jadwalC(t *testing.T) []model.Angsuran {
	t.Helper()
	j, err := BuatJadwal(6_000_000, 6, 100, tgl(2026, time.November, 5))
	if err != nil {
		t.Fatalf("BuatJadwal: %v", err)
	}
	return j
}

func bayar(id int64, tanggal time.Time, jumlah int64) model.Pembayaran {
	return model.Pembayaran{ID: id, Jenis: model.PembayaranAngsuran, TanggalBayar: tanggal, Jumlah: jumlah}
}

func al(id int64, urutan int, denda, jasa, pokok int64) model.Alokasi {
	return model.Alokasi{IDPembayaran: id, Urutan: urutan, Denda: denda, Jasa: jasa, Pokok: pokok}
}

// cekJumlah memastikan setiap pembayaran teralokasi tepat sebesar uang yang masuk
// (di luar biaya administrasi), dan tidak ada angsuran yang dibayar melebihi tagihannya.
func cekJumlah(t *testing.T, jadwal []model.Angsuran, pembayaran []model.Pembayaran, alokasi []model.Alokasi) {
	t.Helper()
	per := map[int64]int64{}
	pokok := map[int]int64{}
	jasa := map[int]int64{}
	for _, a := range alokasi {
		per[a.IDPembayaran] += a.Denda + a.Jasa + a.Pokok
		pokok[a.Urutan] += a.Pokok
		jasa[a.Urutan] += a.Jasa
	}
	for _, p := range pembayaran {
		if want := p.Jumlah - p.BiayaAdmin; per[p.ID] != want {
			t.Errorf("pembayaran %d teralokasi Rp%d, want Rp%d", p.ID, per[p.ID], want)
		}
	}
	for _, a := range jadwal {
		if pokok[a.Urutan] > a.Pokok || jasa[a.Urutan] > a.Jasa {
			t.Errorf("angsuran %d dibayar melebihi tagihan: pokok %d/%d jasa %d/%d",
				a.Urutan, pokok[a.Urutan], a.Pokok, jasa[a.Urutan], a.Jasa)
		}
	}
}

func TestHitungDenda(t *testing.T) {
	wib := time.FixedZone("WIB", 7*3600)
	jatuhTempo := tgl(2026, time.December, 10)
	tests := []struct {
		nama  string
		akhir time.Time
		want  int64
	}{
		{"tepat pada jatuh tempo tidak kena denda", tgl(2026, time.December, 10), 0},
		{"sehari setelah jatuh tempo", tgl(2026, time.December, 11), 2_000},
		{"sebelum jatuh tempo", tgl(2026, time.December, 1), 0},
		{"36 hari terlambat (contoh C)", tgl(2027, time.January, 15), 72_000},
		{"jam dan zona waktu diabaikan", time.Date(2026, time.December, 10, 23, 59, 0, 0, wib), 0},
		{"tanggal 11 pukul 00:01 WIB tetap sehari", time.Date(2026, time.December, 11, 0, 1, 0, 0, wib), 2_000},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			if got := HitungDenda(jatuhTempo, tt.akhir, 2_000); got != tt.want {
				t.Errorf("HitungDenda = %d, want %d", got, tt.want)
			}
		})
	}
}

// Contoh C pada docs/spec-perhitungan.md: dua pembayaran sebagian melewati beberapa
// angsuran yang terlambat.
func TestHitungAlokasi_ContohC(t *testing.T) {
	jadwal := jadwalC(t)
	p1 := bayar(1, tgl(2027, time.January, 15), 1_500_000)
	p2 := bayar(2, tgl(2027, time.February, 12), 800_000)

	got, err := HitungAlokasi(jadwal, []model.Pembayaran{p1, p2}, aturanUji)
	if err != nil {
		t.Fatalf("HitungAlokasi: %v", err)
	}
	want := []model.Alokasi{
		al(1, 1, 72_000, 60_000, 1_000_000),
		al(1, 2, 10_000, 60_000, 298_000),
		al(2, 2, 56_000, 0, 702_000),
		al(2, 3, 4_000, 38_000, 0),
	}
	if !slices.Equal(got, want) {
		t.Errorf("alokasi = %+v\nwant     %+v", got, want)
	}
	cekJumlah(t, jadwal, []model.Pembayaran{p1, p2}, got)

	t.Run("urutan masukan tidak berpengaruh", func(t *testing.T) {
		dibalik, err := HitungAlokasi(jadwal, []model.Pembayaran{p2, p1}, aturanUji)
		if err != nil {
			t.Fatalf("HitungAlokasi: %v", err)
		}
		if !slices.Equal(dibalik, want) {
			t.Errorf("hasil berbeda bila masukan dibalik: %+v", dibalik)
		}
	})
}

// Daftar tunggakan pada contoh C per 2027-02-11, setelah pembayaran 1 dan sebelum pembayaran 2.
func TestHitungStatus_TunggakanContohC(t *testing.T) {
	jadwal := jadwalC(t)
	p1 := bayar(1, tgl(2027, time.January, 15), 1_500_000)
	p2 := bayar(2, tgl(2027, time.February, 12), 800_000) // setelah tanggal perhitungan: diabaikan

	status, err := HitungStatus(jadwal, []model.Pembayaran{p1, p2}, tgl(2027, time.February, 11), aturanUji)
	if err != nil {
		t.Fatalf("HitungStatus: %v", err)
	}
	tests := []struct {
		urutan              int
		sisaPokok, sisaJasa int64
		denda               int64
		lunas               bool
	}{
		{1, 0, 0, 0, true},
		{2, 702_000, 0, 54_000, false}, // 32 hari = 64.000, dikurangi 10.000 yang sudah dibayar
		{3, 1_000_000, 60_000, 2_000, false},
		{4, 1_000_000, 60_000, 0, false}, // belum jatuh tempo
	}
	for _, tt := range tests {
		s := status[tt.urutan-1]
		if s.SisaPokok() != tt.sisaPokok || s.SisaJasa() != tt.sisaJasa || s.DendaTerutang != tt.denda || s.IsLunas() != tt.lunas {
			t.Errorf("angsuran %d: sisa pokok %d, jasa %d, denda %d, lunas %v; want %d, %d, %d, %v",
				tt.urutan, s.SisaPokok(), s.SisaJasa(), s.DendaTerutang, s.IsLunas(),
				tt.sisaPokok, tt.sisaJasa, tt.denda, tt.lunas)
		}
	}
}

// Contoh C: pelunasan dipercepat pada 2027-02-20, lalu pendapatan bulanan.
func TestPelunasan_ContohC(t *testing.T) {
	jadwal := jadwalC(t)
	p1 := bayar(1, tgl(2027, time.January, 15), 1_500_000)
	p2 := bayar(2, tgl(2027, time.February, 12), 800_000)
	asOf := tgl(2027, time.February, 20)

	status, err := HitungStatus(jadwal, []model.Pembayaran{p1, p2}, asOf, aturanUji)
	if err != nil {
		t.Fatalf("HitungStatus: %v", err)
	}
	rincian := HitungPelunasan(status, asOf, aturanUji.BiayaAdminBps)
	wantRincian := model.RincianPelunasan{
		SisaPokok:         4_000_000,
		Denda:             16_000, // 10 hari = 20.000, dikurangi 4.000 yang sudah dibayar
		JasaJatuhTempo:    22_000,
		JasaBulanBerjalan: 60_000, // angsuran 4
		BiayaAdmin:        80_000,
		Total:             4_178_000,
	}
	if rincian != wantRincian {
		t.Fatalf("rincian = %+v, want %+v", rincian, wantRincian)
	}

	p3 := model.Pembayaran{ID: 3, Jenis: model.PembayaranPelunasan, TanggalBayar: asOf, Jumlah: rincian.Total, BiayaAdmin: rincian.BiayaAdmin}
	semua := []model.Pembayaran{p1, p2, p3}
	alokasi, err := HitungAlokasi(jadwal, semua, aturanUji)
	if err != nil {
		t.Fatalf("HitungAlokasi: %v", err)
	}
	wantAlokasi := []model.Alokasi{
		al(1, 1, 72_000, 60_000, 1_000_000),
		al(1, 2, 10_000, 60_000, 298_000),
		al(2, 2, 56_000, 0, 702_000),
		al(2, 3, 4_000, 38_000, 0),
		al(3, 3, 16_000, 22_000, 1_000_000),
		al(3, 4, 0, 60_000, 1_000_000),
		al(3, 5, 0, 0, 1_000_000),
		al(3, 6, 0, 0, 1_000_000),
	}
	if !slices.Equal(alokasi, wantAlokasi) {
		t.Errorf("alokasi = %+v\nwant     %+v", alokasi, wantAlokasi)
	}
	cekJumlah(t, jadwal, semua, alokasi)

	t.Run("jasa bulan 5 dan 6 dihapus, pinjaman lunas", func(t *testing.T) {
		akhir, err := HitungStatus(jadwal, semua, asOf, aturanUji)
		if err != nil {
			t.Fatalf("HitungStatus: %v", err)
		}
		var dihapus int64
		for _, s := range akhir {
			dihapus += s.JasaDihapus
			if !s.IsLunas() {
				t.Errorf("angsuran %d belum lunas setelah pelunasan", s.Urutan)
			}
		}
		if dihapus != 120_000 {
			t.Errorf("jasa dihapus = %d, want 120000", dihapus)
		}
		if !IsSemuaLunas(akhir) {
			t.Error("semuaLunas = false")
		}
	})

	t.Run("pendapatan bulanan berbasis kas", func(t *testing.T) {
		type bulan struct{ jasa, denda, admin int64 }
		got := map[string]*bulan{}
		tanggal := map[int64]time.Time{}
		for _, p := range semua {
			tanggal[p.ID] = p.TanggalBayar
			k := p.TanggalBayar.Format("2006-01")
			if got[k] == nil {
				got[k] = &bulan{}
			}
			got[k].admin += p.BiayaAdmin
		}
		for _, a := range alokasi {
			k := tanggal[a.IDPembayaran].Format("2006-01")
			got[k].jasa += a.Jasa
			got[k].denda += a.Denda
		}
		want := map[string]bulan{
			"2027-01": {jasa: 120_000, denda: 82_000, admin: 0},
			"2027-02": {jasa: 120_000, denda: 76_000, admin: 80_000},
		}
		for k, w := range want {
			if g := got[k]; g == nil || *g != w {
				t.Errorf("pendapatan %s = %+v, want %+v", k, g, w)
			}
		}
	})
}

// Contoh D: pembayaran 1 dibatalkan, pembayaran 2 dihitung ulang sendirian.
func TestHitungAlokasi_PembatalanContohD(t *testing.T) {
	jadwal := jadwalC(t)
	p2 := bayar(2, tgl(2027, time.February, 12), 800_000)

	got, err := HitungAlokasi(jadwal, []model.Pembayaran{p2}, aturanUji)
	if err != nil {
		t.Fatalf("HitungAlokasi: %v", err)
	}
	want := []model.Alokasi{al(2, 1, 128_000, 60_000, 612_000)} // 64 hari terlambat
	if !slices.Equal(got, want) {
		t.Errorf("alokasi = %+v, want %+v", got, want)
	}

	status, err := HitungStatus(jadwal, []model.Pembayaran{p2}, tgl(2027, time.February, 12), aturanUji)
	if err != nil {
		t.Fatalf("HitungStatus: %v", err)
	}
	if got := status[0].SisaPokok(); got != 388_000 {
		t.Errorf("sisa pokok angsuran 1 = %d, want 388000", got)
	}
}

// Contoh E: kasus batas.
func TestHitungAlokasi_KasusBatas(t *testing.T) {
	jadwal := jadwalC(t)
	tests := []struct {
		nama       string
		pembayaran []model.Pembayaran
		want       []model.Alokasi
	}{
		{
			nama:       "bayar tepat pada jatuh tempo: tanpa denda",
			pembayaran: []model.Pembayaran{bayar(1, tgl(2026, time.December, 10), 1_060_000)},
			want:       []model.Alokasi{al(1, 1, 0, 60_000, 1_000_000)},
		},
		{
			nama:       "bayar sehari terlambat: denda 2.000",
			pembayaran: []model.Pembayaran{bayar(1, tgl(2026, time.December, 11), 1_062_000)},
			want:       []model.Alokasi{al(1, 1, 2_000, 60_000, 1_000_000)},
		},
		{
			nama:       "pembayaran lebih kecil dari denda: seluruhnya ke denda",
			pembayaran: []model.Pembayaran{bayar(1, tgl(2026, time.December, 20), 10_000)}, // denda 20.000
			want:       []model.Alokasi{al(1, 1, 10_000, 0, 0)},
		},
		{
			nama:       "bayar sebelum jatuh tempo: jasa lalu pokok, tanpa denda",
			pembayaran: []model.Pembayaran{bayar(1, tgl(2026, time.December, 1), 100_000)},
			want:       []model.Alokasi{al(1, 1, 0, 60_000, 40_000)},
		},
		{
			nama:       "bayar di muka dua angsuran sekaligus",
			pembayaran: []model.Pembayaran{bayar(1, tgl(2026, time.December, 1), 2_120_000)},
			want: []model.Alokasi{
				al(1, 1, 0, 60_000, 1_000_000),
				al(1, 2, 0, 60_000, 1_000_000),
			},
		},
		{
			nama: "denda terus bertambah selama jasa dan pokok belum lunas, walau dendanya sudah dibayar",
			pembayaran: []model.Pembayaran{
				bayar(1, tgl(2027, time.January, 15), 72_000),    // hanya melunasi denda 36 hari
				bayar(2, tgl(2027, time.January, 20), 1_070_000), // denda total 41 hari = 82.000
			},
			want: []model.Alokasi{
				al(1, 1, 72_000, 0, 0),
				al(2, 1, 10_000, 60_000, 1_000_000),
			},
		},
		{
			nama: "pembayaran pada tanggal yang sama diproses menurut ID",
			pembayaran: []model.Pembayaran{
				bayar(2, tgl(2026, time.December, 10), 40_000),
				bayar(1, tgl(2026, time.December, 10), 40_000),
			},
			want: []model.Alokasi{
				al(1, 1, 0, 40_000, 0),
				al(2, 1, 0, 20_000, 20_000),
			},
		},
		{
			nama:       "seluruh sisa kewajiban tepat dibayar sekaligus",
			pembayaran: []model.Pembayaran{bayar(1, tgl(2026, time.December, 10), 6_360_000)},
			want: []model.Alokasi{
				al(1, 1, 0, 60_000, 1_000_000), al(1, 2, 0, 60_000, 1_000_000), al(1, 3, 0, 60_000, 1_000_000),
				al(1, 4, 0, 60_000, 1_000_000), al(1, 5, 0, 60_000, 1_000_000), al(1, 6, 0, 60_000, 1_000_000),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			got, err := HitungAlokasi(jadwal, tt.pembayaran, aturanUji)
			if err != nil {
				t.Fatalf("HitungAlokasi: %v", err)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("alokasi = %+v\nwant     %+v", got, tt.want)
			}
			cekJumlah(t, jadwal, tt.pembayaran, got)
		})
	}
}

func TestHitungAlokasi_Kesalahan(t *testing.T) {
	jadwal := jadwalC(t)
	asOf := tgl(2027, time.February, 20)
	p1 := bayar(1, tgl(2027, time.January, 15), 1_500_000)
	p2 := bayar(2, tgl(2027, time.February, 12), 800_000)
	pelunasan := func(jumlah, admin int64) model.Pembayaran {
		return model.Pembayaran{ID: 3, Jenis: model.PembayaranPelunasan, TanggalBayar: asOf, Jumlah: jumlah, BiayaAdmin: admin}
	}

	tests := []struct {
		nama       string
		pembayaran []model.Pembayaran
		want       error
	}{
		{"melebihi seluruh sisa kewajiban", []model.Pembayaran{bayar(1, tgl(2026, time.December, 10), 6_360_001)}, ErrPembayaranBerlebih},
		{"jumlah nol", []model.Pembayaran{bayar(1, tgl(2026, time.December, 10), 0)}, ErrJumlahPembayaranTidakValid},
		{"jumlah negatif", []model.Pembayaran{bayar(1, tgl(2026, time.December, 10), -5)}, ErrJumlahPembayaranTidakValid},
		{"biaya administrasi pada angsuran biasa", []model.Pembayaran{{ID: 1, Jenis: model.PembayaranAngsuran, TanggalBayar: tgl(2026, time.December, 10), Jumlah: 100_000, BiayaAdmin: 1}}, ErrBiayaAdminTidakValid},
		{"jenis tidak dikenal", []model.Pembayaran{{ID: 1, Jenis: "cicilan", TanggalBayar: tgl(2026, time.December, 10), Jumlah: 100_000}}, ErrJenisTidakDikenal},
		{"pelunasan kurang satu rupiah", []model.Pembayaran{p1, p2, pelunasan(4_177_999, 80_000)}, ErrJumlahPelunasanTidakSesuai},
		{"pelunasan dengan biaya administrasi salah", []model.Pembayaran{p1, p2, pelunasan(4_178_000, 79_000)}, ErrJumlahPelunasanTidakSesuai},
		{"pembayaran setelah pelunasan", []model.Pembayaran{p1, p2, pelunasan(4_178_000, 80_000), bayar(4, tgl(2027, time.February, 25), 1_000)}, ErrPinjamanSudahLunas},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			if _, err := HitungAlokasi(jadwal, tt.pembayaran, aturanUji); !errors.Is(err, tt.want) {
				t.Errorf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestHitungPelunasan_Variasi(t *testing.T) {
	jadwal := jadwalC(t)
	tests := []struct {
		nama       string
		pembayaran []model.Pembayaran
		asOf       time.Time
		want       model.RincianPelunasan
	}{
		{
			nama: "sebelum angsuran pertama jatuh tempo: hanya jasa satu bulan",
			asOf: tgl(2026, time.November, 20),
			want: model.RincianPelunasan{SisaPokok: 6_000_000, JasaBulanBerjalan: 60_000, BiayaAdmin: 120_000, Total: 6_180_000},
		},
		{
			nama:       "jasa bulan depan sudah dibayar di muka: bergeser ke bulan sesudahnya",
			pembayaran: []model.Pembayaran{bayar(1, tgl(2026, time.December, 1), 1_060_000)},
			asOf:       tgl(2026, time.December, 5),
			want:       model.RincianPelunasan{SisaPokok: 5_000_000, JasaBulanBerjalan: 60_000, BiayaAdmin: 100_000, Total: 5_160_000},
		},
		{
			nama: "tepat pada jatuh tempo: angsuran itu sudah jatuh tempo, bukan bulan berjalan",
			asOf: tgl(2026, time.December, 10),
			want: model.RincianPelunasan{SisaPokok: 6_000_000, JasaJatuhTempo: 60_000, JasaBulanBerjalan: 60_000, BiayaAdmin: 120_000, Total: 6_240_000},
		},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			status, err := HitungStatus(jadwal, tt.pembayaran, tt.asOf, aturanUji)
			if err != nil {
				t.Fatalf("HitungStatus: %v", err)
			}
			if got := HitungPelunasan(status, tt.asOf, aturanUji.BiayaAdminBps); got != tt.want {
				t.Errorf("rincian = %+v\nwant      %+v", got, tt.want)
			}
		})
	}
}
