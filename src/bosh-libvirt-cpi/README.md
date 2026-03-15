# BOSH Libvirt CPI - Source Code

This directory contains the source code for the BOSH Libvirt Cloud Provider Interface (CPI).

## Quick Start

```bash
# Build
make build

# Run tests
make test

# Run all checks (format, lint, test)
make check

# Get help
make help
```

## Development

### Prerequisites

- Go 1.25 or later
- libvirt installed (for integration tests)
- qemu-img (for disk operations)

### Building

```bash
make build
```

The binary will be created at `../../bin/cpi`.

### Testing

```bash
# Unit tests only
make test-unit

# With race detector
make test-race

# With coverage
make test-coverage

# Integration tests (requires libvirt)
make test-integration
```

### Code Quality

```bash
# Format code
make fmt

# Run linter
make lint

# Run all checks
make check
```

## Project Structure

```
.
├── cpi/                # CPI implementation
├── disk/               # Disk management
├── driver/             # Command execution
├── main/               # Main entry point
├── provider/           # Libvirt provider
├── qemu/               # QEMU image operations
├── stemcell/           # Stemcell management
├── testhelpers/        # Test utilities
├── vm/                 # VM management
└── vendor/             # Go dependencies
```

## Package Overview

### cpi
Core CPI implementation following the BOSH CPI specification.

### disk
Disk creation, attachment, and management using qcow2 format.

### driver
Command execution layer for virsh and other CLI tools.

### provider
Libvirt provider implementation supporting multiple hypervisors:
- QEMU/KVM
- VirtualBox (via libvirt)
- LXC
- Xen
- VMware (experimental)

### qemu
Native Go wrapper for qemu-img operations.

### stemcell
Stemcell import and management for libvirt.

### vm
Virtual machine lifecycle management, networking, and storage.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run `make check` to ensure quality
5. Submit a pull request

## License

Apache License 2.0


# TODO
- [ ] Add integration tests support
- [ ] Adjust the test helper