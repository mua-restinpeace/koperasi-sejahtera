package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DBTX adalah bagian dari *sql.DB dan *sql.Tx yang dipakai repository, sehingga
// fungsi repository yang sama dapat dijalankan di dalam maupun di luar transaksi.
type DBTX interface {
}

// Sumber adalah sumber data yang dapat dipakai langsung (sebagai DBTX) atau
// untuk menjalankan beberapa langkah dalam satu transaksi. Basis mengimplementasikannya;
// tes service memakai tiruan.
type Sumber interface {
	DBTX
	DenganTx(ctx context.Context, fn func(tx DBTX) error) error
}

// Basis membungkus *sql.DB dan mengimplementasikan Sumber.
type Basis struct {
	*sql.DB
}

// DenganTx menjalankan fn di dalam satu transaksi. Bila fn mengembalikan
// kesalahan (atau panik), transaksi dibatalkan; bila tidak, di-commit.
func (b Basis) DenganTx(ctx context.Context, fn func(tx DBTX) error) error {
	tx, err := b.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("database: error mulai transaksi: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback()
			panic(p)
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("database: rollback: %w", rbErr))
		}

		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("database: error commit: %w", err)
	}

	return nil
}

// NewDB membuka koneksi MySQL dan memastikan server dapat dijangkau.
// DSN wajib memuat parseTime=true agar kolom DATE dan DATETIME terbaca sebagai time.Time.
func NewDB(ctx context.Context, dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, errors.New("database: KOPERASI_DSN belum diisi")
	}

	if !strings.Contains(dsn, "parseTime=true") {
		return nil, errors.New("database: DSN harus memuat parseTime=true (contoh: user:pass@tcp(127.0.0.1:3306)/koperasi?parseTime=true&loc=UTC)")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("database: error buka koneksi: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("database: tidak bisa terhubung ke MySQL: %w", err)
	}

	return db, nil
}
