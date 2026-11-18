package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

func siapAnggota() (*AnggotaService, *fakeDB, *fakeAnggota, *fakeSimpanan, *fakeAudit) {
	s, db, ag, sp, au, _ := siapAnggotaLengkap()
	return s, db, ag, sp, au
}

// siapAnggotaLengkap sama dengan siapAnggota, ditambah tiruan cek pinjaman aktif.
func siapAnggotaLengkap() (*AnggotaService, *fakeDB, *fakeAnggota, *fakeSimpanan, *fakeAudit, *fakePinjamanAktif) {
	db := &fakeDB{}
	ag, sp, au, pj := &fakeAnggota{}, &fakeSimpanan{}, &fakeAudit{}, &fakePinjamanAktif{}
	// 2026-11-10 20:00 UTC = 11 November 03:00 WIB: tanggal bisnis harus 11 November.
	now := func() time.Time { return time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC) }
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	if jkt == nil {
		jkt = time.FixedZone("WIB", 7*3600)
	}
	return NewAnggotaService(db, ag, sp, pj, NewAuditService(au), 500_000, jkt, now), db, ag, sp, au, pj
}

func ctxLogin() context.Context {
	return DenganIdentitas(context.Background(), Identitas{IDPengguna: 7, Username: "ani"})
}

func TestAnggota_DaftarkanBerhasil(t *testing.T) {
	s, db, ag, sp, au := siapAnggota()

	id, err := s.Register(ctxLogin(), DaftarAnggotaInput{NomorAnggota: " A-001 ", NamaLengkap: "Siti Aminah", NoTelepon: "0812"})
	if err != nil || id != 1 {
		t.Fatalf("id=%d err=%v", id, err)
	}
	if db.tx != 1 || db.rollback != 0 {
		t.Errorf("transaksi=%d rollback=%d", db.tx, db.rollback)
	}
	if a := ag.dibuat[0]; a.NomorAnggota != "A-001" || a.TanggalBergabung != time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) {
		t.Errorf("anggota = %+v (tanggal bisnis harus 11 Nov, zona WIB)", a)
	}
	if len(sp.dibuat) != 1 {
		t.Fatalf("setoran pokok = %d", len(sp.dibuat))
	}
	if p := sp.dibuat[0]; p.Kategori != model.SimpananPokok || p.TipeMutasi != model.MutasiSetor || p.Jumlah != 500_000 || p.DibuatOleh != 7 || p.IDAnggota != 1 {
		t.Errorf("simpanan = %+v", p)
	}
	if len(au.entri) != 1 || au.entri[0].Aksi != "daftar_anggota" || au.entri[0].IDPengguna != 7 || au.entri[0].IDEntitas != 1 {
		t.Errorf("audit = %+v", au.entri)
	}
}

func TestAnggota_NomorSudahAda(t *testing.T) {
	s, db, _, sp, au := siapAnggota()
	in := DaftarAnggotaInput{NomorAnggota: "A-001", NamaLengkap: "Siti"}
	if _, err := s.Register(ctxLogin(), in); err != nil {
		t.Fatal(err)
	}
	sp.dibuat, au.entri = nil, nil

	_, err := s.Register(ctxLogin(), in)
	var ve *ValidasiError
	if !errors.As(err, &ve) || ve.Pesan["nomor_anggota"] == "" {
		t.Fatalf("err = %v, want ValidasiError nomor_anggota", err)
	}
	if db.rollback != 1 || len(sp.dibuat) != 0 || len(au.entri) != 0 {
		t.Errorf("rollback=%d simpanan=%d audit=%d", db.rollback, len(sp.dibuat), len(au.entri))
	}
}

func TestAnggota_AuditGagalMembatalkanTransaksi(t *testing.T) {
	s, db, _, _, au := siapAnggota()
	au.err = errors.New("disk penuh")
	_, err := s.Register(ctxLogin(), DaftarAnggotaInput{NomorAnggota: "A-002", NamaLengkap: "Budi"})
	if err == nil || db.rollback != 1 {
		t.Errorf("err=%v rollback=%d; audit gagal harus membatalkan transaksi", err, db.rollback)
	}
}

