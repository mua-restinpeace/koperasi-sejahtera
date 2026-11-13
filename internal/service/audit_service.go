package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
)

type AuditRepo interface {
	Create(ctx context.Context, db database.DBTX, e model.Audit) error
}

type AuditService struct {
	repo AuditRepo
}

func NewAuditService(repo AuditRepo) *AuditService {
	return &AuditService{repo: repo}
}

// NewEntri menulis entri audit memakai db
// Pengurus diambil dari context, detail lain boleh nil
func (s *AuditService) NewEntri(ctx context.Context, db database.DBTX, aksi, entitas string, idEntitas int64, detail any) error {
	id, ok := IdentitasDari(ctx)
	if !ok {
		return ErrBelumLogin
	}
	var raw []byte
	if detail != nil {
		var err error
		if raw, err = json.Marshal(detail); err != nil {
			return fmt.Errorf("audit: error encode detail. %w", err)
		}
	}

	return s.repo.Create(ctx, db, model.Audit{
		IDPengguna: id.IDPengguna,
		Aksi:       aksi,
		Entitas:    entitas,
		IDEntitas:  idEntitas,
		Detail:     raw,
	})
}
