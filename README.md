# Roadmap: Replacing the VirtualBox CLI Wrapper with libvirt

This document outlines the steps required to replace the existing VirtualBox implementation with a `libvirt`-based implementation. The goal is to make the CPI more flexible and remove the dependency on the VirtualBox CLI.

## Phase 1: Analysis and Design

1.  **Analysis of the Existing Codebase:**
    *   Investigate the `vm` and `driver` packages to understand the exact interaction with the VirtualBox CLI.
    *   Identify the core functionalities that need to be replaced by `libvirt` (e.g., VM creation, status queries, network and storage management).
    *   Analyze the test suites (`cpi_suite_test.go`, `vm_suite_test.go`) to understand the scope of test adjustments required for the migration.

2.  **Design of an Abstraction Layer:**
    *   Define a `Driver` interface that encapsulates interactions with the virtualization layer. This interface should include methods for all VM operations (create, delete, start, stop, etc.).
    *   Adapt the `cpi.Factory` to choose different `Driver` implementations. This could be controlled via the `cpi.json` configuration file.

## Phase 2: Implementation

3.  **Implementation of the `libvirt` Driver:**
    *   Create a new Go package for the `libvirt` driver (e.g., `driver/libvirt_driver`).
    *   Implement the `Driver` interface defined in Phase 1 using the `go-libvirt` library.
    *   Ensure that all functions required by the CPI (VM lifecycle, network and disk management) are covered.

4.  **Integration and migration of the `libvirt` Driver:**
    *   Adjust the factory code (`cpi/factory.go`) to instantiate the new `libvirt` driver based on the configuration.
    *   Refactor/ migrate the existing CPI logic to use the new `Driver` interface instead of direct VirtualBox calls.

## Phase 3: Testing

5.  **Unit Tests:**
    *   Create unit tests for the new `libvirt` driver. Mocks for the `go-libvirt` API can be helpful here.
    *   Adapt existing unit tests to account for the new abstraction layer.

6.  **Integration Tests:**
    *   Adapt and extend the existing integration tests (`cpi_suite_test.go`) to fully test the `libvirt` implementation.
    *   Perform end-to-end tests in a BOSH environment to ensure the CPI functions correctly with `libvirt`.

## Phase 4: Documentation and Finalization

7.  **Documentation:**
    *   Update the `README.md` file with instructions on how to configure and use the `libvirt`-based CPI.
    *   Document the new `Driver` architecture and how additional virtualization platforms can be added if needed.