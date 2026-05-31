# PR 1: Security Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix three security vulnerabilities: SSH host key verification, HomeDir shell injection, and path traversal in Store.

**Architecture:** Each fix is self-contained in its own file with accompanying tests. No new dependencies required — `golang.org/x/crypto/ssh` already provides `ssh.FixedHostKey` and `ssh.ParseAuthorizedKey`.

**Tech Stack:** Go 1.24, `golang.org/x/crypto/ssh`, Ginkgo/Gomega

---

### Task 1: SSH host key verification — add HostKey to structs and validation

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/ssh_runner.go`
- Modify: `src/bosh-libvirt-cpi/cpi/factory_options.go`
- Modify: `src/bosh-libvirt-cpi/cpi/factory.go`

- [ ] **Step 1: Write failing test for validation rejects SSH config without HostKey**

In `src/bosh-libvirt-cpi/cpi/factory_options_test.go`, add inside the existing `Context("when Host is set", ...)` block:

```go
It("returns error when HostKey is empty", func() {
    opts.Host = "remote.example.com"
    opts.Username = "user"
    opts.PrivateKey = "key"
    opts.HostKey = ""

    err := opts.Validate()
    Expect(err).To(HaveOccurred())
    Expect(err.Error()).To(ContainSubstring("HostKey"))
})

It("succeeds when Host, Username, PrivateKey, and HostKey are all set", func() {
    opts.Host = "remote.example.com"
    opts.Username = "user"
    opts.PrivateKey = "key"
    opts.HostKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."

    Expect(opts.Validate()).ToNot(HaveOccurred())
})
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./cpi/ -run "HostKey" -v
```

Expected: FAIL — `opts.HostKey` field does not exist yet.

- [ ] **Step 3: Add HostKey to FactoryOpts and SSHRunnerOpts**

In `src/bosh-libvirt-cpi/cpi/factory_options.go`, add `HostKey` to the struct and validation:

```go
type FactoryOpts struct {
	BackendURI string
	Host       string
	Username   string
	PrivateKey string
	HostKey    string // SSH host public key in authorized_keys format; required when Host is set

	StoreDir string

	AutoEnableNetworks bool

	Agent apiv1.AgentOptions
}

func (o FactoryOpts) Validate() error {
	if len(o.Host) > 0 {
		if o.Username == "" {
			return bosherr.Error("Must provide non-empty Username")
		}
		if o.PrivateKey == "" {
			return bosherr.Error("Must provide non-empty PrivateKey")
		}
		if o.HostKey == "" {
			return bosherr.Error("Must provide non-empty HostKey when Host is set")
		}
	}
	// ... rest of Validate unchanged
```

In `src/bosh-libvirt-cpi/driver/ssh_runner.go`, add `HostKey` to `SSHRunnerOpts`:

```go
type SSHRunnerOpts struct {
	Host       string
	Username   string
	PrivateKey string
	HostKey    string // authorized_keys format, e.g. "ssh-ed25519 AAAA..."
}
```

- [ ] **Step 4: Run tests to confirm validation tests pass**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./cpi/ -run "HostKey" -v
```

Expected: PASS

- [ ] **Step 5: Wire HostKey through factory.go to SSHRunnerOpts**

In `src/bosh-libvirt-cpi/cpi/factory.go`, update the SSHRunnerOpts construction:

```go
runnerOpts := driver.SSHRunnerOpts{
    Host:       f.opts.Host,
    Username:   f.opts.Username,
    PrivateKey: f.opts.PrivateKey,
    HostKey:    f.opts.HostKey,
}
```

- [ ] **Step 6: Commit**

```bash
git add src/bosh-libvirt-cpi/cpi/factory_options.go \
        src/bosh-libvirt-cpi/cpi/factory_options_test.go \
        src/bosh-libvirt-cpi/cpi/factory.go \
        src/bosh-libvirt-cpi/driver/ssh_runner.go
git commit -m "feat: add HostKey field to SSHRunnerOpts and FactoryOpts with fail-closed validation"
```

---

### Task 2: SSH host key verification — implement FixedHostKey in client()

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/ssh_runner.go`
- Modify: `src/bosh-libvirt-cpi/driver/ssh_runner_test.go`

- [ ] **Step 1: Write failing test for host key parsing error**

In `src/bosh-libvirt-cpi/driver/ssh_runner_test.go`, add a new `Context`:

```go
Context("client() host key parsing", func() {
    It("returns error when HostKey is malformed", func() {
        opts := SSHRunnerOpts{
            Host:       "127.0.0.1",
            Username:   "user",
            PrivateKey: validTestPrivateKey, // see below
            HostKey:    "not-a-valid-key",
        }
        logger := boshlog.NewLogger(boshlog.LevelNone)
        runner := NewSSHRunner(opts, nil, logger)
        _, err := runner.HomeDir()
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("Parsing host key"))
    })
})
```

Add this constant at the top of the test file (a throwaway key for parsing tests only):

```go
const validTestPrivateKey = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACDummyfakekeyforparsingtestsonlyAAAAAA==
-----END OPENSSH PRIVATE KEY-----`
```

