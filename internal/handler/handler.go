package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/config"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/helper"
)

type Handler struct {
	cfg  config.Config
	log  *slog.Logger
	tmpl map[string]*template.Template
	svc  Service
}

type Service struct {
	Auth     AuthService
	Anggota  AnggotaService
	Simpanan SimpananService
	Laporan  LaporanService
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

	funcs := template.FuncMap{
		"rupiah":  helper.FormatRupiah,
		"inisial": helper.Inisial,
	}

	tmpl := make(map[string]*template.Template, len(pages))
	for _, nama := range pages {
		layout := "layout/layout.html"

		// Login uses the authentication layout.
		if nama == "pages/login.html" {
			layout = "layout/auth.html"
		}

		t, err := template.New(path.Base(layout)).Funcs(funcs).ParseFS(
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

// galat mencatat kesalahan tak terduga dan menampilkan pesan umum ke pengguna.
func (h *Handler) error(w http.ResponseWriter, r *http.Request, err error) {
	h.log.Error("handler gagal", "method", r.Method, "path", r.URL.Path, "error", err)
	http.Error(w, "Terjadi kesalahan pada server", http.StatusInternalServerError)
}
