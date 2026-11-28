//go:build integration

package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

func siapPembayaranIntegrasi(t *testing.T) (*sql.DB, *PembayaranService, context.Context, int64) {
	t.Helper()

	db := bukaDBUji(t)
	ctx := DenganIdentitas(context.Background(), Identitas{
		IDPengguna: 1,
		Username:   "uji",
	})

	// Buat anggota uji.
	idAnggota, err := repository.AnggotaRepo{}.Create(ctx, db, model.Anggota{
		NomorAnggota:     "P-001",
		NamaLengkap:      "Anggota Uji Pembayaran",
		Status:           model.StatusAnggotaAktif,
		TanggalBergabung: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("buat anggota: %v", err)
	}

	// Buat pinjaman aktif Rp6.000.000 dengan tenor 6 bulan.
	result, err := db.ExecContext(ctx, `
        INSERT INTO pinjaman (
            id_anggota, jumlah_pinjaman, tenor_bulan,
            jasa_per_bulan_bps, denda_per_hari,
            tanggal_pencairan, status, dibuat_oleh
        ) VALUES (?, 6000000, 6, 100, 2000, '2026-11-11', 'aktif', 1)
    `, idAnggota)
	if err != nil {
		t.Fatalf("buat pinjaman: %v", err)
	}

	idPinjaman, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("ambil ID pinjaman: %v", err)
	}

	// Buat enam jadwal angsuran.
	for i := 0; i < 6; i++ {
		jatuhTempo := time.Date(
			2026,
			time.December+time.Month(i),
			11, 0, 0, 0, 0, time.UTC,
		)

		_, err := db.ExecContext(ctx, `
            INSERT INTO angsuran (
                id_pinjaman, urutan, tanggal_jatuh_tempo,
                pokok_tagihan, jasa_tagihan
            ) VALUES (?, ?, ?, 1000000, 60000)
        `, idPinjaman, i+1, jatuhTempo)
		if err != nil {
			t.Fatalf("buat angsuran ke-%d: %v", i+1, err)
		}
	}

	aturan := helper.AturanHitung{
		DendaPerHari:  2000,
		BiayaAdminBps: 100,
	}

	pinjamanRepo := repository.PinjamanRepo{}
	pembayaranRepo := repository.PembayaranRepo{}
	angsuranRepo := repository.AngsuranRepo{}
	alokasiRepo := repository.NewAlokasiRepository()

	dbSumber := database.Basis{DB: db}
	auditSvc := NewAuditService(repository.AuditRepo{})

	replaySvc := NewReplayAlokasiService(
		dbSumber,
		pinjamanRepo,
		angsuranRepo,
		pembayaranRepo,
		alokasiRepo,
		aturan,
	)

	now := func() time.Time {
		return time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC)
	}

	svc := NewPembayaranService(
		dbSumber,
		pinjamanRepo,
		pembayaranRepo,
		angsuranRepo,
		replaySvc,
		auditSvc,
		aturan,
		100,
		time.UTC,
		now,
	)

	t.Cleanup(func() {
		_ = db.Close()
	})

	return db, svc, ctx, idPinjaman
}

