.PHONY: test test-unit test-integration build coverage lint

test: test-unit

test-unit:
	go test ./... -coverprofile=coverage.out

test-integration:
	go test -tags integration ./...

build:
	mkdir -p bin
	go build -o bin/cpi ./main/

coverage:
	go tool cover -html=coverage.out

lint:
	golangci-lint run ./...