func TestAnggota_Validasi(t *testing.T) {
	panjang := func(n int) string {
		b := make([]rune, n)
		for i := range b {
			b[i] = 'x'
		}
		return string(b)
	}
	tests := []struct {
		nama  string
		in    DaftarAnggotaInput
		kolom string
	}{
		{"nomor kosong", DaftarAnggotaInput{NamaLengkap: "A"}, "nomor_anggota"},
		{"nomor terlalu panjang", DaftarAnggotaInput{NomorAnggota: panjang(21), NamaLengkap: "A"}, "nomor_anggota"},
		{"nama kosong", DaftarAnggotaInput{NomorAnggota: "1", NamaLengkap: "  "}, "nama_lengkap"},
		{"nama terlalu panjang", DaftarAnggotaInput{NomorAnggota: "1", NamaLengkap: panjang(101)}, "nama_lengkap"},
		{"telepon terlalu panjang", DaftarAnggotaInput{NomorAnggota: "1", NamaLengkap: "A", NoTelepon: panjang(21)}, "no_telepon"},
		{"tanggal salah format", DaftarAnggotaInput{NomorAnggota: "1", NamaLengkap: "A", TanggalBergabung: "11/11/2026"}, "tanggal_bergabung"},
		{"tanggal masa depan", DaftarAnggotaInput{NomorAnggota: "1", NamaLengkap: "A", TanggalBergabung: "2026-11-12"}, "tanggal_bergabung"},
	}
	for _, tt := range tests {
		t.Run(tt.nama, func(t *testing.T) {
			s, db, _, _, _ := siapAnggota()
			_, err := s.Register(ctxLogin(), tt.in)
			var ve *ValidasiError
			if !errors.As(err, &ve) || ve.Pesan[tt.kolom] == "" {
				t.Fatalf("err = %v, want ValidasiError kolom %q", err, tt.kolom)
			}
			if db.tx != 0 {
				t.Error("validasi gagal tidak boleh membuka transaksi")
			}
		})
	}
}

func TestAnggota_TanggalHariIniBolehDanBelumLogin(t *testing.T) {
	s, _, ag, _, _ := siapAnggota()
	if _, err := s.Register(ctxLogin(), DaftarAnggotaInput{NomorAnggota: "1", NamaLengkap: "A", TanggalBergabung: "2026-11-11"}); err != nil {
		t.Fatalf("tanggal hari ini (WIB) harus boleh: %v", err)
	}
	if len(ag.dibuat) != 1 {
		t.Fatal("anggota tidak tersimpan")
	}
	if _, err := s.Register(context.Background(), DaftarAnggotaInput{NomorAnggota: "2", NamaLengkap: "B"}); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("tanpa login: err = %v", err)
	}
}

func TestAnggota_GetListFilteredBerhasil(t *testing.T) {
	s, _, ag, _, _ := siapAnggota()

	expected := []model.Anggota{
		{
			ID:           1,
			NomorAnggota: "A-001",
			NamaLengkap:  "Siti Aminah",
		},
		{
			ID:           2,
			NomorAnggota: "A-002",
			NamaLengkap:  "Budi Santoso",
		},
	}

	ag.hasilFilter = expected
	ag.totalFilter = 15

	result, err := s.GetListFiltered(
		ctxLogin(),
		"A-00",
		"aktif",
		2,
		10,
	)

	if err != nil {
		t.Fatalf("GetListFiltered() error = %v", err)
	}

	if len(result.Data) != len(expected) {
		t.Fatalf("jumlah data = %d, want %d", len(result.Data), len(expected))
	}

	if result.Total != 15 {
		t.Errorf("Total = %d, want 15", result.Total)
	}

	if result.Page != 2 {
		t.Errorf("Page = %d, want 2", result.Page)
	}

	if result.PageSize != 10 {
		t.Errorf("PageSize = %d, want 10", result.PageSize)
	}

	for i, anggota := range expected {
		if result.Data[i].NomorAnggota != anggota.NomorAnggota {
			t.Errorf(
				"Data[%d].NomorAnggota = %q, want %q",
				i,
				result.Data[i].NomorAnggota,
				anggota.NomorAnggota,
			)
		}
	}

	// Pastikan semua parameter diteruskan ke repository.
	if ag.cariFilter != "A-00" {
		t.Errorf("cari = %q, want A-00", ag.cariFilter)
	}

	if ag.statusFilter != "aktif" {
		t.Errorf("status = %q, want aktif", ag.statusFilter)
	}

	if ag.pageFilter != 2 {
		t.Errorf("page = %d, want 2", ag.pageFilter)
	}

	if ag.limitFilter != 10 {
		t.Errorf("limit = %d, want 10", ag.limitFilter)
	}
}

