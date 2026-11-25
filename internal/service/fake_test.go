package service

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

var errTakDipakai = errors.New("fake: DBTX tidak boleh dipakai langsung")

func logDiam() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fakeDB adalah Sumber tiruan: DenganTx menjalankan fn dan mencatat hasilnya.
type fakeDB struct {
	tx       int
	rollback int
}

func (*fakeDB) ExecContext(context.Context, string, ...any) (sql.Result, error) {
	return nil, errTakDipakai
}
func (*fakeDB) QueryContext(context.Context, string, ...any) (*sql.Rows, error) {
	return nil, errTakDipakai
}
func (*fakeDB) QueryRowContext(context.Context, string, ...any) *sql.Row { return nil }

func (f *fakeDB) DenganTx(_ context.Context, fn func(tx database.DBTX) error) error {
	f.tx++
	err := fn(f)
	if err != nil {
		f.rollback++
	}
	return err
}

type fakeHasher struct{ dipanggil int }

func (*fakeHasher) Hash(p string) (string, error) { return "hash:" + p, nil }
func (h *fakeHasher) Cocok(p, hash string) (bool, error) {
	h.dipanggil++
	return hash == "hash:"+p, nil
}

type fakePengguna struct {
	data   map[string]model.Pengguna
	nextID int64
}

func (f *fakePengguna) CariByUsername(_ context.Context, _ database.DBTX, u string) (model.Pengguna, error) {
	p, ok := f.data[u]
	if !ok {
		return model.Pengguna{}, repository.ErrTidakDitemukan
	}
	return p, nil
}

func (f *fakePengguna) Buat(_ context.Context, _ database.DBTX, p model.Pengguna) (int64, error) {
	if _, ada := f.data[p.Username]; ada {
		return 0, repository.ErrDuplikat
	}
	f.nextID++
	p.ID = f.nextID
	if f.data == nil {
		f.data = map[string]model.Pengguna{}
	}
	f.data[p.Username] = p
	return p.ID, nil
}

type fakeSesi struct {
	data map[string]model.Sesi
	user map[int64]model.Pengguna
}

func (f *fakeSesi) Create(_ context.Context, _ database.DBTX, s model.Sesi) error {
	if f.data == nil {
		f.data = map[string]model.Sesi{}
	}
	f.data[s.TokenHash] = s
	return nil
}

func (f *fakeSesi) CariAktif(_ context.Context, _ database.DBTX, h string, now time.Time) (model.Sesi, model.Pengguna, error) {
	s, ok := f.data[h]
	if !ok || !s.KedaluwarsaPada.After(now) {
		return model.Sesi{}, model.Pengguna{}, repository.ErrTidakDitemukan
	}
	p := f.user[s.IDPengguna]
	if !p.Aktif {
		return model.Sesi{}, model.Pengguna{}, repository.ErrTidakDitemukan
	}
	return s, p, nil
}

func (f *fakeSesi) HapusByTokenHash(_ context.Context, _ database.DBTX, h string) error {
	delete(f.data, h)
	return nil
}

func (f *fakeSesi) HapusKedaluwarsa(context.Context, database.DBTX, time.Time) error { return nil }

type fakeAudit struct {
	entri []model.Audit
	err   error
}

func (f *fakeAudit) Create(_ context.Context, _ database.DBTX, e model.Audit) error {
	if f.err != nil {
		return f.err
	}
	f.entri = append(f.entri, e)
	return nil
}

type fakeAnggota struct {
	dibuat []model.Anggota
	nomor  map[string]bool

	hasilFilter []model.Anggota
	totalFilter int
	errFilter   error

	cariFilter   string
	statusFilter string
	pageFilter   int
	limitFilter  int

	// data dipakai GetByID dan GetByIDForUpdate; isi lewat tambah.
	data      map[int64]model.Anggota
	kunci     int // berapa kali baris dikunci (GetByIDForUpdate)
	ubah      []ubahProfil
	keluar    []keluarAnggota
	errUbah   error
	errKeluar error
}

type ubahProfil struct {
	id            int64
	nama, telepon string
}

type keluarAnggota struct {
	id      int64
	tanggal time.Time
}

func (f *fakeAnggota) tambah(a model.Anggota) {
	if f.data == nil {
		f.data = map[int64]model.Anggota{}
	}
	f.data[a.ID] = a
}

