package repository

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-sql-driver/mysql"
)

var (
	ErrTidakDitemukan = errors.New("repository: data tidak ditemukan")
	ErrDuplikat       = errors.New("repository: data sudah ada")
)

// isDuplikat mengenali kesalahan MySQL 1062 (duplicate entry)
func isDuplikat(err error) bool {
	var mysqlError *mysql.MySQLError

	return errors.As(err, &mysqlError) && mysqlError.Number == 1062
}

// petakanError mengubah sql.ErrNoRows menjadi ErrTidakDitemukan dan membungkus error lain
func petakanError(err error, konteks string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return ErrTidakDitemukan
	default:
		return fmt.Errorf("repository: %s error. %w", konteks, err)
	}
}
