//go:build integration

// Tes integrasi: berjalan di MySQL sungguhan untuk hal-hal yang tidak dapat
// dibuktikan oleh tiruan (unique key, CAST saldo, penguncian baris).
//
//	KOPERASI_TEST_DSN='user:pass@tcp(127.0.0.1:3306)/koperasi_test?parseTime=true&loc=UTC' \
//	    go test -tags integration -race ./internal/service/
//
// PERHATIAN: seluruh tabel di database tersebut DIHAPUS lalu dibuat ulang dari
// migrations/*.up.sql. Karena itu nama database wajib berakhiran "_test".
package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

// bukaDBUji membuka database uji, mengosongkannya, dan menerapkan migrasi.
func bukaDBUji(t *testing.T) *sql.DB {
	t.Helper()

	dsn := os.Getenv("KOPERASI_TEST_DSN")
	if dsn == "" {
		t.Skip("KOPERASI_TEST_DSN belum diisi")
	}
	if !strings.Contains(dsn, "multiStatements=true") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		dsn += sep + "multiStatements=true"
	}

	ctx := context.Background()
	db, err := database.NewDB(ctx, dsn)
	if err != nil {
		t.Fatalf("buka database uji: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var nama string
	if err := db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&nama); err != nil || !strings.HasSuffix(nama, "_test") {
		t.Fatalf("nama database %q harus berakhiran _test (err=%v); tes menghapus seluruh tabel", nama, err)
	}

	// FOREIGN_KEY_CHECKS berlaku per koneksi, jadi pakai satu koneksi.
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rows, err := conn.QueryContext(ctx, "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()")
	if err != nil {
		t.Fatal(err)
	}
	var tabel []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		tabel = append(tabel, n)
	}
	rows.Close()

	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		t.Fatal(err)
	}
	for _, n := range tabel {
		if _, err := conn.ExecContext(ctx, "DROP TABLE `"+n+"`"); err != nil {
			t.Fatalf("drop %s: %v", n, err)
		}
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1"); err != nil {
		t.Fatal(err)
	}

	berkas, err := filepath.Glob("../../migrations/*.up.sql")
	if err != nil || len(berkas) == 0 {
		t.Fatalf("migrasi tidak ditemukan: %v", err)
	}
	sort.Strings(berkas)
	for _, b := range berkas {
		isi, err := os.ReadFile(b)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := conn.ExecContext(ctx, string(isi)); err != nil {
			t.Fatalf("migrasi %s: %v", b, err)
		}
	}

	if _, err := conn.ExecContext(ctx, "INSERT INTO pengguna (id, username, password_hash, nama_lengkap) VALUES (1, 'uji', 'x', 'Pengurus Uji')"); err != nil {
		t.Fatal(err)
	}

	return db
}

type lingkunganUji struct {
	db  *sql.DB
	svc *SimpananService
	sp  repository.SimpananRepo
}

func siapIntegrasi(t *testing.T) (lingkunganUji, context.Context) {
	t.Helper()
	db := bukaDBUji(t)
	now := func() time.Time { return time.Date(2026, 11, 11, 3, 0, 0, 0, time.UTC) }
	svc := NewSimpananService(database.Basis{DB: db}, repository.AnggotaRepo{}, repository.SimpananRepo{},
		NewAuditService(repository.AuditRepo{}), 100_000, time.UTC, now)

	return lingkunganUji{db: db, svc: svc}, DenganIdentitas(context.Background(), Identitas{IDPengguna: 1, Username: "uji"})
}

