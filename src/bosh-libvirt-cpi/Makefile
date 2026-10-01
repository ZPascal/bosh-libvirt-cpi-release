.PHONY: test test-unit test-integration test-e2e-qemu test-e2e-lxc build coverage lint

test: test-unit

test-unit:
	go test ./... -coverprofile=coverage.out

test-integration-qemu:
	LIBVIRT_URI=qemu:///system sudo -E go test -tags integration -mod=vendor ./...

test-integration-lxc:
	LIBVIRT_URI=lxc:/// sudo -E go test -tags integration -mod=vendor ./...

build:
	mkdir -p bin
	go build -o bin/cpi ./main/

coverage:
	go tool cover -html=coverage.out

lint:
	golangci-lint run ./...
