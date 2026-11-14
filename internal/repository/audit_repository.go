package repository

import (
	"context"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type AuditRepo struct{}

// Create menambahkan entri ke audit log
func (AuditRepo) Create(ctx context.Context, db database.DBTX, e model.Audit) error {
	var detail any
	if len(e.Detail) > 0 {
		detail = string(e.Detail)
	}

	query := `INSERT INTO log_audit (id_pengguna, aksi, entitas, id_entitas, detail, dibuat_pada)
	VALUES (?, ?, ?, ?, ?, UTC_TIMESTAMP())`

	_, err := db.ExecContext(ctx, query, e.IDPengguna, e.Aksi, e.Entitas, e.IDEntitas, detail)

	return petakanError(err, "new entri audit")
}
