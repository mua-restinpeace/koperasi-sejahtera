package handler

// Model tampilan Dashboard. Handler/service yang asli cukup mengisi struct ini;
// template tidak perlu diubah. Semua uang int64 rupiah, tanggal sudah berupa teks.

type dataDashboard struct {
	Halaman
	Contoh     bool // true selama data masih dari contohDashboard() (menampilkan catatan)
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

// contohDashboard mengisi dashboard dengan angka contoh yang konsisten dengan
// spesifikasi (jatuh tempo tanggal 10, simpanan wajib Rp100.000).
// TODO: ganti dengan data dari service pinjaman/laporan bila sudah siap.
func contohDashboard() dataDashboard {
	d := dataDashboard{
		Contoh:     true,
		Periode:    "November 2026",
		TotalTelat: 3,
		TotalWajib: 41,
		Ringkasan: ringkasanDashboard{
			AnggotaAktif: 248, AnggotaBaru: 3, PinjamanAktif: 64,
			SisaPinjaman: 482_000_000, TotalSimpanan: 715_000_000,
			PendapatanJasa: 4_820_000, PendapatanDenda: 392_000,
		},
		Terlambat: []barisTerlambat{
			{1, "Andi Pratama", "KKS-0001", "PJ-2026-001", "10 Okt 2026", 1_528_571, 48},
			{3, "Siti Rahmawati", "KKS-0003", "PJ-2026-007", "10 Nov 2026", 1_250_000, 17},
			{4, "Agus Salim", "KKS-0004", "PJ-2026-012", "10 Nov 2026", 830_000, 17},
		},
		WajibBelum: []barisWajibBelum{
			{2, "Rudi Hartono", "KKS-0002", 100_000, "Nov 2026"},
			{6, "Maya Putri", "KKS-0006", 100_000, "Nov 2026"},
		},
	}
	bulan := []string{"Jun", "Jul", "Agu", "Sep", "Okt", "Nov"}
	jasa := []int64{3_900_000, 4_100_000, 4_350_000, 4_500_000, 4_700_000, 4_820_000}
	denda := []int64{310_000, 250_000, 420_000, 380_000, 450_000, 392_000}
	var maks int64 = 1
	for i := range bulan {
		maks = max(maks, jasa[i], denda[i])
	}
	for i := range bulan {
		d.Grafik = append(d.Grafik, titikGrafik{
			Bulan: bulan[i], Jasa: jasa[i], Denda: denda[i],
			JasaPct: int(jasa[i] * 100 / maks), DendaPct: int(denda[i] * 100 / maks),
		})
	}
	return d
}
