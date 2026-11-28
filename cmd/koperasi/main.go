package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/joho/godotenv/autoload"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/config"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/handler"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/router"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
	"github.com/mua-restinpeace/koperasi-sejahtera/templates"
)

const zonaBisnis = "Asia/Jakarta"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(log); err != nil {
		log.Error("aplikasi berhenti", "error", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	zona, err := time.LoadLocation(zonaBisnis)
	if err != nil {
		return fmt.Errorf("error memuat zona waktu: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := database.NewDB(ctx, cfg.DSN)
	if err != nil {
		return err
	}

	defer db.Close()
	sumber := database.Basis{DB: db}

	// Repository tidak menyimpan state
	// service menerima keduanya lewat interface
	auditSvc := service.NewAuditService(repository.AuditRepo{})
	// Aturan perhitungan pinjaman dan pembayaran.
	aturan := helper.AturanHitung{DendaPerHari: cfg.DendaPerHari, BiayaAdminBps: cfg.BiayaAdminBps}

	// Repository yang digunakan untuk pembayaran.
	pinjamanRepo := repository.PinjamanRepo{}
	pembayaranRepo := repository.PembayaranRepo{}
	angsuranRepo := repository.AngsuranRepo{}
	alokasiRepo := repository.NewAlokasiRepository()

	// Service untuk menghitung ulang alokasi pembayaran.
	replaySvc := service.NewReplayAlokasiService(sumber, pinjamanRepo, angsuranRepo, pembayaranRepo, alokasiRepo, aturan)

	// Service pembayaran, pelunasan, dan pembatalan.
	pembayaranSvc := service.NewPembayaranService(sumber, pinjamanRepo, pembayaranRepo, angsuranRepo, replaySvc, auditSvc, aturan, cfg.BiayaAdminBps, zona, nil)
	authService := service.NewAuthService(sumber, repository.PenggunaRepo{}, repository.SesiRepo{}, service.HasherPBKDF2{}, time.Duration(cfg.SesiJam)*time.Hour, nil, log)
	anggotaSvc := service.NewAnggotaService(sumber, repository.AnggotaRepo{}, repository.SimpananRepo{}, repository.PinjamanRepo{}, auditSvc, cfg.SimpananPokok, zona, nil)
	simpananSvc := service.NewSimpananService(sumber, repository.AnggotaRepo{}, repository.SimpananRepo{}, auditSvc, cfg.SimpananWajib, zona, nil)

	h, err := handler.NewHandler(cfg, log, templates.FS, handler.Service{
		Auth:       authService,
		Anggota:    anggotaSvc,
		Simpanan:   simpananSvc,
		Pembayaran: pembayaranSvc,
	})

	if err != nil {
		return err
	}

	httpSrv := &http.Server{
		Addr:              cfg.Alamat,
		Handler:           router.NewRouter(h, authService, log),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("server berjalan", "alamat", cfg.Alamat)
		errCh <- httpSrv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		log.Info("menerima sinyal berhenti, mematikan server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
