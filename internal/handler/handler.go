package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/config"
)

type Handler struct {
	cfg  config.Config
	log  *slog.Logger
	tmpl map[string]*template.Template
	svc  Service
}

// New membuat Handler dan memuat seluruh template dari templatesFS, yang
// berisi layout.html dan berkas halaman di akarnya (lihat package templates).
func NewHandler(cfg config.Config, log *slog.Logger, templateFS fs.FS, svc Service) (*Handler, error) {
	var pages []string

	err := fs.WalkDir(templateFS, "pages", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(path, ".html") {
			pages = append(pages, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("handler: error membaca daftar template: %w", err)
	}

	tmpl := make(map[string]*template.Template, len(pages))
	for _, nama := range pages {
		layout := "layout/layout.html"

		// Login uses the authentication layout.
		if nama == "pages/login.html" {
			layout = "layout/auth.html"
		}

		t, err := template.ParseFS(
			templateFS,
			layout,
			"partials/*.html",
			nama,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"handler: error memuat template %s: %w",
				nama,
				err,
			)
		}

		tmpl[nama] = t
	}

	return &Handler{cfg: cfg, log: log, tmpl: tmpl, svc: svc}, nil
}