func TestIntegrasi_CatatPembayaranAngsuran(t *testing.T) {
	db, svc, ctx, idPinjaman := siapPembayaranIntegrasi(t)

	tanggalBayar := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

	idPembayaran, err := svc.CatatPembayaran(ctx, idPinjaman, CatatPembayaranInput{
		TanggalBayar: tanggalBayar,
		Jumlah:       1060000,
		Keterangan:   "Pembayaran angsuran pertama",
	})
	if err != nil {
		t.Fatalf("CatatPembayaran: %v", err)
	}

	// Pastikan pembayaran tersimpan dengan nominal yang benar.
	var jumlah int64
	err = db.QueryRow(`
        SELECT jumlah_diterima
        FROM pembayaran_pinjaman
        WHERE id = ? AND id_pinjaman = ? AND jenis = 'angsuran'
    `, idPembayaran, idPinjaman).Scan(&jumlah)
	if err != nil {
		t.Fatalf("baca pembayaran: %v", err)
	}
	if jumlah != 1060000 {
		t.Errorf("jumlah pembayaran = %d, ingin 1060000", jumlah)
	}

	// Pastikan pembayaran dialokasikan ke pokok dan jasa.
	var pokok, jasa int64
	err = db.QueryRow(`
        SELECT
            COALESCE(SUM(pokok_dibayar), 0),
            COALESCE(SUM(jasa_dibayar), 0)
        FROM alokasi_pembayaran
        WHERE id_pembayaran = ?
    `, idPembayaran).Scan(&pokok, &jasa)
	if err != nil {
		t.Fatalf("baca alokasi pembayaran: %v", err)
	}

	if pokok != 1000000 {
		t.Errorf("alokasi pokok = %d, ingin 1000000", pokok)
	}
	if jasa != 60000 {
		t.Errorf("alokasi jasa = %d, ingin 60000", jasa)
	}
}

// TestIntegrasi_CatatPelunasan menguji pencatatan pelunasan pinjaman.
func TestIntegrasi_CatatPelunasan(t *testing.T) {
	db, svc, ctx, idPinjaman := siapPembayaranIntegrasi(t)

	tanggalBayar := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

	hasil, err := svc.CatatPelunasan(ctx, idPinjaman, CatatPelunasanInput{
		TanggalBayar: tanggalBayar,
		Keterangan:   "Pelunasan pinjaman",
	})
	if err != nil {
		t.Fatalf("CatatPelunasan: %v", err)
	}

	// Pastikan pembayaran pelunasan tersimpan dengan benar.
	var jenis string
	var jumlah, biayaAdmin int64

	err = db.QueryRow(`
                SELECT jenis, jumlah_diterima, biaya_administrasi
                FROM pembayaran_pinjaman
                WHERE id = ? AND id_pinjaman = ?
        `, hasil.IDPembayaran, idPinjaman).Scan(&jenis, &jumlah, &biayaAdmin)
	if err != nil {
		t.Fatalf("baca pembayaran pelunasan: %v", err)
	}

	if jenis != "pelunasan" {
		t.Errorf("jenis pembayaran = %q, ingin pelunasan", jenis)
	}
	if jumlah != 6120000 {
		t.Errorf("jumlah pelunasan = %d, ingin 6120000", jumlah)
	}
	if biayaAdmin != 60000 {
		t.Errorf("biaya administrasi = %d, ingin 60000", biayaAdmin)
	}

	// Pastikan seluruh pokok dan jasa yang wajib dibayar dialokasikan.
	var pokok, jasa int64
	err = db.QueryRow(`
                SELECT
                        COALESCE(SUM(pokok_dibayar), 0),
                        COALESCE(SUM(jasa_dibayar), 0)
                FROM alokasi_pembayaran
                WHERE id_pembayaran = ?
        `, hasil.IDPembayaran).Scan(&pokok, &jasa)
	if err != nil {
		t.Fatalf("baca alokasi pelunasan: %v", err)
	}

	if pokok != 6000000 {
		t.Errorf("alokasi pokok = %d, ingin 6000000", pokok)
	}
	if jasa != 60000 {
		t.Errorf("alokasi jasa = %d, ingin 60000", jasa)
	}

	// Pastikan status pinjaman berubah menjadi lunas.
	var status string
	err = db.QueryRow(`
                SELECT status
                FROM pinjaman
                WHERE id = ?
        `, idPinjaman).Scan(&status)
	if err != nil {
		t.Fatalf("baca status pinjaman: %v", err)
	}

	if status != "lunas" {
		t.Errorf("status pinjaman = %q, ingin lunas", status)
	}
}

