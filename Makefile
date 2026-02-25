.PHONY: test test-verbose build clean install docs help

test:
	CGO_ENABLED=0 go test ./...

test-verbose:
	CGO_ENABLED=0 go test -v ./...

build:
	CGO_ENABLED=0 go build -ldflags="-X github.com/meain/esa/internal/buildinfo.Commit=$$(git rev-parse --short HEAD) -X github.com/meain/esa/internal/buildinfo.Date=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o esa .

docs:
	go run ./cmd/docgen docs/*.md notes/*.md

clean:
	rm -f esa

install: build
	mkdir -p ~/.local/bin
	cp esa ~/.local/bin/esa

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build the esa binary"
	@echo "  test          Run tests"
	@echo "  test-verbose  Run tests with verbose output"
	@echo "  install       Build and install esa to ~/.local/bin"
	@echo "  clean         Remove built binary"
	@echo "  help          Show this help message"
