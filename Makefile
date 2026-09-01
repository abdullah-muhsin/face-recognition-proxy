.PHONY: test vet build up down

test:
	docker run --rm -v "$(CURDIR):/src" -w /src golang:1.24 go test ./cmd/... ./internal/...

vet:
	docker run --rm -v "$(CURDIR):/src" -w /src golang:1.24 go vet ./cmd/... ./internal/...

build:
	docker compose build gateway

up:
	docker compose up --build

down:
	docker compose down
