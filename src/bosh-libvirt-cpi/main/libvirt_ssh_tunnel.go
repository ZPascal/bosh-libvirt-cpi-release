package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	"golang.org/x/crypto/ssh"
)

const libvirtSockPath = "/var/run/libvirt/libvirt-sock"

// sshLibvirtURI establishes an SSH connection to host, forwards the remote
// libvirt Unix socket to a local temp socket, and returns the local
// libvirt URI (e.g. "qemu+unix:///system?socket=/tmp/lv-<pid>.sock") plus
// a cleanup function that tears down the listener and SSH connection.
//
// When host is empty it returns ("", noop, nil) so the caller can always
// use the return value without branching.
func sshLibvirtURI(
	scheme, host string, port int,
	username, privateKeyPEM, hostKeyLine string,
	logger boshlog.Logger,
) (uri string, cleanup func(), err error) {
	cleanup = func() {}
	if host == "" {
		return "", cleanup, nil
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(hostKeyLine))
	if err != nil {
		return "", cleanup, fmt.Errorf("parsing host key: %w", err)
	}
	keySigner, err := ssh.ParsePrivateKey([]byte(privateKeyPEM))
	if err != nil {
		return "", cleanup, fmt.Errorf("parsing private key: %w", err)
	}

	sshPort := port
	if sshPort <= 0 {
		sshPort = 22
	}

	clientCfg := &ssh.ClientConfig{
		User:              username,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(keySigner)},
		HostKeyCallback:   ssh.FixedHostKey(pubKey),
		HostKeyAlgorithms: []string{pubKey.Type()},
	}
	sshClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, sshPort), clientCfg)
	if err != nil {
		return "", cleanup, fmt.Errorf("SSH dial %s:%d: %w", host, sshPort, err)
	}

	tmpSock := fmt.Sprintf("/tmp/libvirt-cpi-%d.sock", os.Getpid())
	_ = os.Remove(tmpSock) // remove stale socket from previous run
	ln, err := net.Listen("unix", tmpSock)
	if err != nil {
		sshClient.Close() //nolint:errcheck
		return "", cleanup, fmt.Errorf("listen unix %s: %w", tmpSock, err)
	}

	cleanup = func() {
		ln.Close()         //nolint:errcheck
		sshClient.Close()  //nolint:errcheck
		os.Remove(tmpSock) //nolint:errcheck
	}

	go serveTunnel(ln, sshClient, logger)

	// Build the libvirt URI: <scheme>+unix:///system?socket=<tmpSock>
	baseScheme := strings.Split(scheme, "+")[0]
	localURI := fmt.Sprintf("%s+unix:///system?socket=%s", baseScheme, tmpSock)

	return localURI, cleanup, nil
}

// serveTunnel accepts connections on ln and bridges each one to the remote
// libvirt Unix socket via the SSH client.
func serveTunnel(ln net.Listener, sshClient *ssh.Client, logger boshlog.Logger) {
	for {
		local, err := ln.Accept()
		if err != nil {
			// listener closed — normal shutdown
			return
		}
		remote, err := sshClient.Dial("unix", libvirtSockPath)
		if err != nil {
			logger.Error("libvirt-ssh-tunnel", "Dial remote socket: %s", err)
			local.Close() //nolint:errcheck
			continue
		}
		go bridge(local, remote)
	}
}

// bridge copies data bidirectionally between two net.Conns, closing both when done.
func bridge(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	copyHalf := func(dst, src net.Conn) {
		defer wg.Done()
		io.Copy(dst, src) //nolint:errcheck
		// Signal EOF to the other side by closing the write half if possible.
		if hc, ok := dst.(interface{ CloseWrite() error }); ok {
			hc.CloseWrite() //nolint:errcheck
		} else {
			dst.Close() //nolint:errcheck
		}
	}
	go copyHalf(a, b)
	go copyHalf(b, a)
	wg.Wait()
	a.Close() //nolint:errcheck
	b.Close() //nolint:errcheck
}
