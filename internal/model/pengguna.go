package model

type Pengguna struct {
	ID           int64
	Username     string
	PasswordHash string
	NamaLengkap  string
	Aktif        string
}
