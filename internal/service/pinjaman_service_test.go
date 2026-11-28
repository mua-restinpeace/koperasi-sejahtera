package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mua-restinpeace/koperasi-sejahtera/internal/database"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/model"
	"github.com/mua-restinpeace/koperasi-sejahtera/internal/repository"
)

type fakePinjamanStore struct {
	adaAktif    bool
	pinjamanMap map[int64]model.PinjamanListItem
	jadwalMap   map[int64][]model.Angsuran
	nextID      int64
}

func newFakePinjamanStore() *fakePinjamanStore {
	return &fakePinjamanStore{
		pinjamanMap: make(map[int64]model.PinjamanListItem),
		jadwalMap:   make(map[int64][]model.Angsuran),
		nextID:      1,
	}
}

func (f *fakePinjamanStore) AdaAktif(context.Context, database.DBTX, int64) (bool, error) {
	return f.adaAktif, nil
}

func (f *fakePinjamanStore) Create(_ context.Context, _ database.DBTX, p model.Pinjaman, jadwal []model.Angsuran) (int64, error) {
	id := f.nextID
	f.nextID++
	p.ID = id
	f.pinjamanMap[id] = model.PinjamanListItem{Pinjaman: p}
	f.jadwalMap[id] = jadwal
	return id, nil
}

func (f *fakePinjamanStore) GetByID(_ context.Context, _ database.DBTX, id int64) (model.PinjamanListItem, error) {
	item, ok := f.pinjamanMap[id]
	if !ok {
		return model.PinjamanListItem{}, repository.ErrTidakDitemukan
	}
	return item, nil
}

func (f *fakePinjamanStore) List(_ context.Context, _ database.DBTX, status string) ([]model.PinjamanListItem, error) {
	var hasil []model.PinjamanListItem
	for _, p := range f.pinjamanMap {
		if status == "" || p.Status == status {
			hasil = append(hasil, p)
		}
	}
	return hasil, nil
}

func (f *fakePinjamanStore) GetJadwalByPinjamanID(_ context.Context, _ database.DBTX, idPinjaman int64) ([]model.Angsuran, error) {
	return f.jadwalMap[idPinjaman], nil
}

func (f *fakePinjamanStore) GetStatusAngsuranByPinjamanID(_ context.Context, _ database.DBTX, idPinjaman int64, _ int64, _ time.Time) ([]model.StatusAngsuran, error) {
	var res []model.StatusAngsuran
	for _, a := range f.jadwalMap[idPinjaman] {
		res = append(res, model.StatusAngsuran{Angsuran: a})
	}
	return res, nil
}

func (f *fakePinjamanStore) GetPembayaranByPinjamanID(_ context.Context, _ database.DBTX, _ int64) ([]model.Pembayaran, error) {
	return nil, nil
}

