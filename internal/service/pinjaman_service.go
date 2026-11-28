package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

var (
	ErrPinjamanTidakDitemukan = errors.New("pinjaman tidak ditemukan")
	ErrBukanAnggotaAktif      = errors.New("anggota tidak aktif")
	ErrAdaPinjamanAktif       = errors.New("anggota masih memiliki pinjaman aktif")
	ErrJumlahMelebihiBatas    = errors.New("jumlah pinjaman melebihi batas maksimal")
	ErrTenorDiLuarJangkauan   = errors.New("tenor di luar ketentuan yang berlaku")
	ErrJumlahPinjamanTidakSah = errors.New("jumlah pinjaman tidak sah")
)

type PinjamanStore interface {
	AdaAktif(ctx context.Context, db database.DBTX, idAnggota int64) (bool, error)
	Create(ctx context.Context, db database.DBTX, p model.Pinjaman, jadwal []model.Angsuran) (int64, error)
	GetByID(ctx context.Context, db database.DBTX, id int64) (model.PinjamanListItem, error)
	List(ctx context.Context, db database.DBTX, status string) ([]model.PinjamanListItem, error)
	GetJadwalByPinjamanID(ctx context.Context, db database.DBTX, idPinjaman int64) ([]model.Angsuran, error)
	GetStatusAngsuranByPinjamanID(ctx context.Context, db database.DBTX, idPinjaman int64, dendaPerHari int64, asOf time.Time) ([]model.StatusAngsuran, error)
	GetPembayaranByPinjamanID(ctx context.Context, db database.DBTX, idPinjaman int64) ([]model.Pembayaran, error)
}

type PinjamanService struct {
	db                   database.Sumber
	pinjamanRepo         PinjamanStore
	anggotaRepo          AnggotaRepo
	simpananRepo         SimpananRepo
	auditSvc             *AuditService
	kelipatanBatasPinjam int
	tenorMin             int
	tenorMax             int
	jasaBps              int
	dendaPerHari         int64
	biayaAdminBps        int
	zona                 *time.Location
	now                  func() time.Time
}

type AjukanPinjamanInput struct {
	IDAnggota        int64
	Jumlah           string
	Tenor            int
	TanggalPencairan string // format YYYY-MM-DD, opsional bawaan hari ini
}

func NewPinjamanService(
	db database.Sumber,
	pinjamanRepo PinjamanStore,
	anggotaRepo AnggotaRepo,
	simpananRepo SimpananRepo,
	auditSvc *AuditService,
	kelipatanBatas int,
	tenorMin, tenorMax int,
	jasaBps int,
	dendaPerHari int64,
	biayaAdminBps int,
	zona *time.Location,
	now func() time.Time,
) *PinjamanService {
	if now == nil {
		now = time.Now
	}
	if zona == nil {
		zona = time.UTC
	}

	return &PinjamanService{
		db:                   db,
		pinjamanRepo:         pinjamanRepo,
		anggotaRepo:          anggotaRepo,
		simpananRepo:         simpananRepo,
		auditSvc:             auditSvc,
		kelipatanBatasPinjam: kelipatanBatas,
		tenorMin:             tenorMin,
		tenorMax:             tenorMax,
		jasaBps:              jasaBps,
		dendaPerHari:         dendaPerHari,
		biayaAdminBps:        biayaAdminBps,
		zona:                 zona,
		now:                  now,
	}
}

