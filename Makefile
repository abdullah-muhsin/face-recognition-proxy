.PHONY: test build up down

test:
	docker run --rm -v "$(CURDIR):/src" -w /src golang:1.24 go test ./...

build:
	docker compose build gateway

up:
	docker compose up --build

down:
	docker compose down
