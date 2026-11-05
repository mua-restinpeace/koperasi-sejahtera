package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// config server
	Alamat string // alamat listen HTTP, contoh ":8080"
	DSN    string // DSN MySQL, contoh "user:pass@tcp(127.0.0.1:3306)/koperasi?parseTime=true&loc=UTC"

	// config koperasi
	SimpananPokok        int64 // dibayar satu kali saat mendaftar
	SimpananWajib        int64 // dibayar setiap bulan
	JasaBps              int   // jasa per bulan dalam basis poin, 100 = 1% (flat)
	DendaPerHari         int64 // denda per hari per angsuran yang belum lunas
	BiayaAdminBps        int   // biaya administrasi pelunasan dipercepat, 200 = 2%
	KelipatanBatasPinjam int   // batas pinjaman = kelipatan x (pokok + wajib)
	TenorMin             int   // bulan
	TenorMax             int   // bulan
}

func Default() Config {
	return Config{
		Alamat:               ":8080",
		SimpananPokok:        500_000,
		SimpananWajib:        100_000,
		JasaBps:              100,
		DendaPerHari:         2_000,
		BiayaAdminBps:        200,
		KelipatanBatasPinjam: 3,
		TenorMin:             6,
		TenorMax:             24,
	}
}

// membaca konfigurasi dari .env, jika tidak diisi memakai nilai bawaan Default
func Load() (Config, error) {
	c := Default()
	c.Alamat = envString("KOPERASI_ALAMAT", c.Alamat)
	c.DSN = envString("KOPERASI_DSN", c.DSN)

	var err error
	set := func(dst *int64, key string) {
		if err == nil {
			*dst, err = envInt64(key, *dst)
		}
	}

	setInt := func(dst *int, key string) {
		if err == nil {
			var v int64
			v, err = envInt64(key, int64(*dst))
			*dst = int(v)
		}
	}

	set(&c.SimpananPokok, "KOPERASI_SIMPANAN_POKOK")
	set(&c.SimpananWajib, "KOPERASI_SIMPANAN_WAJIB")
	set(&c.DendaPerHari, "KOPERASI_DENDA_PER_HARI")
	setInt(&c.JasaBps, "KOPERASI_JASA_BPS")
	setInt(&c.BiayaAdminBps, "KOPERASI_BIAYA_ADMIN_BPS")
	setInt(&c.KelipatanBatasPinjam, "KOPERASI_KELIPATAN_BATAS_PINJAM")
	setInt(&c.TenorMin, "KOPERASI_TENOR_MIN")
	setInt(&c.TenorMax, "KOPERASI_TENOR_MAX")

	if err != nil {
		return Config{}, err
	}

	return c, c.Validasi()
}

func (c Config) Validasi() error {
	switch {
	case c.SimpananPokok <= 0:
		return fmt.Errorf("config: simpanan pokok harus lebih dari 0")
	case c.SimpananWajib <= 0:
		return fmt.Errorf("config: simpanan wajib harus lebih dari 0")
	case c.JasaBps < 0, c.BiayaAdminBps < 0, c.DendaPerHari < 0:
		return fmt.Errorf("config: jasa, biaya administrasi, dan denda tidak boleh negatif")
	case c.KelipatanBatasPinjam <= 0:
		return fmt.Errorf("config: kelipatan batas pinjaman harus lebih dari 0")
	case c.TenorMin <= 0 || c.TenorMax < c.TenorMin:
		return fmt.Errorf("config: rentang tenor tidak valid (%d..%d)", c.TenorMin, c.TenorMax)
	}
	return nil
}

func envString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}

	return def
}

func envInt64(key string, def int64) (int64, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def, nil
	}

	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("config: %s harus berupa bilangan bulat: %w", key, err)
	}

	return n, nil
}
