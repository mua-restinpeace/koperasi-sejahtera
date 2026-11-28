package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

var (
	ErrPembayaranTidakDitemukan  = errors.New("pembayaran tidak ditemukan")
	ErrPembayaranSudahDibatalkan = errors.New("pembayaran sudah dibatalkan")
	ErrPinjamanTidakAktif        = errors.New("pinjaman tidak aktif")
	ErrPinjamanTidakDitemukan    = errors.New("pinjaman tidak ditemukan")
)

const maksPembayaran = int64(1_000_000_000)

type PembayaranRepository interface {
	Create(
		ctx context.Context,
		db database.DBTX,
		p model.Pembayaran,
		dibuatOleh int64,
		keterangan string,
	) (int64, error)

	ListAktifByPinjaman(
		ctx context.Context,
		db database.DBTX,
		idPinjaman int64,
	) ([]model.Pembayaran, error)

	ListRiwayatByPinjaman(
		ctx context.Context,
		db database.DBTX,
		idPinjaman int64,
	) ([]model.Pembayaran, error)

	GetByIDForUpdate(
		ctx context.Context,
		db database.DBTX,
		id int64,
	) (model.Pembayaran, error)

	Batalkan(
		ctx context.Context,
		db database.DBTX,
		id int64,
		oleh int64,
		alasan string,
		pada time.Time,
	) error
}

type PembayaranAngsuranRepository interface {
	ListByPinjaman(
		ctx context.Context,
		db database.DBTX,
		idPinjaman int64,
	) ([]model.Angsuran, error)
}

type PembayaranPinjamanRepository interface {
	GetByID(
		ctx context.Context,
		db database.DBTX,
		idPinjaman int64,
	) (model.Pinjaman, error)

	KunciAktif(
		ctx context.Context,
		db database.DBTX,
		idPinjaman int64,
	) error

	KunciByID(
		ctx context.Context,
		db database.DBTX,
		idPinjaman int64,
	) error

	UbahStatus(
		ctx context.Context,
		db database.DBTX,
		idPinjaman int64,
		status string,
		tanggalLunas *time.Time,
	) error
}

type PembayaranService struct {
	db             database.Sumber
	pinjamanRepo   PembayaranPinjamanRepository
	pembayaranRepo PembayaranRepository
	angsuranRepo   PembayaranAngsuranRepository
	replaySvc      *ReplayAlokasiService
	auditSvc       *AuditService

	aturan        helper.AturanHitung
	biayaAdminBps int

	now  func() time.Time
	zona *time.Location
}

type CatatPembayaranInput struct {
	TanggalBayar time.Time
	Jumlah       int64
	Keterangan   string
}

type CatatPelunasanInput struct {
	TanggalBayar time.Time
	Keterangan   string
}

type HasilPelunasan struct {
	IDPembayaran int64
	Rincian      model.RincianPelunasan
}

type DetailPinjaman struct {
	Pinjaman          model.Pinjaman
	Angsuran          []model.StatusAngsuran
	RiwayatPembayaran []model.Pembayaran
	RincianPelunasan  model.RincianPelunasan
}

func NewPembayaranService(
	db database.Sumber,
	pinjamanRepo PembayaranPinjamanRepository,
	pembayaranRepo PembayaranRepository,
	angsuranRepo PembayaranAngsuranRepository,
	replaySvc *ReplayAlokasiService,
	auditSvc *AuditService,
	aturan helper.AturanHitung,
	biayaAdminBps int,
	zona *time.Location,
	now func() time.Time,
) *PembayaranService {
	if zona == nil {
		zona = time.UTC
	}

	if now == nil {
		now = time.Now
	}

	return &PembayaranService{
		db:             db,
		pinjamanRepo:   pinjamanRepo,
		pembayaranRepo: pembayaranRepo,
		angsuranRepo:   angsuranRepo,
		replaySvc:      replaySvc,
		auditSvc:       auditSvc,
		aturan:         aturan,
		biayaAdminBps:  biayaAdminBps,
		zona:           zona,
		now:            now,
	}
}

