package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

func siapAnggota() (*AnggotaService, *fakeDB, *fakeAnggota, *fakeSimpanan, *fakeAudit) {
	db := &fakeDB{}
	ag, sp, au := &fakeAnggota{}, &fakeSimpanan{}, &fakeAudit{}
	// 2026-11-10 20:00 UTC = 11 November 03:00 WIB: tanggal bisnis harus 11 November.
	now := func() time.Time { return time.Date(2026, 11, 10, 20, 0, 0, 0, time.UTC) }
	jkt, _ := time.LoadLocation("Asia/Jakarta")
	if jkt == nil {
		jkt = time.FixedZone("WIB", 7*3600)
	}
	return NewAnggotaService(db, ag, sp, NewAuditService(au), 500_000, jkt, now), db, ag, sp, au
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
