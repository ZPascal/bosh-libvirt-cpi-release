.PHONY: test test-unit test-integration test-e2e-qemu test-e2e-lxc build coverage lint

test: test-unit

test-unit:
	go test ./... -coverprofile=coverage.out

test-integration-qemu:
	sudo env LIBVIRT_URI=qemu:///system REQUIRE_LIBVIRT=1 $$(command -v go) test -tags integration -mod=vendor ./...

test-integration-lxc:
	sudo env LIBVIRT_URI=lxc:/// REQUIRE_LIBVIRT=1 $$(command -v go) test -tags integration -mod=vendor ./...

build:
	mkdir -p bin
	go build -o bin/cpi ./main/

coverage:
	go tool cover -html=coverage.out

lint:
	golangci-lint run ./...
