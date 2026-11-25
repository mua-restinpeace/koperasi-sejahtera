package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
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
	GetByIDForUpdate(ctx context.Context, db database.DBTX, id int64) (model.TransaksiSimpanan, error)
	Batalkan(ctx context.Context, db database.DBTX, id, oleh int64, alasan string, pada time.Time) error
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

// SetorWajibInput adalah isian form setoran wajib; Periode berformat YYYY-MM.
type SetorWajibInput struct {
	Periode string
}

// SukarelaInput adalah isian form setor/tarik sukarela. Jumlah boleh memakai titik pemisah ribuan.
type SukarelaInput struct {
	Jumlah     string
	Keterangan string
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
func (s *SimpananService) Riwayat(ctx context.Context, idAnggota int64) (RiwayatSimpanan, error) {
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

// SetorWajib mencatat simpanan wajib satu bulan dengan jumlah tetap. Satu
// bulan satu setoran; bulan depan (bayar di muka) ditolak.
func (s *SimpananService) SetorWajib(ctx context.Context, idAnggota int64, in SetorWajibInput) error {
	return s.dalamTx(ctx, idAnggota, func(tx database.DBTX, a model.Anggota, idn Identitas) error {
		periode, err := time.Parse("2006-01", strings.TrimSpace(in.Periode))
		if err != nil {
			return pesanValidasi("periode", "Periode harus berupa bulan yang valid.")
		}

		hari := s.hariIni()
		bulanIni := time.Date(hari.Year(), hari.Month(), 1, 0, 0, 0, 0, time.UTC)
		bulanGabung := time.Date(a.TanggalBergabung.Year(), a.TanggalBergabung.Month(), 1, 0, 0, 0, 0, time.UTC)
		switch {
		case periode.After(bulanIni):
			return pesanValidasi("periode", "Periode tidak boleh di masa depan; simpanan wajib tidak dapat dibayar di muka.")
		case periode.Before(bulanGabung):
			return pesanValidasi("periode", "Periode tidak boleh sebelum bulan anggota bergabung.")
		}

		err = s.simpan(ctx, tx, idn, a, "setor_wajib", model.TransaksiSimpanan{
			Kategori:   model.SimpananWajib,
			TipeMutasi: model.MutasiSetor,
			Jumlah:     s.wajib,
			Periode:    &periode,
			Keterangan: "Simpanan wajib " + periode.Format("01-2006"),
		})
		if errors.Is(err, repository.ErrDuplikat) {
			return pesanValidasi("periode", "Simpanan wajib periode "+periode.Format("01-2006")+" sudah dibayar.")
		}
		return err
	})
}

// SetorSukarela mencatat setoran sukarela dengan jumlah bebas.
func (s *SimpananService) SetorSukarela(ctx context.Context, idAnggota int64, in SukarelaInput) error {
	return s.sukarela(ctx, idAnggota, in, model.MutasiSetor)
}

// TarikSukarela mencatat penarikan sukarela; tidak boleh melebihi saldo sukarela.
func (s *SimpananService) TarikSukarela(ctx context.Context, idAnggota int64, in SukarelaInput) error {
	return s.sukarela(ctx, idAnggota, in, model.MutasiTarik)
}

func (s *SimpananService) sukarela(ctx context.Context, idAnggota int64, in SukarelaInput, mutasi string) error {
	return s.dalamTx(ctx, idAnggota, func(tx database.DBTX, a model.Anggota, idn Identitas) error {
		jumlah, err := strconv.ParseInt(strings.ReplaceAll(strings.TrimSpace(in.Jumlah), ".", ""), 10, 64)
		if err != nil || jumlah <= 0 || jumlah > maksJumlahSimpanan {
			return pesanValidasi("jumlah", "Jumlah harus berupa angka rupiah lebih dari 0 dan paling banyak "+helper.FormatRupiah(maksJumlahSimpanan)+".")
		}
		ket := strings.TrimSpace(in.Keterangan)
		if utf8.RuneCountInString(ket) > 255 {
			return pesanValidasi("keterangan", "Keterangan maksimal 255 karakter.")
		}

		// Saldo dibaca setelah kunci anggota didapat, jadi tidak ada penarikan lain yang menyelip.
		if mutasi == model.MutasiTarik {
			saldo, err := s.simpananRepo.Saldo(ctx, tx, a.ID)
			if err != nil {
				return fmt.Errorf("saldo simpanan: %w", err)
			}
			if jumlah > saldo.Sukarela {
				return pesanValidasi("jumlah", "Penarikan melebihi saldo simpanan sukarela ("+helper.FormatRupiah(saldo.Sukarela)+").")
			}
		}

		return s.simpan(ctx, tx, idn, a, mutasi+"_sukarela", model.TransaksiSimpanan{
			Kategori:   model.SimpananSukarela,
			TipeMutasi: mutasi,
			Jumlah:     jumlah,
			Keterangan: ket,
		})
	})
}

// Simpanan pokok tidak dapat dibatalkan (belum ada cara mencatat ulang), dan
// setoran tidak dapat dibatalkan bila saldo kategorinya menjadi negatif.
func (s *SimpananService) Batalkan(ctx context.Context, idTransaksi int64, alasan string) (int64, error) {
	idn, ok := IdentitasDari(ctx)
	if !ok {
		return 0, ErrBelumLogin
	}

	alasan = strings.TrimSpace(alasan)
	if alasan == "" || utf8.RuneCountInString(alasan) > 255 {
		return 0, pesanValidasi("alasan", "Alasan pembatalan wajib diisi (maksimal 255 karakter).")
	}

	var idAnggota int64
	err := s.db.DenganTx(ctx, func(tx database.DBTX) error {
		// Urutan kunci: transaksi lalu anggota; keduanya dibaca dengan locking read
		// (lihat GetByIDForUpdate) sebelum ada pembacaan biasa.
		t, err := s.simpananRepo.GetByIDForUpdate(ctx, tx, idTransaksi)
		if errors.Is(err, repository.ErrTidakDitemukan) {
			return ErrTransaksiTidakDitemukan
		}
		if err != nil {
			return fmt.Errorf("baca transaksi simpanan: %w", err)
		}
		idAnggota = t.IDAnggota

		a, err := findAnggota(ctx, s.anggotaRepo, tx, t.IDAnggota, true)
		if err != nil {
			return err
		}

		switch {
		case a.Status != model.StatusAnggotaAktif:
			return ErrAnggotaSudahKeluar
		case t.DibatalkanPada != nil:
			return ErrSudahDibatalkan
		case t.Kategori == model.SimpananPokok:
			return ErrPokokTidakBisaDibatalkan
		}

		if t.TipeMutasi == model.MutasiSetor {
			saldo, err := s.simpananRepo.Saldo(ctx, tx, a.ID)
			if err != nil {
				return fmt.Errorf("saldo simpanan: %w", err)
			}
			sisa := saldo.Wajib
			if t.Kategori == model.SimpananSukarela {
				sisa = saldo.Sukarela
			}
			if sisa < t.Jumlah {
				return pesanValidasi("jumlah", "Setoran tidak dapat dibatalkan karena saldo simpanan "+t.Kategori+" ("+helper.FormatRupiah(sisa)+") akan menjadi negatif; batalkan penarikannya lebih dulu.")
			}
		}

		err = s.simpananRepo.Batalkan(ctx, tx, t.ID, idn.IDPengguna, alasan, s.now().UTC())
		if errors.Is(err, repository.ErrTidakDitemukan) {
			return ErrSudahDibatalkan
		}
		if err != nil {
			return fmt.Errorf("batalkan transaksi simpanan: %w", err)
		}

		return s.auditSvc.NewEntri(ctx, tx, "batal_simpanan", "transaksi_simpanan", t.ID, map[string]any{
			"id_anggota":    a.ID,
			"nomor_anggota": a.NomorAnggota,
			"kategori":      t.Kategori,
			"tipe_mutasi":   t.TipeMutasi,
			"jumlah":        t.Jumlah,
			"alasan":        alasan,
		})
	})

	return idAnggota, err
}

// HELPER //

func pesanValidasi(kolom, pesan string) *ValidasiError {
	return &ValidasiError{Pesan: map[string]string{kolom: pesan}}
}

func (s *SimpananService) hariIni() time.Time {
	n := s.now().In(s.zona)
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *SimpananService) dalamTx(ctx context.Context, idAnggota int64, fn func(tx database.DBTX, a model.Anggota, idn Identitas) error) error {
	idn, ok := IdentitasDari(ctx)
	if !ok {
		return ErrBelumLogin
	}

	return s.db.DenganTx(ctx, func(tx database.DBTX) error {
		a, err := findAnggota(ctx, s.anggotaRepo, tx, idAnggota, true)
		if err != nil {
			return err
		}
		if a.Status != model.StatusAnggotaAktif {
			return ErrAnggotaSudahKeluar
		}

		return fn(tx, a, idn)
	})
}

// simpan menulis transaksi beserta entri auditnya dalam transaksi yang sama.
func (s *SimpananService) simpan(ctx context.Context, tx database.DBTX, idn Identitas, a model.Anggota, aksi string, t model.TransaksiSimpanan) error {
	t.IDAnggota = a.ID
	t.TanggalTransaksi = s.hariIni()
	t.DibuatOleh = idn.IDPengguna

	id, err := s.simpananRepo.Create(ctx, tx, t)
	if err != nil {
		return err
	}

	return s.auditSvc.NewEntri(ctx, tx, aksi, "transaksi_simpanan", id, map[string]any{
		"id_anggota":    a.ID,
		"nomor_anggota": a.NomorAnggota,
		"kategori":      t.Kategori,
		"jumlah":        t.Jumlah,
	})
}