func TestAjukanPinjaman(t *testing.T) {
	now := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		login      bool
		anggota    model.Anggota
		saldo      model.SaldoSimpanan
		adaAktif   bool
		input      AjukanPinjamanInput
		wantErr    error
		wantJadwal int
	}{
		{
			name:    "gagal belum login",
			login:   false,
			input:   AjukanPinjamanInput{IDAnggota: 1, Jumlah: "3.000.000", Tenor: 12},
			wantErr: ErrBelumLogin,
		},
		{
			name:    "gagal jumlah tidak sah",
			login:   true,
			input:   AjukanPinjamanInput{IDAnggota: 1, Jumlah: "nol", Tenor: 12},
			wantErr: ErrJumlahPinjamanTidakSah,
		},
		{
			name:    "gagal tenor terlalu pendek",
			login:   true,
			input:   AjukanPinjamanInput{IDAnggota: 1, Jumlah: "3.000.000", Tenor: 5},
			wantErr: ErrTenorDiLuarJangkauan,
		},
		{
			name:    "gagal tenor terlalu panjang",
			login:   true,
			input:   AjukanPinjamanInput{IDAnggota: 1, Jumlah: "3.000.000", Tenor: 25},
			wantErr: ErrTenorDiLuarJangkauan,
		},
		{
			name:  "gagal anggota bukan aktif",
			login: true,
			anggota: model.Anggota{
				ID:     1,
				Status: model.StatusAnggotaKeluar,
			},
			saldo:   model.SaldoSimpanan{Pokok: 500_000, Wajib: 1_000_000},
			input:   AjukanPinjamanInput{IDAnggota: 1, Jumlah: "3.000.000", Tenor: 12},
			wantErr: ErrBukanAnggotaAktif,
		},
		{
			name:  "gagal masih ada pinjaman aktif",
			login: true,
			anggota: model.Anggota{
				ID:     1,
				Status: model.StatusAnggotaAktif,
			},
			saldo:    model.SaldoSimpanan{Pokok: 500_000, Wajib: 1_000_000},
			adaAktif: true,
			input:    AjukanPinjamanInput{IDAnggota: 1, Jumlah: "3.000.000", Tenor: 12},
			wantErr:  ErrAdaPinjamanAktif,
		},
		{
			name:  "gagal melebihi batas (3 x 1.5jt = 4.5jt, ajukan 5jt)",
			login: true,
			anggota: model.Anggota{
				ID:     1,
				Status: model.StatusAnggotaAktif,
			},
			saldo:    model.SaldoSimpanan{Pokok: 500_000, Wajib: 1_000_000},
			adaAktif: false,
			input:    AjukanPinjamanInput{IDAnggota: 1, Jumlah: "5.000.000", Tenor: 12},
			wantErr:  ErrJumlahMelebihiBatas,
		},
		{
			name:  "sukses ajukan pinjaman pas batas",
			login: true,
			anggota: model.Anggota{
				ID:     1,
				Status: model.StatusAnggotaAktif,
			},
			saldo:      model.SaldoSimpanan{Pokok: 500_000, Wajib: 1_000_000},
			adaAktif:   false,
			input:      AjukanPinjamanInput{IDAnggota: 1, Jumlah: "4.500.000", Tenor: 12},
			wantErr:    nil,
			wantJadwal: 12,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pStore := newFakePinjamanStore()
			pStore.adaAktif = tc.adaAktif

			aRepo := &fakeAnggota{data: make(map[int64]model.Anggota)}
			if tc.anggota.ID > 0 {
				aRepo.data[tc.anggota.ID] = tc.anggota
			}

			sRepo := &fakeSimpanan{
				saldo: tc.saldo,
			}

			svc := NewPinjamanService(
				&fakeDB{},
				pStore,
				aRepo,
				sRepo,
				NewAuditService(&fakeAudit{}),
				3,     // kelipatan batas = 3x
				6, 24, // tenor 6..24
				100,  // jasa 1%
				2000, // denda Rp2.000
				200,  // biaya admin 2%
				time.UTC,
				func() time.Time { return now },
			)

			ctx := context.Background()
			if tc.login {
				ctx = DenganIdentitas(ctx, Identitas{IDPengguna: 99, Username: "admin"})
			}

			id, err := svc.AjukanPinjaman(ctx, tc.input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("AjukanPinjaman() error = %v, wantErr = %v", err, tc.wantErr)
			}

			if tc.wantErr == nil {
				if id <= 0 {
					t.Errorf("id pinjaman diharapkan > 0, dapat %d", id)
				}
				jadwal := pStore.jadwalMap[id]
				if len(jadwal) != tc.wantJadwal {
					t.Errorf("jumlah jadwal = %d, want %d", len(jadwal), tc.wantJadwal)
				}
			}
		})
	}
}

func TestPinjamanListDanDetail(t *testing.T) {
	now := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	pStore := newFakePinjamanStore()
	pStore.pinjamanMap[1] = model.PinjamanListItem{
		Pinjaman: model.Pinjaman{
			ID:               1,
			IDAnggota:        10,
			JumlahPinjaman:   3_000_000,
			TenorBulan:       12,
			JasaPerBulanBps:  100,
			DendaPerHari:     2000,
			TanggalPencairan: now,
			Status:           model.StatusPinjamanAktif,
		},
		NomorAnggota: "KKS-0001",
		NamaAnggota:  "Andi Pratama",
	}
	pStore.jadwalMap[1] = []model.Angsuran{
		{
			ID:         1,
			IDPinjaman: 1,
			Urutan:     1,
			JatuhTempo: time.Date(2025, 2, 10, 0, 0, 0, 0, time.UTC),
			Pokok:      250_000,
			Jasa:       30_000,
		},
	}

	svc := NewPinjamanService(
		&fakeDB{},
		pStore,
		&fakeAnggota{},
		&fakeSimpanan{},
		NewAuditService(&fakeAudit{}),
		3, 6, 24, 100, 2000, 200,
		time.UTC,
		func() time.Time { return now },
	)

	ctx := DenganIdentitas(context.Background(), Identitas{IDPengguna: 99, Username: "admin"})

	// Test List
	list, err := svc.List(ctx, "")
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("List() length = %d, want 1", len(list))
	}

	// Test Detail
	detail, err := svc.Detail(ctx, 1)
	if err != nil {
		t.Fatalf("Detail() unexpected error: %v", err)
	}
	if detail.ID != 1 || detail.NamaAnggota != "Andi Pratama" {
		t.Fatalf("Detail() mismatch: %+v", detail)
	}
	if detail.RincianLunas == nil {
		t.Fatal("Detail() RincianLunas should not be nil for active loan")
	}
}
