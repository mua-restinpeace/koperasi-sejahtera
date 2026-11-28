package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/service"
)

type PinjamanService interface {
	AjukanPinjaman(ctx context.Context, in service.AjukanPinjamanInput) (int64, error)
	List(ctx context.Context, status string) ([]model.PinjamanListItem, error)
	Detail(ctx context.Context, id int64) (model.DetailPinjaman, error)
	InfoPengajuan(ctx context.Context, idAnggota int64) (int64, model.SaldoSimpanan, model.Anggota, error)
}

type dataDaftarPinjaman struct {
	Halaman
	Daftar       []model.PinjamanListItem
	FilterStatus string
}

type dataFormPinjaman struct {
	Halaman
	DaftarAnggota   []model.Anggota
	AnggotaTerpilih *model.Anggota
	BatasPinjaman   int64
	SaldoSimpanan   model.SaldoSimpanan
	TenorMin        int
	TenorMax        int
	JasaBps         int
	HariIni         time.Time
	Isian           struct {
		IDAnggota        int64
		Jumlah           string
		Tenor            int
		TanggalPencairan string
	}
	Error string
}

type dataDetailPinjaman struct {
	Halaman
	Detail model.DetailPinjaman
}

// DaftarPinjaman menampilkan daftar pinjaman anggota
func (h *Handler) DaftarPinjaman(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	list, err := h.svc.Pinjaman.List(r.Context(), status)
	if err != nil {
		h.internalError(w, r, err)
		return
	}

	data := dataDaftarPinjaman{
		Halaman:      h.halaman(r, "Daftar Pinjaman", "pinjaman"),
		Daftar:       list,
		FilterStatus: status,
	}

	if pesan := r.URL.Query().Get("berhasil"); pesan != "" {
		data.Pesan = pesan
	}

	h.render(w, http.StatusOK, "pages/pinjaman.html", data)
}

// FormPengajuan menampilkan formulir pengajuan pinjaman baru
func (h *Handler) FormPengajuan(w http.ResponseWriter, r *http.Request) {
	h.renderFormPengajuan(w, r, http.StatusOK, "")
}

func (h *Handler) renderFormPengajuan(w http.ResponseWriter, r *http.Request, httpStatus int, pesanError string) {
	// Ambil daftar anggota aktif untuk dropdown pilihan peminjam
	resAnggota, err := h.svc.Anggota.GetListFiltered(r.Context(), "", "aktif", 1, 200)
	if err != nil {
		h.internalError(w, r, err)
		return
	}

	var idAnggota int64
	if idStr := r.URL.Query().Get("id_anggota"); idStr != "" {
		idAnggota, _ = strconv.ParseInt(idStr, 10, 64)
	}
	if idAnggota == 0 && r.Method == http.MethodPost {
		idAnggota, _ = strconv.ParseInt(r.PostFormValue("id_anggota"), 10, 64)
	}

	data := dataFormPinjaman{
		Halaman:       h.halaman(r, "Pengajuan Pinjaman", "pinjaman"),
		DaftarAnggota: resAnggota.Data,
		TenorMin:      h.cfg.TenorMin,
		TenorMax:      h.cfg.TenorMax,
		JasaBps:       h.cfg.JasaBps,
		HariIni:       time.Now(),
		Error:         pesanError,
	}

	data.Isian.IDAnggota = idAnggota
	if r.Method == http.MethodPost {
		data.Isian.Jumlah = r.PostFormValue("jumlah")
		data.Isian.Tenor, _ = strconv.Atoi(r.PostFormValue("tenor"))
		data.Isian.TanggalPencairan = r.PostFormValue("tanggal_pencairan")
	} else {
		data.Isian.Tenor = h.cfg.TenorMin
		data.Isian.TanggalPencairan = time.Now().Format("2006-01-02")
	}

	// Jika ada anggota yang dipilih, ambil info batas dan profilnya
	if idAnggota > 0 {
		batas, saldo, agt, err := h.svc.Pinjaman.InfoPengajuan(r.Context(), idAnggota)
		if err == nil {
			data.AnggotaTerpilih = &agt
			data.BatasPinjaman = batas
			data.SaldoSimpanan = saldo
		}
	}

	h.render(w, httpStatus, "pages/pinjaman_form.html", data)
}

// SimpanPinjaman memproses pengajuan pinjaman
func (h *Handler) SimpanPinjaman(w http.ResponseWriter, r *http.Request) {
	idAnggota, _ := strconv.ParseInt(r.PostFormValue("id_anggota"), 10, 64)
	tenor, _ := strconv.Atoi(r.PostFormValue("tenor"))
	jumlah := r.PostFormValue("jumlah")
	tglPencairan := r.PostFormValue("tanggal_pencairan")

	in := service.AjukanPinjamanInput{
		IDAnggota:        idAnggota,
		Jumlah:           jumlah,
		Tenor:            tenor,
		TanggalPencairan: tglPencairan,
	}

	idPinjaman, err := h.svc.Pinjaman.AjukanPinjaman(r.Context(), in)
	if err != nil {
		var pesan string
		switch {
		case errors.Is(err, service.ErrBelumLogin):
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		case errors.Is(err, service.ErrAnggotaTidakDitemukan):
			pesan = "Anggota tidak ditemukan."
		case errors.Is(err, service.ErrBukanAnggotaAktif):
			pesan = "Anggota sudah tidak berstatus aktif."
		case errors.Is(err, service.ErrAdaPinjamanAktif):
			pesan = "Anggota ini masih memiliki pinjaman aktif yang belum lunas."
		case errors.Is(err, service.ErrJumlahMelebihiBatas):
			pesan = "Jumlah pinjaman melebihi batas maksimal (3x total simpanan pokok dan wajib)."
		case errors.Is(err, service.ErrTenorDiLuarJangkauan):
			pesan = fmt.Sprintf("Tenor harus antara %d sampai %d bulan.", h.cfg.TenorMin, h.cfg.TenorMax)
		case errors.Is(err, service.ErrJumlahPinjamanTidakSah):
			pesan = "Nominal pinjaman tidak sah atau harus lebih dari 0."
		default:
			pesan = fmt.Sprintf("Gagal mengajukan pinjaman: %v", err)
		}

		h.renderFormPengajuan(w, r, http.StatusUnprocessableEntity, pesan)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/pinjaman/%d?berhasil=Pinjaman+berhasil+dicairkan", idPinjaman), http.StatusSeeOther)
}

// DetailPinjaman menampilkan detail pinjaman dan jadwal angsuran
func (h *Handler) DetailPinjaman(w http.ResponseWriter, r *http.Request) {
	id, ok := idDariPath(r)
	if !ok {
		h.notFound(w)
		return
	}

	detail, err := h.svc.Pinjaman.Detail(r.Context(), id)
	if err != nil {
		if errors.Is(err, service.ErrPinjamanTidakDitemukan) {
			h.notFound(w)
			return
		}
		h.internalError(w, r, err)
		return
	}

	data := dataDetailPinjaman{
		Halaman: h.halaman(r, fmt.Sprintf("Pinjaman #%d - %s", detail.ID, detail.NamaAnggota), "pinjaman"),
		Detail:  detail,
	}

	if pesan := r.URL.Query().Get("berhasil"); pesan != "" {
		data.Pesan = pesan
	}

	h.render(w, http.StatusOK, "pages/pinjaman_detail.html", data)
}
