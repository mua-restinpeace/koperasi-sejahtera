package service

import (
	"errors"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

// siapSimpanan: tanggal bisnis 2026-11-11 (WIB), anggota 5 aktif bergabung 2026-03-15.
func siapSimpanan() (*SimpananService, *fakeDB, *fakeAnggota, *fakeSimpanan, *fakeAudit) {
	db := &fakeDB{}
	ag, sp, au := &fakeAnggota{}, &fakeSimpanan{}, &fakeAudit{}
	ag.tambah(model.Anggota{ID: 5, NomorAnggota: "A-005", NamaLengkap: "Budi", Status: model.StatusAnggotaAktif,
		TanggalBergabung: time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)})
	now := func() time.Time { return time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC) }
	jkt := time.FixedZone("WIB", 7*3600)
	return NewSimpananService(db, ag, sp, NewAuditService(au), 100_000, jkt, now), db, ag, sp, au
}

func pesanDari(t *testing.T, err error, kolom string) {
	t.Helper()
	var ve *ValidasiError
	if !errors.As(err, &ve) || ve.Pesan[kolom] == "" {
		t.Fatalf("err = %v, ingin ValidasiError kolom %q", err, kolom)
	}
}

func TestSimpanan_SetorWajib(t *testing.T) {
	tests := []struct {
		nama, periode string
		galat         bool
	}{
		{"bulan ini", "2026-11", false},
		{"bulan lalu", "2026-10", false},
		{"bulan gabung", "2026-03", false},
		{"bulan depan (bayar di muka)", "2026-12", true},
		{"sebelum bergabung", "2026-02", true},
		{"format salah", "11-2026", true},
		{"kosong", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.nama, func(t *testing.T) {
			s, _, _, sp, au := siapSimpanan()
			err := s.SetorWajib(ctxLogin(), 5, SetorWajibInput{Periode: tc.periode})
			if tc.galat {
				pesanDari(t, err, "periode")
				if len(sp.dibuat) != 0 || len(au.entri) != 0 {
					t.Errorf("tidak boleh ada tulisan: %d simpanan, %d audit", len(sp.dibuat), len(au.entri))
				}
				return
			}
			if err != nil || len(sp.dibuat) != 1 {
				t.Fatalf("err=%v dibuat=%d", err, len(sp.dibuat))
			}
			x := sp.dibuat[0]
			if x.Kategori != model.SimpananWajib || x.TipeMutasi != model.MutasiSetor || x.Jumlah != 100_000 ||
				x.IDAnggota != 5 || x.DibuatOleh != 7 || x.Periode == nil || x.Periode.Day() != 1 ||
				!x.TanggalTransaksi.Equal(time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)) {
				t.Errorf("simpanan = %+v", x)
			}
			if len(au.entri) != 1 || au.entri[0].Aksi != "setor_wajib" {
				t.Errorf("audit = %+v", au.entri)
			}
		})
	}
}

func TestSimpanan_SetorWajibDuplikat(t *testing.T) {
	s, db, _, sp, au := siapSimpanan()
	if err := s.SetorWajib(ctxLogin(), 5, SetorWajibInput{Periode: "2026-10"}); err != nil {
		t.Fatal(err)
	}
	err := s.SetorWajib(ctxLogin(), 5, SetorWajibInput{Periode: "2026-10"})
	pesanDari(t, err, "periode")
	if len(sp.dibuat) != 1 || len(au.entri) != 1 || db.rollback != 1 {
		t.Errorf("dibuat=%d audit=%d rollback=%d", len(sp.dibuat), len(au.entri), db.rollback)
	}
}

func TestSimpanan_SetorSukarela(t *testing.T) {
	tests := []struct {
		nama, jumlah string
		ingin        int64 // 0 = ditolak
	}{
		{"angka biasa", "250000", 250_000},
		{"titik ribuan", " 1.500.000 ", 1_500_000},
		{"nol", "0", 0},
		{"negatif", "-5", 0},
		{"bukan angka", "seratus", 0},
		{"kosong", "", 0},
		{"terlalu besar", "1000000001", 0},
	}
	for _, tc := range tests {
		t.Run(tc.nama, func(t *testing.T) {
			s, _, _, sp, _ := siapSimpanan()
			err := s.SetorSukarela(ctxLogin(), 5, SukarelaInput{Jumlah: tc.jumlah, Keterangan: " gaji "})
			if tc.ingin == 0 {
				pesanDari(t, err, "jumlah")
				if len(sp.dibuat) != 0 {
					t.Errorf("dibuat = %d", len(sp.dibuat))
				}
				return
			}
			if err != nil || len(sp.dibuat) != 1 {
				t.Fatalf("err=%v dibuat=%d", err, len(sp.dibuat))
			}
			if x := sp.dibuat[0]; x.Jumlah != tc.ingin || x.Kategori != model.SimpananSukarela || x.TipeMutasi != model.MutasiSetor || x.Keterangan != "gaji" || x.Periode != nil {
				t.Errorf("simpanan = %+v", x)
			}
		})
	}
}

