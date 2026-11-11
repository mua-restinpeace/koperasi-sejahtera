.PHONY: run build test fmt fmt-check migrasi-naik migrasi-turun test-integrasi buat-pengguna

# URL basis data untuk alat migrasi golang-migrate (multiStatements wajib agar berkas .sql bisa berisi banyak perintah).
# Ganti sesuai komputer Anda: make migrasi-naik MIGRATE_URL='mysql://user:pass@tcp(127.0.0.1:3306)/koperasi?multiStatements=true'
MIGRATE_URL ?= mysql://koperasi:koperasi@tcp(127.0.0.1:3306)/koperasi?multiStatements=true

run:
	go run ./cmd/koperasi

build:
	go build -o bin/koperasi ./cmd/koperasi

test:
	go vet ./...
	go test ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo 'Belum diformat, jalankan: make fmt'; gofmt -l .; exit 1)

# Menjalankan semua migrasi yang belum dijalankan. Pasang alatnya sekali:
#   go install -tags 'mysql' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrasi-naik:
	migrate -path migrations -database "$(MIGRATE_URL)" up

# Membatalkan satu migrasi terakhir.
migrasi-turun:
	migrate -path migrations -database "$(MIGRATE_URL)" down 1

# Tes yang memakai MySQL sungguhan. KOPERASI_TEST_DSN harus menunjuk ke basis data khusus tes
# (nama berakhiran _test) karena tabelnya dikosongkan. Lihat docs/setup-lokal.md.
test-integrasi:
	go test -tags integration -count=1 ./internal/...

# Membuat akun pengurus: make buat-pengguna ARGS='-username budi -nama "Budi Santoso"'
# Kata sandi dibaca dari stdin (satu baris).
buat-pengguna:
	go run ./cmd/buat-pengguna $(ARGS)