// CatatPembayaran mencatat pembayaran angsuran biasa.
// Alokasi dihitung ulang berdasarkan seluruh pembayaran aktif.
func (s *PembayaranService) CatatPembayaran(
	ctx context.Context,
	idPinjaman int64,
	in CatatPembayaranInput,
) (int64, error) {
	idn, ok := IdentitasDari(ctx)
	if !ok {
		return 0, ErrBelumLogin
	}

	if idPinjaman <= 0 {
		return 0, pesanValidasi("pinjaman", "Pinjaman tidak valid.")
	}

	if in.Jumlah <= 0 || in.Jumlah > maksPembayaran {
		return 0, pesanValidasi(
			"jumlah",
			"Jumlah pembayaran harus lebih dari 0 dan tidak melebihi batas yang diizinkan.",
		)
	}

	tanggal := s.tanggalBisnis(in.TanggalBayar)
	if tanggal.IsZero() {
		return 0, pesanValidasi("tanggal_bayar", "Tanggal pembayaran wajib diisi.")
	}

	keterangan := strings.TrimSpace(in.Keterangan)
	if utf8.RuneCountInString(keterangan) > 255 {
		return 0, pesanValidasi("keterangan", "Keterangan maksimal 255 karakter.")
	}

	var idPembayaran int64

	err := s.db.DenganTx(ctx, func(tx database.DBTX) error {
		// Kunci pinjaman agar dua pembayaran bersamaan
		// tidak menghasilkan alokasi yang saling bertabrakan.
		if err := s.pinjamanRepo.KunciAktif(ctx, tx, idPinjaman); err != nil {
			if errors.Is(err, repository.ErrTidakDitemukan) {
				return ErrPinjamanTidakAktif
			}
			return err
		}

		p := model.Pembayaran{
			IDPinjaman:   idPinjaman,
			Jenis:        model.PembayaranAngsuran,
			TanggalBayar: tanggal,
			Jumlah:       in.Jumlah,
		}

		id, err := s.pembayaranRepo.Create(
			ctx, tx, p, idn.IDPengguna, keterangan,
		)
		if err != nil {
			return fmt.Errorf("simpan pembayaran: %w", err)
		}

		p.ID = id
		idPembayaran = id

		// Replay menolak kelebihan pembayaran.
		if err := s.replaySvc.ReplayTx(ctx, tx, idPinjaman); err != nil {
			return fmt.Errorf("alokasi pembayaran: %w", err)
		}

		if err := s.perbaruiStatus(ctx, tx, idPinjaman); err != nil {
			return err
		}

		return s.auditSvc.NewEntri(
			ctx, tx,
			"catat_pembayaran",
			"pembayaran_pinjaman",
			id,
			map[string]any{
				"id_pinjaman":   idPinjaman,
				"jenis":         p.Jenis,
				"tanggal_bayar": tanggal.Format("2006-01-02"),
				"jumlah":        p.Jumlah,
			},
		)
	})

	if err != nil {
		return 0, err
	}

	return idPembayaran, nil
}

// CatatPelunasan mencatat pelunasan dipercepat.
func (s *PembayaranService) CatatPelunasan(
	ctx context.Context,
	idPinjaman int64,
	in CatatPelunasanInput,
) (HasilPelunasan, error) {
	idn, ok := IdentitasDari(ctx)
	if !ok {
		return HasilPelunasan{}, ErrBelumLogin
	}

	if idPinjaman <= 0 {
		return HasilPelunasan{}, pesanValidasi("pinjaman", "Pinjaman tidak valid.")
	}

	tanggal := s.tanggalBisnis(in.TanggalBayar)
	if tanggal.IsZero() {
		return HasilPelunasan{}, pesanValidasi("tanggal_bayar", "Tanggal pelunasan wajib diisi.")
	}

	keterangan := strings.TrimSpace(in.Keterangan)
	if utf8.RuneCountInString(keterangan) > 255 {
		return HasilPelunasan{}, pesanValidasi("keterangan", "Keterangan maksimal 255 karakter.")
	}

	var hasil HasilPelunasan

	err := s.db.DenganTx(ctx, func(tx database.DBTX) error {
		if err := s.pinjamanRepo.KunciAktif(ctx, tx, idPinjaman); err != nil {
			if errors.Is(err, repository.ErrTidakDitemukan) {
				return ErrPinjamanTidakAktif
			}
			return err
		}

		rincian, err := s.hitungRincianPelunasan(
			ctx, tx, idPinjaman, tanggal,
		)
		if err != nil {
			return err
		}

		if rincian.Total <= 0 {
			return pesanValidasi("pinjaman", "Tidak ada sisa kewajiban yang dapat dilunasi.")
		}

		p := model.Pembayaran{
			IDPinjaman:   idPinjaman,
			Jenis:        model.PembayaranPelunasan,
			TanggalBayar: tanggal,
			Jumlah:       rincian.Total,
			BiayaAdmin:   rincian.BiayaAdmin,
		}

		id, err := s.pembayaranRepo.Create(
			ctx, tx, p, idn.IDPengguna, keterangan,
		)
		if err != nil {
			return fmt.Errorf("simpan pelunasan: %w", err)
		}

		if err := s.replaySvc.ReplayTx(ctx, tx, idPinjaman); err != nil {
			return fmt.Errorf("alokasi pelunasan: %w", err)
		}

		if err := s.perbaruiStatus(ctx, tx, idPinjaman); err != nil {
			return err
		}

		if err := s.auditSvc.NewEntri(
			ctx, tx,
			"catat_pelunasan",
			"pembayaran_pinjaman",
			id,
			map[string]any{
				"id_pinjaman": idPinjaman,
				"rincian":     rincian,
			},
		); err != nil {
			return err
		}

		hasil = HasilPelunasan{
			IDPembayaran: id,
			Rincian:      rincian,
		}

		return nil
	})

	if err != nil {
		return HasilPelunasan{}, err
	}

	return hasil, nil
}