// TestIntegrasi_BatalkanPembayaran menguji pembatalan pembayaran angsuran.
func TestIntegrasi_BatalkanPembayaran(t *testing.T) {
	db, svc, ctx, idPinjaman := siapPembayaranIntegrasi(t)

	tanggalBayar := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

	idPembayaran, err := svc.CatatPembayaran(ctx, idPinjaman, CatatPembayaranInput{
		TanggalBayar: tanggalBayar,
		Jumlah:       1060000,
		Keterangan:   "Pembayaran yang akan dibatalkan",
	})
	if err != nil {
		t.Fatalf("CatatPembayaran: %v", err)
	}

	// Batalkan pembayaran.
	idHasil, err := svc.BatalkanPembayaran(
		ctx,
		idPembayaran,
		"Kesalahan pencatatan pembayaran",
	)
	if err != nil {
		t.Fatalf("BatalkanPembayaran: %v", err)
	}

	if idHasil != idPinjaman {
		t.Errorf("ID pinjaman hasil = %d, ingin %d", idHasil, idPinjaman)
	}

	// Pastikan pembayaran memiliki status dibatalkan.
	var dibatalkanPada sql.NullTime
	var alasan string

	err = db.QueryRow(`
                SELECT dibatalkan_pada, alasan_pembatalan
                FROM pembayaran_pinjaman
                WHERE id = ?
        `, idPembayaran).Scan(&dibatalkanPada, &alasan)
	if err != nil {
		t.Fatalf("baca status pembatalan: %v", err)
	}

	if !dibatalkanPada.Valid {
		t.Error("pembayaran seharusnya sudah dibatalkan")
	}
	if alasan != "Kesalahan pencatatan pembayaran" {
		t.Errorf("alasan pembatalan = %q, tidak sesuai", alasan)
	}

	// Pastikan alokasi pembayaran lama sudah dihapus melalui replay.
	var jumlahAlokasi int
	err = db.QueryRow(`
                SELECT COUNT(*)
                FROM alokasi_pembayaran
                WHERE id_pembayaran = ?
        `, idPembayaran).Scan(&jumlahAlokasi)
	if err != nil {
		t.Fatalf("hitung alokasi pembayaran: %v", err)
	}

	if jumlahAlokasi != 0 {
		t.Errorf("jumlah alokasi pembayaran = %d, ingin 0", jumlahAlokasi)
	}

	// Pastikan pinjaman kembali berstatus aktif.
	var status string
	err = db.QueryRow(`
                SELECT status
                FROM pinjaman
                WHERE id = ?
        `, idPinjaman).Scan(&status)
	if err != nil {
		t.Fatalf("baca status pinjaman: %v", err)
	}

	if status != "aktif" {
		t.Errorf("status pinjaman = %q, ingin aktif", status)
	}
}

// TestIntegrasi_BatalkanPembayaranDuaKali memastikan pembatalan ganda ditolak.
func TestIntegrasi_BatalkanPembayaranDuaKali(t *testing.T) {
	_, svc, ctx, idPinjaman := siapPembayaranIntegrasi(t)

	tanggalBayar := time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC)

	idPembayaran, err := svc.CatatPembayaran(ctx, idPinjaman, CatatPembayaranInput{
		TanggalBayar: tanggalBayar,
		Jumlah:       1060000,
		Keterangan:   "Pembayaran untuk tes pembatalan ganda",
	})
	if err != nil {
		t.Fatalf("CatatPembayaran: %v", err)
	}

	// Pembatalan pertama harus berhasil.
	_, err = svc.BatalkanPembayaran(
		ctx,
		idPembayaran,
		"Pembatalan pertama",
	)
	if err != nil {
		t.Fatalf("pembatalan pertama: %v", err)
	}

	// Pembatalan kedua harus ditolak.
	_, err = svc.BatalkanPembayaran(
		ctx,
		idPembayaran,
		"Pembatalan kedua",
	)
	if !errors.Is(err, ErrPembayaranSudahDibatalkan) {
		t.Errorf(
			"pembatalan kedua menghasilkan error %v, ingin ErrPembayaranSudahDibatalkan",
			err,
		)
	}
}