func TestSimpanan_TarikSukarela(t *testing.T) {
	tests := []struct {
		nama   string
		saldo  int64
		jumlah string
		boleh  bool
	}{
		{"di bawah saldo", 300_000, "100000", true},
		{"tepat saldo", 300_000, "300000", true},
		{"melebihi saldo", 300_000, "300001", false},
		{"saldo kosong", 0, "1", false},
	}
	for _, tc := range tests {
		t.Run(tc.nama, func(t *testing.T) {
			s, _, ag, sp, au := siapSimpanan()
			sp.saldo = model.SaldoSimpanan{Pokok: 500_000, Wajib: 900_000, Sukarela: tc.saldo} // pokok/wajib tidak boleh ikut dihitung
			err := s.TarikSukarela(ctxLogin(), 5, SukarelaInput{Jumlah: tc.jumlah})
			if ag.kunci != 1 {
				t.Errorf("baris anggota harus dikunci sekali, kunci=%d", ag.kunci)
			}
			if !tc.boleh {
				pesanDari(t, err, "jumlah")
				if len(sp.dibuat) != 0 || len(au.entri) != 0 {
					t.Errorf("tidak boleh ada tulisan")
				}
				return
			}
			if err != nil || len(sp.dibuat) != 1 || sp.dibuat[0].TipeMutasi != model.MutasiTarik || au.entri[0].Aksi != "tarik_sukarela" {
				t.Fatalf("err=%v dibuat=%+v audit=%+v", err, sp.dibuat, au.entri)
			}
		})
	}
}

func TestSimpanan_AnggotaTidakBolehMencatat(t *testing.T) {
	s, _, ag, sp, _ := siapSimpanan()
	if err := s.SetorSukarela(ctxLogin(), 99, SukarelaInput{Jumlah: "1000"}); !errors.Is(err, ErrAnggotaTidakDitemukan) {
		t.Errorf("anggota tak ada: err = %v", err)
	}
	a := ag.data[5]
	a.Status = model.StatusAnggotaKeluar
	ag.tambah(a)
	if err := s.SetorWajib(ctxLogin(), 5, SetorWajibInput{Periode: "2026-11"}); !errors.Is(err, ErrAnggotaSudahKeluar) {
		t.Errorf("anggota keluar: err = %v", err)
	}
	if len(sp.dibuat) != 0 {
		t.Errorf("dibuat = %d", len(sp.dibuat))
	}
}

func TestSimpanan_ButuhLogin(t *testing.T) {
	s, _, _, _, _ := siapSimpanan()
	ctx := t.Context()
	if err := s.SetorWajib(ctx, 5, SetorWajibInput{Periode: "2026-11"}); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("SetorWajib: %v", err)
	}
	if _, err := s.Batalkan(ctx, 1, "x"); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("Batalkan: %v", err)
	}
	if _, err := s.Riwayat(ctx, 5); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("Riwayat: %v", err)
	}
}