// BatalkanPembayaran membatalkan transaksi dan menghitung ulang
// seluruh alokasi pembayaran pada pinjaman terkait.
func (s *PembayaranService) BatalkanPembayaran(
	ctx context.Context,
	idPembayaran int64,
	alasan string,
) (int64, error) {
	idn, ok := IdentitasDari(ctx)
	if !ok {
		return 0, ErrBelumLogin
	}

	alasan = strings.TrimSpace(alasan)
	if alasan == "" || utf8.RuneCountInString(alasan) > 255 {
		return 0, pesanValidasi("alasan", "Alasan pembatalan wajib diisi maksimal 255 karakter.")
	}

	if idPembayaran <= 0 {
		return 0, pesanValidasi("pembayaran", "Pembayaran tidak valid.")
	}

	var idPinjaman int64

	err := s.db.DenganTx(ctx, func(tx database.DBTX) error {
		p, err := s.pembayaranRepo.GetByIDForUpdate(
			ctx, tx, idPembayaran,
		)
		if errors.Is(err, repository.ErrTidakDitemukan) {
			return ErrPembayaranTidakDitemukan
		}
		if err != nil {
			return err
		}

		idPinjaman = p.IDPinjaman

		if err := s.pinjamanRepo.KunciByID(ctx, tx, idPinjaman); err != nil {
			return err
		}

		err = s.pembayaranRepo.Batalkan(
			ctx,
			tx,
			idPembayaran,
			idn.IDPengguna,
			alasan,
			s.now().UTC(),
		)

		if errors.Is(err, repository.ErrTidakDitemukan) {
			return ErrPembayaranSudahDibatalkan
		}
		if err != nil {
			return err
		}

		if err := s.replaySvc.ReplayTx(ctx, tx, idPinjaman); err != nil {
			return fmt.Errorf("replay setelah pembatalan: %w", err)
		}

		if err := s.perbaruiStatus(ctx, tx, idPinjaman); err != nil {
			return err
		}

		return s.auditSvc.NewEntri(
			ctx, tx,
			"batalkan_pembayaran",
			"pembayaran_pinjaman",
			idPembayaran,
			map[string]any{
				"id_pinjaman": idPinjaman,
				"alasan":      alasan,
			},
		)
	})

	return idPinjaman, err
}

// GetDetailPinjaman mengambil informasi pinjaman, status angsuran,
// riwayat pembayaran, dan estimasi pelunasan jika pinjaman masih aktif.
func (s *PembayaranService) GetDetailPinjaman(
	ctx context.Context,
	idPinjaman int64,
) (DetailPinjaman, error) {
	_, ok := IdentitasDari(ctx)
	if !ok {
		return DetailPinjaman{}, ErrBelumLogin
	}

	if idPinjaman <= 0 {
		return DetailPinjaman{}, pesanValidasi(
			"pinjaman",
			"Pinjaman tidak valid.",
		)
	}

	var hasil DetailPinjaman
	tanggal := s.hariIni()

	err := s.db.DenganTx(ctx, func(tx database.DBTX) error {
		pinjaman, err := s.pinjamanRepo.GetByID(
			ctx, tx, idPinjaman,
		)
		if errors.Is(err, repository.ErrTidakDitemukan) {
			return ErrPinjamanTidakDitemukan
		}
		if err != nil {
			return err
		}

		jadwal, err := s.angsuranRepo.ListByPinjaman(
			ctx, tx, idPinjaman,
		)
		if err != nil {
			return err
		}

		pembayaranAktif, err := s.pembayaranRepo.ListAktifByPinjaman(
			ctx, tx, idPinjaman,
		)
		if err != nil {
			return err
		}

		statusAngsuran, err := helper.HitungStatus(
			jadwal,
			pembayaranAktif,
			tanggal,
			s.aturan,
		)
		if err != nil {
			return err
		}

		riwayat, err := s.pembayaranRepo.ListRiwayatByPinjaman(
			ctx, tx, idPinjaman,
		)
		if err != nil {
			return err
		}

		hasil = DetailPinjaman{
			Pinjaman:          pinjaman,
			Angsuran:          statusAngsuran,
			RiwayatPembayaran: riwayat,
		}

		if pinjaman.Status == "aktif" {
			hasil.RincianPelunasan = helper.HitungPelunasan(
				statusAngsuran,
				tanggal,
				s.biayaAdminBps,
			)
		}

		return nil
	})

	if err != nil {
		return DetailPinjaman{}, err
	}

	return hasil, nil
}