func (f *fakeAnggota) Create(_ context.Context, _ database.DBTX, a model.Anggota) (int64, error) {
	if f.nomor[a.NomorAnggota] {
		return 0, repository.ErrDuplikat
	}
	if f.nomor == nil {
		f.nomor = map[string]bool{}
	}
	f.nomor[a.NomorAnggota] = true
	f.dibuat = append(f.dibuat, a)
	return int64(len(f.dibuat)), nil
}

func (f *fakeAnggota) ListFiltered(
	ctx context.Context,
	db database.DBTX,
	cari, status string,
	page, limit int,
) ([]model.Anggota, int, error) {

	f.cariFilter = cari
	f.statusFilter = status
	f.pageFilter = page
	f.limitFilter = limit

	return f.hasilFilter, f.totalFilter, f.errFilter
}

func (f *fakeAnggota) GetByID(_ context.Context, _ database.DBTX, id int64) (model.Anggota, error) {
	a, ok := f.data[id]
	if !ok {
		return model.Anggota{}, repository.ErrTidakDitemukan
	}
	return a, nil
}

func (f *fakeAnggota) GetByIDForUpdate(ctx context.Context, db database.DBTX, id int64) (model.Anggota, error) {
	f.kunci++
	return f.GetByID(ctx, db, id)
}

func (f *fakeAnggota) Update(_ context.Context, _ database.DBTX, id int64, nama, telepon string) error {
	if f.errUbah != nil {
		return f.errUbah
	}
	f.ubah = append(f.ubah, ubahProfil{id, nama, telepon})
	return nil
}

func (f *fakeAnggota) Deactivate(_ context.Context, _ database.DBTX, id int64, tanggal time.Time) error {
	if f.errKeluar != nil {
		return f.errKeluar
	}
	f.keluar = append(f.keluar, keluarAnggota{id, tanggal})
	return nil
}

type fakeSimpanan struct {
	dibuat []model.TransaksiSimpanan
	saldo  model.SaldoSimpanan // dikembalikan apa adanya oleh Saldo

	// data dipakai GetByIDForUpdate dan List; isi lewat tambah.
	data     map[int64]model.TransaksiSimpanan
	batal    []int64 // ID transaksi yang dibatalkan
	alasan   string
	errBatal error
	kunciTrx int
}

func (f *fakeSimpanan) tambah(t model.TransaksiSimpanan) {
	if f.data == nil {
		f.data = map[int64]model.TransaksiSimpanan{}
	}
	f.data[t.ID] = t
}

func (f *fakeSimpanan) Create(_ context.Context, _ database.DBTX, t model.TransaksiSimpanan) (int64, error) {
	if t.Kategori == model.SimpananWajib && t.TipeMutasi == model.MutasiSetor {
		for _, d := range f.dibuat {
			if d.IDAnggota == t.IDAnggota && d.Kategori == model.SimpananWajib && d.Periode.Equal(*t.Periode) {
				return 0, repository.ErrDuplikat
			}
		}
	}
	f.dibuat = append(f.dibuat, t)
	return int64(len(f.dibuat)), nil
}

func (f *fakeSimpanan) Saldo(context.Context, database.DBTX, int64) (model.SaldoSimpanan, error) {
	return f.saldo, nil
}

func (f *fakeSimpanan) List(_ context.Context, _ database.DBTX, idAnggota int64) ([]model.TransaksiSimpanan, error) {
	var hasil []model.TransaksiSimpanan
	for _, t := range f.data {
		if t.IDAnggota == idAnggota {
			hasil = append(hasil, t)
		}
	}
	return hasil, nil
}

func (f *fakeSimpanan) GetByIDForUpdate(_ context.Context, _ database.DBTX, id int64) (model.TransaksiSimpanan, error) {
	f.kunciTrx++
	t, ok := f.data[id]
	if !ok {
		return model.TransaksiSimpanan{}, repository.ErrTidakDitemukan
	}
	return t, nil
}

func (f *fakeSimpanan) Batalkan(_ context.Context, _ database.DBTX, id, _ int64, alasan string, _ time.Time) error {
	if f.errBatal != nil {
		return f.errBatal
	}
	f.batal = append(f.batal, id)
	f.alasan = alasan
	return nil
}

type fakePinjamanAktif struct{ aktif bool }

func (f *fakePinjamanAktif) AdaAktif(context.Context, database.DBTX, int64) (bool, error) {
	return f.aktif, nil
}
