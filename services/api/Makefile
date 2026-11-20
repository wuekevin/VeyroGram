# VeyroGram API — build/test entry points. The toolchain is pinned: WuKongIM requires go1.26.10, so the business service uses the same.
export GOTOOLCHAIN ?= go1.26.10
GO ?= go
BIN ?= bin/veyrogram-api
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: test vet fmt build linux
test: ; $(GO) test -race -count=1 ./...
vet:  ; $(GO) vet ./...
fmt:  ; test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
build: ; CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN) ./cmd/api
linux: ; GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(BIN)-linux-amd64 ./cmd/api

anchor-store-linux: ; GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build -trimpath -ldflags "-s -w" -o bin/anchor-store-linux-amd64 ./cmd/anchor-store
