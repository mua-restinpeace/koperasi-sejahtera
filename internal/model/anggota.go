package model

import "time"

// status keanggotaan
const (
	StatusAnggotaAktif  = "aktif"
	StatusAnggotaKeluar = "keluar"
)

type Anggota struct {
	ID               int64
	NomorAnggota     string
	NamaLengkap      string
	NoTelepon        string
	TanggalBergabung time.Time
	Status           string
	TanggalKeluar    *time.Time
}
