.PHONY: run build test race vet fmt check

run:
	go run ./cmd/dotgame

build:
	go build -o bin/dotgame ./cmd/dotgame

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -w cmd internal

check: test vet