func TestAnggota_GetListFilteredKosong(t *testing.T) {
	s, _, ag, _, _ := siapAnggota()

	ag.hasilFilter = []model.Anggota{}
	ag.totalFilter = 0

	result, err := s.GetListFiltered(
		ctxLogin(),
		"",
		"",
		1,
		10,
	)

	if err != nil {
		t.Fatalf("GetListFiltered() error = %v", err)
	}

	if len(result.Data) != 0 {
		t.Errorf("jumlah data = %d, want 0", len(result.Data))
	}

	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}

	if result.Page != 1 {
		t.Errorf("Page = %d, want 1", result.Page)
	}

	if result.PageSize != 10 {
		t.Errorf("PageSize = %d, want 10", result.PageSize)
	}
}

func TestAnggota_GetListFilteredRepositoryGagal(t *testing.T) {
	s, _, ag, _, _ := siapAnggota()

	expectedErr := errors.New("database connection failed")
	ag.errFilter = expectedErr

	result, err := s.GetListFiltered(
		ctxLogin(),
		"A-001",
		"aktif",
		1,
		10,
	)

	if err == nil {
		t.Fatal("GetListFiltered() seharusnya mengembalikan error")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("error = %v, want %v", err, expectedErr)
	}

	if len(result.Data) != 0 {
		t.Errorf("Data seharusnya kosong ketika repository gagal")
	}

	if result.Total != 0 {
		t.Errorf("Total = %d, want 0", result.Total)
	}
}

func TestAnggota_GetListFilteredButuhLogin(t *testing.T) {
	s, _, _, _, _ := siapAnggota()

	if _, err := s.GetListFiltered(context.Background(), "", "", 1, 10); !errors.Is(err, ErrBelumLogin) {
		t.Errorf("tanpa login: err = %v, want ErrBelumLogin", err)
	}
}

// anggotaUji adalah anggota aktif dengan ID 5 yang dipakai tes detail, ubah, dan keluar.
func anggotaUji() model.Anggota {
	return model.Anggota{
		ID:               5,
		NomorAnggota:     "A-005",
		NamaLengkap:      "Siti Aminah",
		NoTelepon:        "0812",
		TanggalBergabung: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		Status:           model.StatusAnggotaAktif,
	}
}

func TestAnggota_Detail(t *testing.T) {
	t.Run("berhasil", func(t *testing.T) {
		s, _, ag, sp, _, pj := siapAnggotaLengkap()
		ag.tambah(anggotaUji())
		sp.saldo = model.SaldoSimpanan{Pokok: 500_000, Wajib: 1_200_000, Sukarela: 250_000}
		pj.aktif = true

		d, err := s.GetAnggota(ctxLogin(), 5)
		if err != nil {
			t.Fatalf("GetAnggota: %v", err)
		}
		if d.Anggota.NomorAnggota != "A-005" || d.Saldo.Total() != 1_950_000 || !d.PinjamanAktif {
			t.Errorf("GetAnggota = %+v", d)
		}
	})

	t.Run("anggota tidak ada", func(t *testing.T) {
		s, _, _, _, _ := siapAnggota()
		if _, err := s.GetAnggota(ctxLogin(), 99); !errors.Is(err, ErrAnggotaTidakDitemukan) {
			t.Errorf("err = %v, want ErrAnggotaTidakDitemukan", err)
		}
	})

	t.Run("butuh login", func(t *testing.T) {
		s, _, _, _, _ := siapAnggota()
		if _, err := s.GetAnggota(context.Background(), 5); !errors.Is(err, ErrBelumLogin) {
			t.Errorf("err = %v, want ErrBelumLogin", err)
		}
		if _, err := s.GetByID(context.Background(), 5); !errors.Is(err, ErrBelumLogin) {
			t.Errorf("GetByID: err = %v, want ErrBelumLogin", err)
		}
	})
}

