package model

type Audit struct {
	IDPengguna int64
	Aksi       string // contoh: "register anggota"
	Entitas    string // contoh: "anggota"
	IDEntitas  int64
	Detail     []byte
}
