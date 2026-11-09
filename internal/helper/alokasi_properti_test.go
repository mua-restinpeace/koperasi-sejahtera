package helper

import (
	"cmp"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"

	"math/rand"
	"slices"
	"testing"
	"time"
)

// TestProperti_Alokasi memeriksa sifat-sifat yang harus selalu berlaku pada ribuan
// skenario acak (pinjaman, tenor, tanggal, dan jumlah pembayaran berbeda-beda).
// Seed tetap, sehingga kegagalan selalu dapat diulang.
func TestProperti_Alokasi(t *testing.T) {
	rng := rand.New(rand.NewSource(20261107))
	valid, pelunasanDicoba := 0, 0

	for iter := 0; iter < 3000; iter++ {
		tenor := 6 + rng.Intn(19)
		jumlah := int64(1+rng.Intn(24))*1_000_000 + int64(rng.Intn(1000))*100
		pencairan := tgl(2026, time.January, 1).AddDate(0, 0, rng.Intn(365))
		jadwal, err := BuatJadwal(jumlah, tenor, 100, pencairan)
		if err != nil {
			t.Fatalf("iter %d: BuatJadwal: %v", iter, err)
		}

		var pembayaran []model.Pembayaran
		for i, n := 0, 1+rng.Intn(6); i < n; i++ {
			hari := rng.Intn((tenor + 3) * 30)
			pembayaran = append(pembayaran, bayar(int64(i+1), pencairan.AddDate(0, 0, hari), int64(1+rng.Intn(40))*50_000))
		}

		alokasi, err := HitungAlokasi(jadwal, pembayaran, aturanUji)
		if err != nil {
			continue // pembayaran melebihi sisa kewajiban: skenario tidak sah, dilewati
		}
		valid++
		gagal := func(format string, args ...any) {
			t.Helper()
			t.Fatalf("iter %d (jumlah %d, tenor %d, pencairan %s, pembayaran %+v): "+format,
				append([]any{iter, jumlah, tenor, pencairan.Format("2006-01-02"), pembayaran}, args...)...)
		}

		// 1. Uang yang masuk = uang yang dialokasikan; tidak ada nilai negatif; tidak ada angsuran dibayar berlebih.
		cekJumlah(t, jadwal, pembayaran, alokasi)
		for _, a := range alokasi {
			if a.Denda < 0 || a.Jasa < 0 || a.Pokok < 0 || a.Denda+a.Jasa+a.Pokok == 0 {
				gagal("alokasi tidak wajar: %+v", a)
			}
		}

		// 2. Urutan masukan tidak berpengaruh.
		acak := slices.Clone(pembayaran)
		rng.Shuffle(len(acak), func(i, j int) { acak[i], acak[j] = acak[j], acak[i] })
		if dua, err := HitungAlokasi(jadwal, acak, aturanUji); err != nil || !slices.Equal(dua, alokasi) {
			gagal("hasil berbeda setelah masukan diacak: %+v (err %v)", dua, err)
		}

		// 3. Setiap prefix pembayaran menghasilkan awalan dari alokasi penuh, dan
		//    angsuran terlama dilunasi lebih dulu: bila pembayaran menyentuh angsuran k,
		//    seluruh angsuran sebelum k sudah lunas.
		urut := slices.Clone(pembayaran)
		slices.SortStableFunc(urut, func(x, y model.Pembayaran) int {
			if c := tanggalSaja(x.TanggalBayar).Compare(tanggalSaja(y.TanggalBayar)); c != 0 {
				return c
			}
			return cmp.Compare(x.ID, y.ID)
		})
		for i := range urut {
			status, awal, err := replay(jadwal, urut[:i+1], aturanUji)
			if err != nil || len(awal) > len(alokasi) || !slices.Equal(awal, alokasi[:len(awal)]) {
				gagal("prefix %d bukan awalan alokasi penuh (err %v)", i+1, err)
			}
			terjauh := 0
			for _, a := range awal {
				if a.IDPembayaran == urut[i].ID {
					terjauh = max(terjauh, a.Urutan)
				}
			}
			for j := 0; j < terjauh-1; j++ {
				if !status[j].IsLunas() {
					gagal("pembayaran %d menyentuh angsuran %d padahal angsuran %d belum lunas", urut[i].ID, terjauh, j+1)
				}
			}
		}

		// 4. Membatalkan pembayaran mana pun tidak pernah membuat sisanya ditolak
		//    (kewajiban hanya bertambah), sehingga hitung ulang setelah pembatalan selalu berhasil.
		for k := range pembayaran {
			tanpa := slices.Delete(slices.Clone(pembayaran), k, k+1)
			if _, err := HitungAlokasi(jadwal, tanpa, aturanUji); err != nil {
				gagal("hitung ulang tanpa pembayaran %d gagal: %v", pembayaran[k].ID, err)
			}
		}

		// 5. Pelunasan dipercepat pada tanggal setelah pembayaran terakhir selalu sah,
		//    melunasi semuanya, tidak pernah lebih mahal daripada melunasi seluruh
		//    tagihan penuh ditambah biaya administrasi, dan teralokasi tepat.
		terakhir := tanggalSaja(urut[len(urut)-1].TanggalBayar)
		d := terakhir.AddDate(0, 0, rng.Intn(60))
		status, err := HitungStatus(jadwal, pembayaran, d, aturanUji)
		if err != nil {
			gagal("HitungStatus: %v", err)
		}
		if IsSemuaLunas(status) {
			continue
		}
		pelunasanDicoba++
		r := HitungPelunasan(status, d, aturanUji.BiayaAdminBps)
		var penuh int64
		for _, s := range status {
			penuh += s.DendaTerutang + s.SisaJasa() + s.SisaPokok()
		}
		if r.Total > penuh+r.BiayaAdmin {
			gagal("pelunasan Rp%d lebih mahal dari tagihan penuh Rp%d + administrasi Rp%d", r.Total, penuh, r.BiayaAdmin)
		}
		pl := model.Pembayaran{ID: 99, Jenis: model.PembayaranPelunasan, TanggalBayar: d, Jumlah: r.Total, BiayaAdmin: r.BiayaAdmin}
		dengan := append(slices.Clone(pembayaran), pl)
		semuaAlokasi, err := HitungAlokasi(jadwal, dengan, aturanUji)
		if err != nil {
			gagal("pelunasan %+v ditolak: %v", pl, err)
		}
		cekJumlah(t, jadwal, dengan, semuaAlokasi)
		akhir, err := HitungStatus(jadwal, dengan, d, aturanUji)
		if err != nil || !IsSemuaLunas(akhir) {
			gagal("setelah pelunasan belum lunas (err %v)", err)
		}
		for _, s := range akhir {
			if s.DendaTerutang != 0 {
				gagal("angsuran %d masih punya denda terutang Rp%d setelah pelunasan", s.Urutan, s.DendaTerutang)
			}
		}
	}

	t.Logf("skenario sah: %d dari 3000; pelunasan diuji: %d", valid, pelunasanDicoba)
	if valid < 1000 || pelunasanDicoba < 500 {
		t.Errorf("terlalu sedikit skenario yang sah (%d) atau pelunasan (%d); sebaran jumlah acak perlu disesuaikan", valid, pelunasanDicoba)
	}
}