func TestSimpanan_Batalkan(t *testing.T) {
	waktu := time.Now()
	tests := []struct {
		nama   string
		trx    model.TransaksiSimpanan
		saldo  model.SaldoSimpanan
		alasan string
		galat  error // nil = berhasil
		valid  string
	}{
		{"setor wajib", model.TransaksiSimpanan{Kategori: "wajib", TipeMutasi: "setor", Jumlah: 100_000}, model.SaldoSimpanan{Wajib: 300_000}, "salah periode", nil, ""},
		{"tarik sukarela", model.TransaksiSimpanan{Kategori: "sukarela", TipeMutasi: "tarik", Jumlah: 50_000}, model.SaldoSimpanan{}, "salah ketik", nil, ""},
		{"setor sukarela yang sudah ditarik", model.TransaksiSimpanan{Kategori: "sukarela", TipeMutasi: "setor", Jumlah: 200_000}, model.SaldoSimpanan{Sukarela: 100_000}, "salah ketik", nil, "jumlah"},
		{"setor sukarela, saldo cukup", model.TransaksiSimpanan{Kategori: "sukarela", TipeMutasi: "setor", Jumlah: 200_000}, model.SaldoSimpanan{Sukarela: 200_000}, "salah ketik", nil, ""},
		{"simpanan pokok", model.TransaksiSimpanan{Kategori: "pokok", TipeMutasi: "setor", Jumlah: 500_000}, model.SaldoSimpanan{Pokok: 500_000}, "x", ErrPokokTidakBisaDibatalkan, ""},
		{"sudah dibatalkan", model.TransaksiSimpanan{Kategori: "wajib", TipeMutasi: "setor", Jumlah: 100_000, DibatalkanPada: &waktu}, model.SaldoSimpanan{}, "x", ErrSudahDibatalkan, ""},
		{"alasan kosong", model.TransaksiSimpanan{Kategori: "wajib", TipeMutasi: "setor", Jumlah: 100_000}, model.SaldoSimpanan{Wajib: 100_000}, "  ", nil, "alasan"},
	}
	for _, tc := range tests {
		t.Run(tc.nama, func(t *testing.T) {
			s, db, _, sp, au := siapSimpanan()
			tc.trx.ID, tc.trx.IDAnggota = 9, 5
			sp.tambah(tc.trx)
			sp.saldo = tc.saldo

			idAng, err := s.Batalkan(ctxLogin(), 9, tc.alasan)
			berhasil := tc.galat == nil && tc.valid == ""
			switch {
			case tc.valid != "":
				pesanDari(t, err, tc.valid)
			case tc.galat != nil:
				if !errors.Is(err, tc.galat) {
					t.Fatalf("err = %v, ingin %v", err, tc.galat)
				}
			case err != nil:
				t.Fatalf("err = %v", err)
			}

			if berhasil {
				if idAng != 5 || len(sp.batal) != 1 || sp.batal[0] != 9 || sp.alasan != tc.alasan {
					t.Errorf("idAng=%d batal=%v alasan=%q", idAng, sp.batal, sp.alasan)
				}
				if len(au.entri) != 1 || au.entri[0].Aksi != "batal_simpanan" || au.entri[0].IDEntitas != 9 {
					t.Errorf("audit = %+v", au.entri)
				}
				return
			}
			if len(sp.batal) != 0 || len(au.entri) != 0 {
				t.Errorf("tidak boleh ada tulisan: batal=%v audit=%d", sp.batal, len(au.entri))
			}
			if tc.valid == "alasan" {
				return // ditolak sebelum transaksi dimulai
			}
			if db.rollback != 1 {
				t.Errorf("rollback = %d", db.rollback)
			}
		})
	}
}

func TestSimpanan_BatalkanKasusLain(t *testing.T) {
	s, _, ag, sp, _ := siapSimpanan()
	if _, err := s.Batalkan(ctxLogin(), 404, "x"); !errors.Is(err, ErrTransaksiTidakDitemukan) {
		t.Errorf("transaksi tak ada: %v", err)
	}

	sp.tambah(model.TransaksiSimpanan{ID: 9, IDAnggota: 5, Kategori: "sukarela", TipeMutasi: "tarik", Jumlah: 1})
	if _, err := s.Batalkan(ctxLogin(), 9, "x"); err != nil || ag.kunci != 1 || sp.kunciTrx != 2 {
		t.Errorf("kunci anggota=%d transaksi=%d err=%v", ag.kunci, sp.kunciTrx, err)
	}

	a := ag.data[5]
	a.Status = model.StatusAnggotaKeluar
	ag.tambah(a)
	if _, err := s.Batalkan(ctxLogin(), 9, "x"); !errors.Is(err, ErrAnggotaSudahKeluar) {
		t.Errorf("anggota keluar: %v", err)
	}
}

func TestSimpanan_Riwayat(t *testing.T) {
	s, _, _, sp, _ := siapSimpanan()
	sp.tambah(model.TransaksiSimpanan{ID: 1, IDAnggota: 5})
	sp.tambah(model.TransaksiSimpanan{ID: 2, IDAnggota: 6})
	sp.saldo = model.SaldoSimpanan{Pokok: 500_000}

	r, err := s.Riwayat(ctxLogin(), 5)
	if err != nil || len(r.Transaksi) != 1 || r.Saldo.Pokok != 500_000 || r.Wajib != 100_000 || r.Anggota.ID != 5 {
		t.Fatalf("riwayat = %+v err=%v", r, err)
	}
	if _, err := s.Riwayat(ctxLogin(), 99); !errors.Is(err, ErrAnggotaTidakDitemukan) {
		t.Errorf("err = %v", err)
	}
}
