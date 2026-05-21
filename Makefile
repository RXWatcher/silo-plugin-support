BINARY := continuum-plugin-support
GO ?= go
PNPM ?= pnpm

.PHONY: build web-deps web-build test test-go test-web clean

build: web-build
	$(GO) build -o $(BINARY) ./cmd/continuum-plugin-support

web-deps:
	cd web && $(PNPM) install --frozen-lockfile

web-build: web-deps
	cd web && $(PNPM) build

test: test-go test-web

test-go:
	$(GO) test ./...

test-web:
	cd web && $(PNPM) run test

clean:
	rm -f $(BINARY)
	rm -rf web/node_modules internal/server/public/dist
