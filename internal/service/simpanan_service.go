package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

var (
	ErrTransaksiTidakDitemukan  = errors.New("transaksi simpanan tidak ditemukan")
	ErrSudahDibatalkan          = errors.New("transaksi simpanan sudah dibatalkan")
	ErrPokokTidakBisaDibatalkan = errors.New("simpanan pokok tidak dapat dibatalkan")
)

// maksJumlahSimpanan membatasi isian jumlah agar salah ketik tertolak
const maksJumlahSimpanan = 1_000_000_000

type SimpananRepo interface {
	Create(ctx context.Context, db database.DBTX, t model.TransaksiSimpanan) (int64, error)
	Saldo(ctx context.Context, db database.DBTX, idAnggota int64) (model.SaldoSimpanan, error)
	List(ctx context.Context, db database.DBTX, idAnggota int64) ([]model.TransaksiSimpanan, error)
}

type SimpananService struct {
	db           database.Sumber
	anggotaRepo  AnggotaRepo
	simpananRepo SimpananRepo
	auditSvc     *AuditService
	wajib        int64 // jumlah tetap simpanan wajib per bulan
	now          func() time.Time
	zona         *time.Location
}

func NewSimpananService(db database.Sumber, anggotaRepo AnggotaRepo, simpananRepo SimpananRepo, auditSvc *AuditService, wajib int64, zona *time.Location, now func() time.Time) *SimpananService {
	if now == nil {
		now = time.Now
	}

	if zona == nil {
		zona = time.UTC
	}

	return &SimpananService{
		db:           db,
		anggotaRepo:  anggotaRepo,
		simpananRepo: simpananRepo,
		auditSvc:     auditSvc,
		wajib:        wajib,
		now:          now,
		zona:         zona,
	}
}

// RiwayatSimpanan adalah isi halaman simpanan satu anggota
type RiwayatSimpanan struct {
	Anggota   model.Anggota
	Saldo     model.SaldoSimpanan
	Transaksi []model.TransaksiSimpanan
	Wajib     int64
	Hari      time.Time
}

// Riwayat menampilkan saldo dan seluruh transaksi simpanan seorang anggota
func (s *SimpananService) Riwayet(ctx context.Context, idAnggota int64) (RiwayatSimpanan, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return RiwayatSimpanan{}, ErrBelumLogin
	}

	a, err := findAnggota(ctx, s.anggotaRepo, s.db, idAnggota, false)
	if err != nil {
		return RiwayatSimpanan{}, err
	}

	saldo, err := s.simpananRepo.Saldo(ctx, s.db, idAnggota)
	if err != nil {
		return RiwayatSimpanan{}, fmt.Errorf("saldo simpanan: %w", err)
	}

	trx, err := s.simpananRepo.List(ctx, s.db, idAnggota)
	if err != nil {
		return RiwayatSimpanan{}, fmt.Errorf("riwayat simpanan: %w", err)
	}

	return RiwayatSimpanan{
		Anggota:   a,
		Saldo:     saldo,
		Transaksi: trx,
		Wajib:     s.wajib,
		Hari:      s.hariIni(),
	}, nil
}

func (s *SimpananService) hariIni() time.Time {
	n := s.now().In(s.zona)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}
