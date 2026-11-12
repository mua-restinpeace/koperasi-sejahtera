package model

import "time"

type Sesi struct {
	ID              int64
	IDPengguna      int64
	TokenHash       string
	CSRFToken       string
	DibuatPada      time.Time
	KedaluwarsaPada time.Time
}
