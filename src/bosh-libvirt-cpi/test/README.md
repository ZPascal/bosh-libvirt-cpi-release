# Integration Tests Setup

## Voraussetzungen

- libvirt installiert und laufend
- qemu-img verfügbar
- Test-Berechtigungen für libvirt

## Verzeichnisstruktur

```
test/
├── integration/
│   ├── cpi_test.go           # End-to-End CPI Tests
│   ├── vm_lifecycle_test.go  # VM Lifecycle Tests
│   ├── disk_test.go          # Disk Operations Tests
│   └── fixtures/
│       ├── test-stemcell.tgz
│       └── test-config.json
└── README.md
```

## Integration Tests ausführen

```bash
# Mit echtem libvirt (benötigt Berechtigungen)
go test -tags=integration ./test/integration -v

# Mit Mock libvirt
go test ./test/integration -v
```

## Test-Umgebung Setup

```bash
# libvirt Test-Netzwerk erstellen
virsh net-define test/fixtures/test-network.xml
virsh net-start test-network

# Test-Storage-Pool
virsh pool-define test/fixtures/test-pool.xml
virsh pool-start test-pool
```

## Cleanup nach Tests

```bash
# Alle Test-VMs löschen
virsh list --all | grep "test-" | awk '{print $2}' | xargs -I {} virsh undefine {} --remove-all-storage

# Test-Netzwerk entfernen
virsh net-destroy test-network
virsh net-undefine test-network
```

## Continuous Integration

Die Tests können in CI/CD Pipelines integriert werden:

```yaml
# .github/workflows/test.yml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.25'
      - name: Install libvirt
        run: |
          sudo apt-get update
          sudo apt-get install -y qemu-kvm libvirt-daemon-system
      - name: Run Unit Tests
        run: go test -short ./...
      - name: Run Integration Tests
        run: go test -tags=integration ./test/integration
```
