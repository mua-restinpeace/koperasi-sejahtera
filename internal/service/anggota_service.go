package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

type AnggotaRepo interface {
	Create(ctx context.Context, db database.DBTX, a model.Anggota) (int64, error)
	ListFiltered(ctx context.Context, db database.DBTX, cari, status string, page, limit int) ([]model.Anggota, int, error)
	GetByID(ctx context.Context, db database.DBTX, id int64) (model.Anggota, error)
	GetByIDForUpdate(ctx context.Context, db database.DBTX, id int64) (model.Anggota, error)
	Update(ctx context.Context, db database.DBTX, id int64, nama, telepon string) error
}

type SimpananRepo interface {
	Create(ctx context.Context, db database.DBTX, t model.TransaksiSimpanan) (int64, error)
	Saldo(ctx context.Context, db database.DBTX, idAnggota int64) (model.SaldoSimpanan, error)
}

type PinjamanRepo interface {
	AdaAktif(ctx context.Context, db database.DBTX, idAnggota int64) (bool, error)
}

type PaginatedAnggotaResponse struct {
	Data     []model.Anggota
	Total    int
	Page     int
	PageSize int
}

type AnggotaService struct {
	db            database.Sumber
	anggotaRepo   AnggotaRepo
	simpananRepo  SimpananRepo
	pinjamanRepo  PinjamanRepo
	auditSvc      *AuditService
	simpananPokok int64
	now           func() time.Time
	zona          *time.Location
}

// UpdateAnggotaInput adalah isian form update data anggota. nomor angota dan tanggal bergabung tidak dapat diubah
type UpdateAnggotaInput struct {
	NamaLengkap string
	NoTelepon   string
}

type GetAnggota struct {
	Anggota       model.Anggota
	Saldo         model.SaldoSimpanan
	PinjamanAktif bool
}

func NewAnggotaService(db database.Sumber, anggotaRepo AnggotaRepo, simpananRepo SimpananRepo, pinjamanRepo PinjamanRepo, auditSvc *AuditService, simpananPokok int64, zona *time.Location, now func() time.Time) *AnggotaService {
	if now == nil {
		now = time.Now
	}

	if zona == nil {
		zona = time.UTC
	}

	return &AnggotaService{
		db:            db,
		anggotaRepo:   anggotaRepo,
		simpananRepo:  simpananRepo,
		pinjamanRepo:  pinjamanRepo,
		auditSvc:      auditSvc,
		simpananPokok: simpananPokok,
		zona:          zona,
		now:           now}
}

// DaftarAnggotaInput adalah isian form pendaftar anggota
type DaftarAnggotaInput struct {
	NomorAnggota     string
	NamaLengkap      string
	NoTelepon        string
	TanggalBergabung string // format YYYY-MM-DD; kosong = hari ini
}

// Register mendaftarkan anggota baru beserta setoran simpanan pokok
func (s *AnggotaService) Register(ctx context.Context, in DaftarAnggotaInput) (int64, error) {
	idn, ok := IdentitasDari(ctx)
	if !ok {
		return 0, ErrBelumLogin
	}

	a, err := s.validasi(in)
	if err != nil {
		return 0, err
	}

	var idAnggota int64
	err = s.db.DenganTx(ctx, func(tx database.DBTX) error {
		var err error
		idAnggota, err = s.anggotaRepo.Create(ctx, tx, a)
		if errors.Is(err, repository.ErrDuplikat) {
			return &ValidasiError{Pesan: map[string]string{"nomor_anggota": "sudah dipakai anggota lain"}}
		}
		if err != nil {
			fmt.Printf("error insert new anggota: %s\n", err)
			return err
		}

		idSimpanan, err := s.simpananRepo.Create(ctx, tx, model.TransaksiSimpanan{
			IDAnggota:        idAnggota,
			Kategori:         model.SimpananPokok,
			TipeMutasi:       model.MutasiSetor,
			Jumlah:           s.simpananPokok,
			TanggalTransaksi: a.TanggalBergabung,
			Keterangan:       "Simpanan pokok saat pendaftaran",
			DibuatOleh:       idn.IDPengguna,
		})

		if err != nil {
			fmt.Printf("error insert new simpanan: %s\n", err)
			return err
		}

		return s.auditSvc.NewEntri(ctx, tx, "daftar_anggota", "anggota", idAnggota, map[string]any{
			"nomor_anggota":  a.NomorAnggota,
			"id_simpanan":    idSimpanan,
			"simpanan_pokok": s.simpananPokok,
		})
	})

	if err != nil {
		var ve *ValidasiError
		if errors.As(err, &ve) {
			return 0, err
		}
		return 0, fmt.Errorf("register anggota: %w", err)
	}
	return idAnggota, nil
}

// GetByID mengembalikan satu anggota; ErrAnggotaTidakDitemukan bila ID tidak ada
func (s *AnggotaService) GetByID(ctx context.Context, id int64) (model.Anggota, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return model.Anggota{}, ErrBelumLogin
	}

	return s.findAnggota(ctx, s.db, id, false)
}

// GetAnggota mengembalikan data halaman detail anggota
func (s *AnggotaService) GetAnggota(ctx context.Context, id int64) (GetAnggota, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return GetAnggota{}, ErrBelumLogin
	}

	a, err := s.findAnggota(ctx, s.db, id, false)
	if err != nil {
		return GetAnggota{}, err
	}

	saldo, err := s.simpananRepo.Saldo(ctx, s.db, id)
	if err != nil {
		return GetAnggota{}, fmt.Errorf("GetAnggota %d: saldo: %w", id, err)
	}

	aktif, err := s.pinjamanRepo.AdaAktif(ctx, s.db, id)
	if err != nil {
		return GetAnggota{}, fmt.Errorf("GetAnggota %d: pinjaman aktif: %w", id, err)
	}

	return GetAnggota{
		Anggota:       a,
		Saldo:         saldo,
		PinjamanAktif: aktif,
	}, nil
}