// PratinjauPelunasan menghitung estimasi kewajiban pelunasan
// tanpa mencatat pembayaran atau mengubah status pinjaman.
func (s *PembayaranService) PratinjauPelunasan(
	ctx context.Context,
	idPinjaman int64,
	tanggalBayar time.Time,
) (model.RincianPelunasan, error) {
	_, ok := IdentitasDari(ctx)
	if !ok {
		return model.RincianPelunasan{}, ErrBelumLogin
	}

	if idPinjaman <= 0 {
		return model.RincianPelunasan{}, pesanValidasi(
			"pinjaman",
			"Pinjaman tidak valid.",
		)
	}

	tanggal := s.tanggalBisnis(tanggalBayar)
	if tanggal.IsZero() {
		return model.RincianPelunasan{}, pesanValidasi(
			"tanggal_bayar",
			"Tanggal pelunasan wajib diisi.",
		)
	}

	var rincian model.RincianPelunasan

	err := s.db.DenganTx(ctx, func(tx database.DBTX) error {
		pinjaman, err := s.pinjamanRepo.GetByID(
			ctx, tx, idPinjaman,
		)
		if errors.Is(err, repository.ErrTidakDitemukan) {
			return ErrPinjamanTidakAktif
		}
		if err != nil {
			return err
		}

		if pinjaman.Status != "aktif" {
			return ErrPinjamanTidakAktif
		}

		rincian, err = s.hitungRincianPelunasan(
			ctx, tx, idPinjaman, tanggal,
		)
		return err
	})

	if err != nil {
		return model.RincianPelunasan{}, err
	}

	return rincian, nil
}

// hitungRincianPelunasan menghitung kewajiban berdasarkan
// seluruh pembayaran aktif pada tanggal pelunasan.
func (s *PembayaranService) hitungRincianPelunasan(
	ctx context.Context,
	db database.DBTX,
	idPinjaman int64,
	tanggal time.Time,
) (model.RincianPelunasan, error) {
	jadwal, err := s.angsuranRepo.ListByPinjaman(ctx, db, idPinjaman)
	if err != nil {
		return model.RincianPelunasan{}, err
	}

	pembayaran, err := s.pembayaranRepo.ListAktifByPinjaman(
		ctx, db, idPinjaman,
	)
	if err != nil {
		return model.RincianPelunasan{}, err
	}

	status, err := helper.HitungStatus(
		jadwal, pembayaran, tanggal, s.aturan,
	)
	if err != nil {
		return model.RincianPelunasan{}, err
	}

	return helper.HitungPelunasan(
		status, tanggal, s.biayaAdminBps,
	), nil
}

// perbaruiStatus menyinkronkan status pinjaman dengan hasil replay.
func (s *PembayaranService) perbaruiStatus(
	ctx context.Context,
	tx database.DBTX,
	idPinjaman int64,
) error {
	jadwal, err := s.angsuranRepo.ListByPinjaman(ctx, tx, idPinjaman)
	if err != nil {
		return err
	}

	pembayaran, err := s.pembayaranRepo.ListAktifByPinjaman(
		ctx, tx, idPinjaman,
	)
	if err != nil {
		return err
	}

	status, err := helper.HitungStatus(
		jadwal,
		pembayaran,
		s.hariIni(),
		s.aturan,
	)
	if err != nil {
		return err
	}

	if helper.IsSemuaLunas(status) {
		var tanggalLunas time.Time

		for _, a := range status {
			if a.LunasPada.After(tanggalLunas) {
				tanggalLunas = a.LunasPada
			}
		}

		if tanggalLunas.IsZero() {
			tanggalLunas = s.hariIni()
		}

		return s.pinjamanRepo.UbahStatus(
			ctx, tx, idPinjaman, "lunas", &tanggalLunas,
		)
	}

	return s.pinjamanRepo.UbahStatus(
		ctx, tx, idPinjaman, "aktif", nil,
	)
}

func (s *PembayaranService) tanggalBisnis(t time.Time) time.Time {
	if t.IsZero() {
		return time.Time{}
	}

	n := t.In(s.zona)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *PembayaranService) hariIni() time.Time {
	n := s.now().In(s.zona)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}