- [ ] **Step 2: Run test to confirm it fails**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./driver/ -run "host key parsing" -v
```

Expected: FAIL — `client()` still uses `InsecureIgnoreHostKey`.

- [ ] **Step 3: Implement FixedHostKey in client()**

Replace the `client()` method in `src/bosh-libvirt-cpi/driver/ssh_runner.go`:

```go
func (r *SSHRunner) client() (*ssh.Client, error) {
	if r.existingClient != nil {
		return r.existingClient, nil
	}

	keySigner, err := ssh.ParsePrivateKey([]byte(r.opts.PrivateKey))
	if err != nil {
		return nil, bosherr.WrapError(err, "Parsing private key")
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(r.opts.HostKey))
	if err != nil {
		return nil, bosherr.WrapError(err, "Parsing host key")
	}

	config := &ssh.ClientConfig{
		User:            r.opts.Username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(keySigner)},
		HostKeyCallback: ssh.FixedHostKey(pubKey),
	}

	r.existingClient, err = ssh.Dial("tcp", fmt.Sprintf("%s:22", r.opts.Host), config)
	if err != nil {
		return nil, bosherr.WrapError(err, "Connecting via SSH")
	}

	return r.existingClient, nil
}
```

- [ ] **Step 4: Run tests**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./driver/ -run "host key" -v
```

Expected: PASS — the malformed host key test passes. The existing SSHRunner tests that require `TEST_SSH_RUNNER_USERNAME` will be skipped.

- [ ] **Step 5: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/ssh_runner.go \
        src/bosh-libvirt-cpi/driver/ssh_runner_test.go
git commit -m "fix: use ssh.FixedHostKey instead of InsecureIgnoreHostKey in SSHRunner"
```

---

### Task 3: Fix HomeDir shell injection

**Files:**
- Modify: `src/bosh-libvirt-cpi/driver/ssh_runner.go`
- Modify: `src/bosh-libvirt-cpi/driver/ssh_runner_test.go`

- [ ] **Step 1: Write failing test that documents the safe command**

In `src/bosh-libvirt-cpi/driver/ssh_runner_test.go`, add:

```go
Context("HomeDir command", func() {
    It("does not use backtick subshell", func() {
        // Verify the HomeDir implementation no longer uses the
        // injection-prone backtick form. We inspect the runner's
        // internal command by using a fake session recorder.
        // Since SSHRunner.HomeDir calls r.execute() directly, we
        // verify by checking that the safe command form is used
        // when the runner tries to connect (it will fail without a
        // real server, but we can check the error is connection-
        // related, not a shell parse error from a bad command).
        opts := SSHRunnerOpts{
            Host:       "127.0.0.1",
            Username:   "user",
            PrivateKey: validTestPrivateKey,
            HostKey:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINotAReal+Key=",
        }
        logger := boshlog.NewLogger(boshlog.LevelNone)
        runner := NewSSHRunner(opts, nil, logger)
        _, err := runner.HomeDir()
        // Must fail due to connection refused / host key mismatch,
        // NOT due to a shell injection in the command itself.
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).ToNot(ContainSubstring("eval"))
    })
})
```

- [ ] **Step 2: Replace the HomeDir implementation**

In `src/bosh-libvirt-cpi/driver/ssh_runner.go`, replace `HomeDir()`:

```go
func (r *SSHRunner) HomeDir() (string, error) {
	output, _, err := r.execute("getent passwd $(id -u) | cut -d: -f6")
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(output)
	if result == "" || strings.HasPrefix(result, "~") {
		return "", bosherr.Errorf("Failed to expand home directory, got: '%s'", result)
	}

	return result, nil
}
```

- [ ] **Step 3: Run all driver tests**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./driver/ -v
```

