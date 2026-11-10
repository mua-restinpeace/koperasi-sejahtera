.PHONY: run build test fmt fmt-check

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