// AjukanPinjaman memeriksa kelayakan, menghitung jadwal angsuran, lalu menyimpannya dalam satu transaksi.
func (s *PinjamanService) AjukanPinjaman(ctx context.Context, in AjukanPinjamanInput) (int64, error) {
	pengguna, ok := IdentitasDari(ctx)
	if !ok {
		return 0, ErrBelumLogin
	}

	// Bersihkan dan validasi nominal
	bersih := strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(in.Jumlah), ".", ""), ",", "")
	jumlah, err := strconv.ParseInt(bersih, 10, 64)
	if err != nil || jumlah <= 0 {
		return 0, ErrJumlahPinjamanTidakSah
	}

	// Validasi tenor
	if in.Tenor < s.tenorMin || in.Tenor > s.tenorMax {
		return 0, ErrTenorDiLuarJangkauan
	}

	// Tentukan tanggal pencairan
	var tglPencairan time.Time
	if strings.TrimSpace(in.TanggalPencairan) != "" {
		t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(in.TanggalPencairan), s.zona)
		if err != nil {
			return 0, fmt.Errorf("format tanggal pencairan tidak valid: %w", err)
		}
		tglPencairan = t
	} else {
		tglPencairan = s.now().In(s.zona)
	}

	var idPinjamanBaru int64

	err = s.db.DenganTx(ctx, func(tx database.DBTX) error {
		// 1. Cek anggota
		anggota, err := s.anggotaRepo.GetByID(ctx, tx, in.IDAnggota)
		if err != nil {
			if errors.Is(err, repository.ErrTidakDitemukan) {
				return ErrAnggotaTidakDitemukan
			}
			return err
		}
		if anggota.Status != model.StatusAnggotaAktif {
			return ErrBukanAnggotaAktif
		}

		// 2. Cek apakah ada pinjaman aktif
		adaAktif, err := s.pinjamanRepo.AdaAktif(ctx, tx, in.IDAnggota)
		if err != nil {
			return err
		}
		if adaAktif {
			return ErrAdaPinjamanAktif
		}

		// 3. Cek batas pinjaman
		saldo, err := s.simpananRepo.Saldo(ctx, tx, in.IDAnggota)
		if err != nil {
			return err
		}
		batas := helper.HitungBatas(saldo.Pokok, saldo.Wajib, s.kelipatanBatasPinjam)
		if jumlah > batas {
			return ErrJumlahMelebihiBatas
		}

		// 4. Hitung jadwal angsuran
		jadwal, err := helper.BuatJadwal(jumlah, in.Tenor, s.jasaBps, tglPencairan)
		if err != nil {
			return err
		}

		// 5. Simpan pinjaman dan angsuran
		p := model.Pinjaman{
			IDAnggota:        in.IDAnggota,
			JumlahPinjaman:   jumlah,
			TenorBulan:       in.Tenor,
			JasaPerBulanBps:  s.jasaBps,
			DendaPerHari:     s.dendaPerHari,
			TanggalPencairan: tglPencairan,
			Status:           model.StatusPinjamanAktif,
			DibuatOleh:       pengguna.IDPengguna,
			DibuatPada:       s.now().UTC(),
		}

		idPinjaman, err := s.pinjamanRepo.Create(ctx, tx, p, jadwal)
		if err != nil {
			return err
		}
		idPinjamanBaru = idPinjaman

		// 6. Log audit
		detailAudit := map[string]any{
			"id_anggota": anggota.ID,
			"jumlah":     jumlah,
			"tenor":      in.Tenor,
			"batas":      batas,
		}
		return s.auditSvc.NewEntri(ctx, tx, "tambah", "pinjaman", idPinjaman, detailAudit)
	})

	if err != nil {
		return 0, err
	}

	return idPinjamanBaru, nil
}

// List mengambil daftar ringkas seluruh pinjaman dengan filter status opsional.
func (s *PinjamanService) List(ctx context.Context, status string) ([]model.PinjamanListItem, error) {
	_, ok := IdentitasDari(ctx)
	if !ok {
		return nil, ErrBelumLogin
	}

	return s.pinjamanRepo.List(ctx, s.db, status)
}

// Detail mengambil rincian lengkap pinjaman beserta status terkini setiap angsuran dan simulasi pelunasan.
func (s *PinjamanService) Detail(ctx context.Context, id int64) (model.DetailPinjaman, error) {
	_, ok := IdentitasDari(ctx)
	if !ok {
		return model.DetailPinjaman{}, ErrBelumLogin
	}

	item, err := s.pinjamanRepo.GetByID(ctx, s.db, id)
	if err != nil {
		if errors.Is(err, repository.ErrTidakDitemukan) {
			return model.DetailPinjaman{}, ErrPinjamanTidakDitemukan
		}
		return model.DetailPinjaman{}, err
	}

	asOf := s.now().In(s.zona)
	statusJadwal, err := s.pinjamanRepo.GetStatusAngsuranByPinjamanID(ctx, s.db, id, item.DendaPerHari, asOf)
	if err != nil {
		return model.DetailPinjaman{}, err
	}

	var totalBayar, sisaKewajiban int64
	for _, st := range statusJadwal {
		totalBayar += st.PokokDibayar + st.JasaDibayar + st.DendaDibayar
		sisaKewajiban += st.SisaPokok() + st.SisaJasa() + st.DendaTerutang
	}

	detail := model.DetailPinjaman{
		Pinjaman:        item.Pinjaman,
		NomorAnggota:    item.NomorAnggota,
		NamaAnggota:     item.NamaAnggota,
		Jadwal:          statusJadwal,
		TotalSudahBayar: totalBayar,
		SisaKewajiban:   sisaKewajiban,
	}

	if item.Status == model.StatusPinjamanAktif {
		rincian := helper.HitungPelunasan(statusJadwal, asOf, s.biayaAdminBps)
		detail.RincianLunas = &rincian
	}

	return detail, nil
}

// InfoPengajuan mengembalikan informasi batas pinjaman dan profil anggota untuk formulir pengajuan.
func (s *PinjamanService) InfoPengajuan(ctx context.Context, idAnggota int64) (int64, model.SaldoSimpanan, model.Anggota, error) {
	_, ok := IdentitasDari(ctx)
	if !ok {
		return 0, model.SaldoSimpanan{}, model.Anggota{}, ErrBelumLogin
	}

	anggota, err := s.anggotaRepo.GetByID(ctx, s.db, idAnggota)
	if err != nil {
		if errors.Is(err, repository.ErrTidakDitemukan) {
			return 0, model.SaldoSimpanan{}, model.Anggota{}, ErrAnggotaTidakDitemukan
		}
		return 0, model.SaldoSimpanan{}, model.Anggota{}, err
	}

	saldo, err := s.simpananRepo.Saldo(ctx, s.db, idAnggota)
	if err != nil {
		return 0, model.SaldoSimpanan{}, model.Anggota{}, err
	}

	batas := helper.HitungBatas(saldo.Pokok, saldo.Wajib, s.kelipatanBatasPinjam)
	return batas, saldo, anggota, nil
}