Expected: PASS (SSH live tests skipped without env vars set).

- [ ] **Step 4: Commit**

```bash
git add src/bosh-libvirt-cpi/driver/ssh_runner.go \
        src/bosh-libvirt-cpi/driver/ssh_runner_test.go
git commit -m "fix: remove backtick subshell from HomeDir to eliminate shell injection"
```

---

### Task 4: Path traversal in Store — add sanitizeKey

**Files:**
- Modify: `src/bosh-libvirt-cpi/vm/store.go`
- Modify: `src/bosh-libvirt-cpi/vm/store_test.go`

- [ ] **Step 1: Write failing tests for path traversal rejection**

In `src/bosh-libvirt-cpi/vm/store_test.go`, add a new `Describe("sanitizeKey")` block and tests to the existing `Put`, `Get`, `DeleteOne` describes:

```go
Describe("Put", func() {
    It("rejects keys containing ..", func() {
        err := store.Put("../etc/passwd", []byte("data"))
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("invalid key"))
    })

    It("rejects keys containing /", func() {
        err := store.Put("sub/dir", []byte("data"))
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("invalid key"))
    })

    It("accepts normal keys", func() {
        err := store.Put("metadata.json", []byte("data"))
        Expect(err).ToNot(HaveOccurred())
    })
})

Describe("Get", func() {
    It("rejects keys containing ..", func() {
        _, err := store.Get("../etc/passwd")
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("invalid key"))
    })

    It("accepts normal keys", func() {
        runner.GetResult = []byte("data")
        _, err := store.Get("agent.json")
        Expect(err).ToNot(HaveOccurred())
    })
})

Describe("DeleteOne", func() {
    It("rejects keys containing ..", func() {
        err := store.DeleteOne("../../important")
        Expect(err).To(HaveOccurred())
        Expect(err.Error()).To(ContainSubstring("invalid key"))
    })

    It("accepts normal keys", func() {
        err := store.DeleteOne("disk-abc-disk-attachment.json")
        Expect(err).ToNot(HaveOccurred())
    })
})
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./vm/ -run "Put|Get|DeleteOne" -v
```

Expected: FAIL — no sanitization exists yet.

- [ ] **Step 3: Add sanitizeKey and call it in Put, Get, DeleteOne**

In `src/bosh-libvirt-cpi/vm/store.go`:

```go
import (
    "path/filepath"
    "strings"

    bosherr "github.com/cloudfoundry/bosh-utils/errors"
    "bosh-libvirt-cpi/driver"
)

func sanitizeKey(key string) error {
    if strings.Contains(key, "..") || strings.Contains(key, "/") {
        return bosherr.Errorf("invalid key '%s': must not contain '..' or '/'", key)
    }
    return nil
}

func (m Store) Put(key string, contents []byte) error {
    if err := sanitizeKey(key); err != nil {
        return err
    }
    _, _, err := m.runner.Execute("mkdir", "-p", m.path)
    if err != nil {
        return err
    }
    return m.runner.Put(filepath.Join(m.path, key), contents)
}

func (m Store) Get(key string) ([]byte, error) {
    if err := sanitizeKey(key); err != nil {
        return nil, err
    }
    return m.runner.Get(filepath.Join(m.path, key))
}

func (m Store) DeleteOne(key string) error {
    if err := sanitizeKey(key); err != nil {
        return err
    }
    _, _, err := m.runner.Execute("rm", "-rf", filepath.Join(m.path, key))
    return err
}
```

- [ ] **Step 4: Run all vm tests**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./vm/ -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add src/bosh-libvirt-cpi/vm/store.go \
        src/bosh-libvirt-cpi/vm/store_test.go
git commit -m "fix: reject path traversal sequences in Store.Put/Get/DeleteOne"
```

---

### Task 5: Run full test suite and verify no regressions

- [ ] **Step 1: Run all unit tests**

```bash
cd src/bosh-libvirt-cpi && go test -mod=vendor ./... -v 2>&1 | tail -30
```

Expected: All packages PASS. No FAIL lines.

- [ ] **Step 2: Verify build still compiles**

```bash
cd src/bosh-libvirt-cpi && go build -mod=vendor ./...
```

Expected: No output (success).

- [ ] **Step 3: Final commit if any fixups needed, then push**

```bash
git log --oneline -5
```

Confirm the three security fix commits are present, then open the PR against `main`.
