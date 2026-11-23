.PHONY: build test check web run

build:
	go build -o bin/invisi-explorer ./cmd/invisi-explorer

test:
	go test ./...

check:
	go vet ./...

web:
	cd web && npm ci && npm run build

run:
	go run ./cmd/invisi-explorer run
