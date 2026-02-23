.PHONY: test test-verbose build clean help

test:
	CGO_ENABLED=0 go test ./...

test-verbose:
	CGO_ENABLED=0 go test -v ./...

build:
	CGO_ENABLED=0 go build -ldflags="-X github.com/meain/esa/internal/buildinfo.Commit=$$(git rev-parse --short HEAD)" -o esa .

clean:
	rm -f esa

help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build         Build the esa binary"
	@echo "  test          Run tests"
	@echo "  test-verbose  Run tests with verbose output"
	@echo "  clean         Remove built binary"
	@echo "  help          Show this help message"
