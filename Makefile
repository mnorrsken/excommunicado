PKG     := github.com/mnorrsken/excommunicado
VERSION := $(shell git describe --tags --always --dirty)
COMMIT  := $(shell git rev-parse --short HEAD)
LDFLAGS := -s -w -X $(PKG)/internal/version.Version=$(VERSION) -X $(PKG)/internal/version.Commit=$(COMMIT)
COMPOSE ?= docker compose

.PHONY: build run test race vet fmt vuln clean docker-build up down

build:
	go build -ldflags '$(LDFLAGS)' -o bin/excommunicado ./cmd/excommunicado

# Expects a llama.cpp server on port 8080. `make up` starts everything in containers.
run:
	go run ./cmd/excommunicado

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...

clean:
	rm -rf bin dist data

docker-build:
	$(COMPOSE) build

up:
	$(COMPOSE) up -d --build

down:
	$(COMPOSE) down
