BINARY := uf
BIN_DIR := bin

.PHONY: build test bench fmt vet check clean cross

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

clean:
	rm -rf $(BIN_DIR)
