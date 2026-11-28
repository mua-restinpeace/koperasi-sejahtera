package service

import (
	"context"
	"fmt"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type ReplayPinjamanRepo interface {
	KunciByID(ctx context.Context, db database.DBTX, idPinjaman int64) error
}

type ReplayAngsuranRepo interface {
	ListByPinjaman(ctx context.Context, db database.DBTX, idPinjaman int64) ([]model.Angsuran, error)
}

type ReplayPembayaranRepo interface {
	ListAktifByPinjaman(ctx context.Context, db database.DBTX, idPinjaman int64) ([]model.Pembayaran, error)
}

type ReplayAlokasiRepo interface {
	DeleteByPinjaman(ctx context.Context, db database.DBTX, idPinjaman int64) error
	CreateBatch(ctx context.Context, db database.DBTX, idPinjaman int64, alokasi []model.Alokasi) error
}

type ReplayAlokasiService struct {
	db             database.Sumber
	pinjamanRepo   ReplayPinjamanRepo
	angsuranRepo   ReplayAngsuranRepo
	pembayaranRepo ReplayPembayaranRepo
	alokasiRepo    ReplayAlokasiRepo
	aturan         helper.AturanHitung
}

func NewReplayAlokasiService(
	db database.Sumber,
	pinjamanRepo ReplayPinjamanRepo,
	angsuranRepo ReplayAngsuranRepo,
	pembayaranRepo ReplayPembayaranRepo,
	alokasiRepo ReplayAlokasiRepo,
	aturan helper.AturanHitung,
) *ReplayAlokasiService {
	return &ReplayAlokasiService{
		db:             db,
		pinjamanRepo:   pinjamanRepo,
		angsuranRepo:   angsuranRepo,
		pembayaranRepo: pembayaranRepo,
		alokasiRepo:    alokasiRepo,
		aturan:         aturan,
	}
}

// Replay menjalankan perhitungan ulang alokasi dalam transaksi DB.
func (s *ReplayAlokasiService) Replay(
	ctx context.Context,
	idPinjaman int64,
) error {
	return s.db.DenganTx(ctx, func(tx database.DBTX) error {
		return s.ReplayTx(ctx, tx, idPinjaman)
	})
}

// ReplayTx menghitung ulang alokasi menggunakan transaksi yang sudah berjalan.
// Fungsi ini dapat dipanggil dari service pembayaran atau pembatalan.
func (s *ReplayAlokasiService) ReplayTx(
	ctx context.Context,
	tx database.DBTX,
	idPinjaman int64,
) error {
	// Kunci pinjaman untuk mencegah perubahan bersamaan.
	if err := s.pinjamanRepo.KunciByID(ctx, tx, idPinjaman); err != nil {
		return err
	}

	// Ambil seluruh jadwal angsuran.
	jadwal, err := s.angsuranRepo.ListByPinjaman(ctx, tx, idPinjaman)
	if err != nil {
		return fmt.Errorf("replay: mengambil jadwal: %w", err)
	}

	// Ambil pembayaran yang masih aktif.
	pembayaran, err := s.pembayaranRepo.ListAktifByPinjaman(ctx, tx, idPinjaman)
	if err != nil {
		return fmt.Errorf("replay: mengambil pembayaran: %w", err)
	}

	// Hitung ulang seluruh alokasi dari awal.
	alokasi, err := helper.HitungAlokasi(jadwal, pembayaran, s.aturan)
	if err != nil {
		return fmt.Errorf("replay: menghitung alokasi: %w", err)
	}

	// Hapus alokasi lama.
	if err := s.alokasiRepo.DeleteByPinjaman(ctx, tx, idPinjaman); err != nil {
		return fmt.Errorf("replay: menghapus alokasi lama: %w", err)
	}

	// Simpan hasil alokasi terbaru.
	if err := s.alokasiRepo.CreateBatch(ctx, tx, idPinjaman, alokasi); err != nil {
		return fmt.Errorf("replay: menyimpan alokasi baru: %w", err)
	}

	return nil
}
