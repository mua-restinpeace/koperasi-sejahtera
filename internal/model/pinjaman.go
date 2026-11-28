package model

import "time"

type Pinjaman struct {
	ID               int64
	IDAnggota        int64
	JumlahPinjaman   int64
	TenorBulan       int
	JasaPerBulanBps  int
	DendaPerHari     int64
	TanggalPencairan time.Time
	Status           string
	TanggalLunas     *time.Time
}