func TestAnggota_Update(t *testing.T) {
	t.Run("berhasil: tersimpan, baris dikunci, audit mencatat sebelum dan sesudah", func(t *testing.T) {
		s, db, ag, _, au := siapAnggota()
		ag.tambah(anggotaUji())

		err := s.Update(ctxLogin(), 5, UpdateAnggotaInput{NamaLengkap: "  Siti Aminah Putri ", NoTelepon: "0813"})
		if err != nil {
			t.Fatalf("Update: %v", err)
		}
		if len(ag.ubah) != 1 || ag.ubah[0] != (ubahProfil{5, "Siti Aminah Putri", "0813"}) {
			t.Errorf("ubah = %+v", ag.ubah)
		}
		if ag.kunci != 1 || db.tx != 1 || db.rollback != 0 {
			t.Errorf("kunci=%d tx=%d rollback=%d; baris harus dikunci dalam satu transaksi", ag.kunci, db.tx, db.rollback)
		}
		if len(au.entri) != 1 || au.entri[0].Aksi != "ubah_anggota" || au.entri[0].IDEntitas != 5 || au.entri[0].IDPengguna != 7 {
			t.Fatalf("audit = %+v", au.entri)
		}
		detail := string(au.entri[0].Detail)
		for _, want := range []string{`"dari":"Siti Aminah"`, `"menjadi":"Siti Aminah Putri"`, `"dari":"0812"`, `"menjadi":"0813"`} {
			if !strings.Contains(detail, want) {
				t.Errorf("detail audit %s tidak memuat %s", detail, want)
			}
		}
	})

	t.Run("hanya kolom yang berubah dicatat di audit", func(t *testing.T) {
		s, _, ag, _, au := siapAnggota()
		ag.tambah(anggotaUji())

		if err := s.Update(ctxLogin(), 5, UpdateAnggotaInput{NamaLengkap: "Siti Aminah", NoTelepon: ""}); err != nil {
			t.Fatal(err)
		}
		detail := string(au.entri[0].Detail)
		if strings.Contains(detail, "nama_lengkap") || !strings.Contains(detail, "no_telepon") {
			t.Errorf("detail audit = %s; hanya no_telepon yang berubah", detail)
		}
	})

	t.Run("tidak ada perubahan: tidak menulis apa pun", func(t *testing.T) {
		s, _, ag, _, au := siapAnggota()
		ag.tambah(anggotaUji())

		if err := s.Update(ctxLogin(), 5, UpdateAnggotaInput{NamaLengkap: " Siti Aminah ", NoTelepon: "0812"}); err != nil {
			t.Fatal(err)
		}
		if len(ag.ubah) != 0 || len(au.entri) != 0 {
			t.Errorf("ubah=%d audit=%d, want 0 dan 0", len(ag.ubah), len(au.entri))
		}
	})

	t.Run("validasi gagal: tidak membuka transaksi", func(t *testing.T) {
		tests := []struct {
			nama  string
			in    UpdateAnggotaInput
			kolom string
		}{
			{"nama kosong", UpdateAnggotaInput{NamaLengkap: "  "}, "nama_lengkap"},
			{"nama terlalu panjang", UpdateAnggotaInput{NamaLengkap: strings.Repeat("x", 101)}, "nama_lengkap"},
			{"telepon terlalu panjang", UpdateAnggotaInput{NamaLengkap: "A", NoTelepon: strings.Repeat("1", 21)}, "no_telepon"},
		}
		for _, tt := range tests {
			t.Run(tt.nama, func(t *testing.T) {
				s, db, ag, _, _ := siapAnggota()
				ag.tambah(anggotaUji())

				err := s.Update(ctxLogin(), 5, tt.in)
				var ve *ValidasiError
				if !errors.As(err, &ve) || ve.Pesan[tt.kolom] == "" {
					t.Fatalf("err = %v, want ValidasiError kolom %q", err, tt.kolom)
				}
				if db.tx != 0 {
					t.Error("validasi gagal tidak boleh membuka transaksi")
				}
			})
		}
	})

	t.Run("anggota tidak ada", func(t *testing.T) {
		s, _, _, _, _ := siapAnggota()
		err := s.Update(ctxLogin(), 99, UpdateAnggotaInput{NamaLengkap: "A"})
		if !errors.Is(err, ErrAnggotaTidakDitemukan) {
			t.Errorf("err = %v, want ErrAnggotaTidakDitemukan", err)
		}
	})

	t.Run("audit gagal membatalkan transaksi", func(t *testing.T) {
		s, db, ag, _, au := siapAnggota()
		ag.tambah(anggotaUji())
		au.err = errors.New("disk penuh")

		err := s.Update(ctxLogin(), 5, UpdateAnggotaInput{NamaLengkap: "Nama Baru"})
		if err == nil || db.rollback != 1 {
			t.Errorf("err=%v rollback=%d; audit gagal harus membatalkan transaksi", err, db.rollback)
		}
	})

	t.Run("butuh login", func(t *testing.T) {
		s, _, _, _, _ := siapAnggota()
		if err := s.Update(context.Background(), 5, UpdateAnggotaInput{NamaLengkap: "A"}); !errors.Is(err, ErrBelumLogin) {
			t.Errorf("err = %v, want ErrBelumLogin", err)
		}
	})
}

