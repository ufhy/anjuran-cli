BINARY := uf
BIN_DIR := bin

# Versi paket spec Fig yang dipakai. Naikkan angka ini untuk menyegarkan spec.
FIG_VERSION := 2.692.3

.PHONY: build test bench fmt vet check clean cross specs snapshot release-check

build:
	go build -o $(BIN_DIR)/$(BINARY) ./cmd/uf

test:
	go test ./...

bench:
	go test ./internal/engine/ -bench=. -benchmem -run=XXX

fmt:
	gofmt -w .

vet:
	go vet ./...

check: fmt vet test

# cross membuktikan sejak awal bahwa tidak ada kode OS-specific yang menyelinap masuk.
cross:
	@for t in darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		os=$${t%/*}; arch=$${t#*/}; ext=""; \
		[ "$$os" = "windows" ] && ext=".exe"; \
		GOOS=$$os GOARCH=$$arch go build -o $(BIN_DIR)/$(BINARY)-$$os-$$arch$$ext ./cmd/uf || exit 1; \
		echo "  ok $$os/$$arch"; \
	done

# specs membangun ulang direktori spec dari paket npm Fig. Butuh node dan curl.
specs:
	@tools/transpile/build-specs.sh $(FIG_VERSION) specs

# snapshot membangun rilis percobaan lengkap ke dist/ tanpa mempublikasikan
# apa pun. Ini satu-satunya cara memastikan paketnya benar-benar berisi spec:
# kesalahan pola berkas tidak pernah terlihat dari konfigurasinya saja.
snapshot:
	goreleaser release --snapshot --clean --skip=publish

release-check:
	goreleaser check

clean:
	rm -rf $(BIN_DIR) dist