func (l lingkunganUji) anggota(t *testing.T, nomor string) int64 {
	t.Helper()
	id, err := repository.AnggotaRepo{}.Create(context.Background(), l.db, model.Anggota{
		NomorAnggota: nomor, NamaLengkap: "Uji " + nomor, Status: model.StatusAnggotaAktif,
		TanggalBergabung: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (l lingkunganUji) saldo(t *testing.T, id int64) model.SaldoSimpanan {
	t.Helper()
	s, err := l.sp.Saldo(context.Background(), l.db, id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (l lingkunganUji) hitungAudit(t *testing.T, aksi string) (n int) {
	t.Helper()
	if err := l.db.QueryRow("SELECT COUNT(*) FROM log_audit WHERE aksi = ?", aksi).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestIntegrasi_WajibSatuPerPeriode(t *testing.T) {
	l, ctx := siapIntegrasi(t)
	a, b := l.anggota(t, "A-1"), l.anggota(t, "A-2")

	if err := l.svc.SetorWajib(ctx, a, SetorWajibInput{Periode: "2026-10"}); err != nil {
		t.Fatal(err)
	}
	pesanDari(t, l.svc.SetorWajib(ctx, a, SetorWajibInput{Periode: "2026-10"}), "periode") // unique key
	if err := l.svc.SetorWajib(ctx, b, SetorWajibInput{Periode: "2026-10"}); err != nil {
		t.Fatalf("anggota lain, periode sama harus boleh: %v", err)
	}
	if got := l.saldo(t, a).Wajib; got != 100_000 {
		t.Errorf("saldo wajib = %d", got)
	}

	// Dibatalkan -> periode itu boleh dicatat ulang (koreksi = batalkan + catat ulang).
	var idTrx int64
	if err := l.db.QueryRow("SELECT id FROM transaksi_simpanan WHERE id_anggota = ? AND kategori = 'wajib'", a).Scan(&idTrx); err != nil {
		t.Fatal(err)
	}
	if _, err := l.svc.Batalkan(ctx, idTrx, "salah anggota"); err != nil {
		t.Fatal(err)
	}
	if got := l.saldo(t, a).Wajib; got != 0 {
		t.Errorf("setelah batal, saldo wajib = %d", got)
	}
	if err := l.svc.SetorWajib(ctx, a, SetorWajibInput{Periode: "2026-10"}); err != nil {
		t.Fatalf("catat ulang setelah batal: %v", err)
	}
	if got := l.saldo(t, a).Wajib; got != 100_000 {
		t.Errorf("setelah catat ulang, saldo wajib = %d", got)
	}
	if _, err := l.svc.Batalkan(ctx, idTrx, "lagi"); !errors.Is(err, ErrSudahDibatalkan) {
		t.Errorf("batal dua kali: %v", err)
	}

	r, err := l.svc.Riwayat(ctx, a)
	if err != nil || len(r.Transaksi) != 2 {
		t.Fatalf("riwayat harus memuat baris yang dibatalkan: %d err=%v", len(r.Transaksi), err)
	}
	if n := l.hitungAudit(t, "batal_simpanan"); n != 1 {
		t.Errorf("audit batal = %d", n)
	}
}

func TestIntegrasi_PokokHanyaSekali(t *testing.T) {
	l, _ := siapIntegrasi(t)
	a := l.anggota(t, "A-1")
	buat := func() error {
		_, err := l.sp.Create(context.Background(), l.db, model.TransaksiSimpanan{
			IDAnggota: a, Kategori: model.SimpananPokok, TipeMutasi: model.MutasiSetor, Jumlah: 500_000,
			TanggalTransaksi: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), DibuatOleh: 1,
		})
		return err
	}
	if err := buat(); err != nil {
		t.Fatal(err)
	}
	if err := buat(); !errors.Is(err, repository.ErrDuplikat) {
		t.Errorf("pokok kedua: %v", err)
	}
}

// Saldo memakai CAST AS SIGNED; tanpa itu MySQL melempar "BIGINT UNSIGNED value is out of range".
func TestIntegrasi_SaldoBolehNegatifTanpaError(t *testing.T) {
	l, _ := siapIntegrasi(t)
	a := l.anggota(t, "A-1")
	_, err := l.sp.Create(context.Background(), l.db, model.TransaksiSimpanan{
		IDAnggota: a, Kategori: model.SimpananSukarela, TipeMutasi: model.MutasiTarik, Jumlah: 500,
		TanggalTransaksi: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), DibuatOleh: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := l.saldo(t, a).Sukarela; got != -500 {
		t.Errorf("saldo = %d, ingin -500", got)
	}
}

func TestIntegrasi_TarikBersamaanTidakMelebihiSaldo(t *testing.T) {
	l, ctx := siapIntegrasi(t)
	a := l.anggota(t, "A-1")
	if err := l.svc.SetorSukarela(ctx, a, SukarelaInput{Jumlah: "100000"}); err != nil {
		t.Fatal(err)
	}

	const n = 8
	var wg sync.WaitGroup
	hasil := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hasil[i] = l.svc.TarikSukarela(ctx, a, SukarelaInput{Jumlah: "60000"})
		}()
	}
	wg.Wait()

	berhasil := 0
	for _, err := range hasil {
		var ve *ValidasiError
		switch {
		case err == nil:
			berhasil++
		case errors.As(err, &ve):
		default:
			t.Errorf("galat tak terduga: %v", err)
		}
	}
	if berhasil != 1 {
		t.Errorf("penarikan berhasil = %d, ingin 1", berhasil)
	}
	if got := l.saldo(t, a).Sukarela; got != 40_000 {
		t.Errorf("saldo = %d, ingin 40000", got)
	}
}

// Membatalkan setoran sambil menarik: apa pun urutannya, saldo tidak boleh negatif.
func TestIntegrasi_BatalSetorDanTarikBersamaan(t *testing.T) {
	for range 10 {
		l, ctx := siapIntegrasi(t)
		a := l.anggota(t, "A-1")
		if err := l.svc.SetorSukarela(ctx, a, SukarelaInput{Jumlah: "100000"}); err != nil {
			t.Fatal(err)
		}
		var idSetor int64
		if err := l.db.QueryRow("SELECT id FROM transaksi_simpanan WHERE id_anggota = ?", a).Scan(&idSetor); err != nil {
			t.Fatal(err)
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); _, _ = l.svc.Batalkan(ctx, idSetor, "uji") }()
		go func() { defer wg.Done(); _ = l.svc.TarikSukarela(ctx, a, SukarelaInput{Jumlah: "100000"}) }()
		wg.Wait()

		if got := l.saldo(t, a).Sukarela; got < 0 {
			t.Fatalf("saldo negatif: %d", got)
		}
	}
}