func TestAnggota_PertinjauResign(t *testing.T) {
	t.Run("boleh keluar: saldo ditampilkan, tanpa penghalang", func(t *testing.T) {
		s, db, ag, sp, _ := siapAnggota()
		ag.tambah(anggotaUji())
		sp.saldo = model.SaldoSimpanan{Pokok: 500_000, Wajib: 300_000}

		r, err := s.PertinjauResign(ctxLogin(), 5)
		if err != nil || r.Penghalang != nil {
			t.Fatalf("err=%v penghalang=%v", err, r.Penghalang)
		}
		if r.Saldo.Total() != 800_000 {
			t.Errorf("saldo = %+v", r.Saldo)
		}
		if db.tx != 0 || len(ag.keluar) != 0 {
			t.Error("pratinjau tidak boleh menulis apa pun")
		}
	})

	t.Run("masih ada pinjaman aktif", func(t *testing.T) {
		s, _, ag, _, _, pj := siapAnggotaLengkap()
		ag.tambah(anggotaUji())
		pj.aktif = true

		r, err := s.PertinjauResign(ctxLogin(), 5)
		if err != nil || !errors.Is(r.Penghalang, ErrMasihAdaPinjamanAktif) {
			t.Errorf("err=%v penghalang=%v, want ErrMasihAdaPinjamanAktif", err, r.Penghalang)
		}
	})

	t.Run("sudah keluar", func(t *testing.T) {
		s, _, ag, _, _ := siapAnggota()
		a := anggotaUji()
		a.Status = model.StatusAnggotaKeluar
		ag.tambah(a)

		r, err := s.PertinjauResign(ctxLogin(), 5)
		if err != nil || !errors.Is(r.Penghalang, ErrAnggotaSudahKeluar) {
			t.Errorf("err=%v penghalang=%v, want ErrAnggotaSudahKeluar", err, r.Penghalang)
		}
	})

	t.Run("anggota tidak ada", func(t *testing.T) {
		s, _, _, _, _ := siapAnggota()
		if _, err := s.PertinjauResign(ctxLogin(), 99); !errors.Is(err, ErrAnggotaTidakDitemukan) {
			t.Errorf("err = %v, want ErrAnggotaTidakDitemukan", err)
		}
	})
}

