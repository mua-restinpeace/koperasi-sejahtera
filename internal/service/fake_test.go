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