// GetListFiltered mengembalikan daftar anggota yand difilter
func (s *AnggotaService) GetListFiltered(ctx context.Context, cari, status string, page, limit int) (PaginatedAnggotaResponse, error) {
	if _, ok := IdentitasDari(ctx); !ok {
		return PaginatedAnggotaResponse{}, ErrBelumLogin
	}

	listAnggota, total, err := s.anggotaRepo.ListFiltered(ctx, s.db, cari, status, page, limit)
	if err != nil {
		return PaginatedAnggotaResponse{}, err
	}

	return PaginatedAnggotaResponse{
		Data:     listAnggota,
		Total:    total,
		Page:     page,
		PageSize: limit,
	}, err
}

// Update hanya dapat mengubah nama dan telepon anggota.
// jika tidak ada yang berubah, maka audit tidak dicatat
func (s *AnggotaService) Update(ctx context.Context, id int64, in UpdateAnggotaInput) error {
	if _, ok := IdentitasDari(ctx); !ok {
		return ErrBelumLogin
	}

	nama, telepon := strings.TrimSpace(in.NamaLengkap), strings.TrimSpace(in.NoTelepon)

	pesan := map[string]string{}
	if nama == "" {
		pesan["nama_lengkap"] = "wajib diisi"
	} else if utf8.RuneCountInString(nama) > 100 {
		pesan["nama_lengkap"] = "maksimal 100 karakter"
	}

	if utf8.RuneCountInString(telepon) > 20 {
		pesan["no_telepon"] = "maksimal 20 karakter"
	}

	if len(pesan) > 0 {
		return &ValidasiError{Pesan: pesan}
	}

	err := s.db.DenganTx(ctx, func(tx database.DBTX) error {
		old, err := s.findAnggota(ctx, s.db, id, true)
		if err != nil {
			return err
		}

		changes := map[string]map[string]string{}
		if old.NamaLengkap != nama {
			changes["nama_lengkap"] = map[string]string{"dari": old.NamaLengkap, "menjadi": nama}
		}
		if old.NoTelepon != telepon {
			changes["no_telepon"] = map[string]string{"dari": old.NoTelepon, "menjadi": telepon}
		}

		if len(changes) == 0 {
			return nil
		}

		if err := s.anggotaRepo.Update(ctx, s.db, id, nama, telepon); err != nil {
			return fmt.Errorf("Save update anggota: %w", err)
		}

		return s.auditSvc.NewEntri(ctx, tx, "ubah_anggota", "anggota", id, map[string]any{
			"nomor_anggota": old.NomorAnggota,
			"perubahan":     changes,
		})
	})

	if err != nil {
		return fmt.Errorf("Update anggota %d: %w", id, err)
	}

	return nil

}

//

// HELPER //

// FormatTanggal mengembalikan tanggal bisnis hari ini(jam dibuang) pada zona bisnis
func (s *AnggotaService) FormatTanggal() time.Time {
	n := s.now().In(s.zona)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *AnggotaService) validasi(in DaftarAnggotaInput) (model.Anggota, error) {
	a := model.Anggota{
		NomorAnggota: strings.TrimSpace(in.NomorAnggota),
		NamaLengkap:  strings.TrimSpace(in.NamaLengkap),
		NoTelepon:    strings.TrimSpace(in.NoTelepon),
	}
	pesan := map[string]string{}

	if a.NomorAnggota == "" {
		pesan["nomor_anggota"] = "wajib diisi"
	} else if utf8.RuneCountInString(a.NomorAnggota) > 20 {
		pesan["nomor_anggota"] = "maksimal 20 karakter"
	}
	if a.NamaLengkap == "" {
		pesan["nama_lengkap"] = "wajib disisi"
	} else if utf8.RuneCountInString(a.NamaLengkap) > 100 {
		pesan["nama_lengkap"] = "maksimal 100 karakter"
	}

	if utf8.RuneCountInString(a.NoTelepon) > 20 {
		pesan["no_telepon"] = "maksimal 20 karakter"
	}

	d := s.FormatTanggal()
	tgl := strings.TrimSpace(in.TanggalBergabung)
	if tgl == "" {
		a.TanggalBergabung = d
	} else if t, err := time.Parse("2006-01-02", tgl); err != nil {
		pesan["tanggal_bergabung"] = "format tanggal tidak valid"
	} else if t.After(d) {
		pesan["tanggal_bergabung"] = "tidak boleh di masa depan"
	} else {
		a.TanggalBergabung = t
	}

	if len(pesan) > 0 {
		return model.Anggota{}, &ValidasiError{Pesan: pesan}
	}

	return a, nil
}

func (s *AnggotaService) findAnggota(ctx context.Context, db database.DBTX, id int64, kunci bool) (model.Anggota, error) {
	var (
		a   model.Anggota
		err error
	)

	if kunci {
		a, err = s.anggotaRepo.GetByIDForUpdate(ctx, db, id)
	} else {
		a, err = s.anggotaRepo.GetByID(ctx, db, id)
	}

	if errors.Is(err, repository.ErrTidakDitemukan) {
		return model.Anggota{}, ErrAnggotaTidakDitemukan
	}
	if err != nil {
		return model.Anggota{}, fmt.Errorf("findAnggota %d: %w", id, err)
	}

	return a, nil
}