func TestAnggota_Keluarkan(t *testing.T) {
	t.Run("berhasil: penarikan tiap kategori, status keluar, audit, satu transaksi", func(t *testing.T) {
		s, db, ag, sp, au := siapAnggota()
		ag.tambah(anggotaUji())
		sp.saldo = model.SaldoSimpanan{Pokok: 500_000, Wajib: 1_200_000, Sukarela: 250_000}

		saldo, err := s.Resign(ctxLogin(), 5)
		if err != nil {
			t.Fatalf("Resign: %v", err)
		}
		if saldo.Total() != 1_950_000 {
			t.Errorf("saldo dikembalikan = %+v", saldo)
		}
		if db.tx != 1 || db.rollback != 0 || ag.kunci != 1 {
			t.Errorf("tx=%d rollback=%d kunci=%d; satu transaksi dengan baris terkunci", db.tx, db.rollback, ag.kunci)
		}

		want := map[string]int64{model.SimpananPokok: 500_000, model.SimpananWajib: 1_200_000, model.SimpananSukarela: 250_000}
		if len(sp.dibuat) != 3 {
			t.Fatalf("penarikan = %d, want 3", len(sp.dibuat))
		}
		for _, p := range sp.dibuat {
			if p.TipeMutasi != model.MutasiTarik || p.IDAnggota != 5 || p.DibuatOleh != 7 || p.Periode != nil || p.Jumlah != want[p.Kategori] {
				t.Errorf("penarikan tidak sesuai: %+v", p)
			}
			if p.TanggalTransaksi != time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) {
				t.Errorf("tanggal = %v, want 2026-11-11 (tanggal bisnis WIB)", p.TanggalTransaksi)
			}
		}

		if len(ag.keluar) != 1 || ag.keluar[0].id != 5 || ag.keluar[0].tanggal != time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) {
			t.Errorf("keluar = %+v", ag.keluar)
		}
		if len(au.entri) != 1 || au.entri[0].Aksi != "keluar_anggota" || au.entri[0].IDEntitas != 5 {
			t.Fatalf("audit = %+v", au.entri)
		}
		if detail := string(au.entri[0].Detail); !strings.Contains(detail, `"total_dikembalikan":1950000`) {
			t.Errorf("detail audit = %s", detail)
		}
	})

	t.Run("kategori kosong dilewati", func(t *testing.T) {
		s, _, ag, sp, _ := siapAnggota()
		ag.tambah(anggotaUji())
		sp.saldo = model.SaldoSimpanan{Pokok: 500_000}

		if _, err := s.Resign(ctxLogin(), 5); err != nil {
			t.Fatal(err)
		}
		if len(sp.dibuat) != 1 || sp.dibuat[0].Kategori != model.SimpananPokok {
			t.Errorf("penarikan = %+v, want hanya pokok (jumlah harus > 0)", sp.dibuat)
		}
	})

	t.Run("ditolak bila masih ada pinjaman aktif: tidak ada yang tertulis", func(t *testing.T) {
		s, db, ag, sp, au, pj := siapAnggotaLengkap()
		ag.tambah(anggotaUji())
		sp.saldo = model.SaldoSimpanan{Pokok: 500_000}
		pj.aktif = true

		_, err := s.Resign(ctxLogin(), 5)
		if !errors.Is(err, ErrMasihAdaPinjamanAktif) {
			t.Fatalf("err = %v, want ErrMasihAdaPinjamanAktif", err)
		}
		if db.rollback != 1 || len(sp.dibuat) != 0 || len(ag.keluar) != 0 || len(au.entri) != 0 {
			t.Errorf("rollback=%d penarikan=%d keluar=%d audit=%d", db.rollback, len(sp.dibuat), len(ag.keluar), len(au.entri))
		}
	})

	t.Run("ditolak bila sudah keluar", func(t *testing.T) {
		s, _, ag, sp, _ := siapAnggota()
		a := anggotaUji()
		a.Status = model.StatusAnggotaKeluar
		ag.tambah(a)
		sp.saldo = model.SaldoSimpanan{Pokok: 1}

		if _, err := s.Resign(ctxLogin(), 5); !errors.Is(err, ErrAnggotaSudahKeluar) {
			t.Errorf("err = %v, want ErrAnggotaSudahKeluar", err)
		}
		if len(sp.dibuat) != 0 {
			t.Error("tidak boleh ada penarikan untuk anggota yang sudah keluar")
		}
	})

	t.Run("saldo negatif ditolak", func(t *testing.T) {
		s, _, ag, sp, _ := siapAnggota()
		ag.tambah(anggotaUji())
		sp.saldo = model.SaldoSimpanan{Pokok: 500_000, Sukarela: -1}

		if _, err := s.Resign(ctxLogin(), 5); err == nil || len(sp.dibuat) != 0 {
			t.Errorf("err=%v penarikan=%d; saldo negatif tidak boleh diproses", err, len(sp.dibuat))
		}
	})

	t.Run("anggota tidak ada", func(t *testing.T) {
		s, _, _, _, _ := siapAnggota()
		if _, err := s.Resign(ctxLogin(), 99); !errors.Is(err, ErrAnggotaTidakDitemukan) {
			t.Errorf("err = %v, want ErrAnggotaTidakDitemukan", err)
		}
	})

	t.Run("audit gagal membatalkan transaksi", func(t *testing.T) {
		s, db, ag, sp, au := siapAnggota()
		ag.tambah(anggotaUji())
		sp.saldo = model.SaldoSimpanan{Pokok: 500_000}
		au.err = errors.New("disk penuh")

		if _, err := s.Resign(ctxLogin(), 5); err == nil || db.rollback != 1 {
			t.Errorf("err=%v rollback=%d; audit gagal harus membatalkan transaksi", err, db.rollback)
		}
	})

	t.Run("butuh login", func(t *testing.T) {
		s, _, _, _, _ := siapAnggota()
		if _, err := s.Resign(context.Background(), 5); !errors.Is(err, ErrBelumLogin) {
			t.Errorf("err = %v, want ErrBelumLogin", err)
		}
	})
}
