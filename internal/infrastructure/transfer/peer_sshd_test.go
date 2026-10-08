package transfer

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Run in the Linux worker-compatible container: this exercises sshd's actual
// authorized_keys parser, not only Go's syntax parser. No real user key file
// is touched and the server is bound exclusively to loopback.
func TestScopedPeerCredentialAuthenticatesWithOpenSSHServer(t *testing.T) {
	sshd, err := exec.LookPath("sshd")
	if err != nil || os.Geteuid() != 0 {
		t.Skip("isolated sshd integration requires Linux root container with openssh-server")
	}
	if err := os.MkdirAll("/run/sshd", 0755); err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp("/root", ".akoflow-sshd-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	write := func(name string, content []byte) string {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	hostPublic, hostPrivate, err := ephemeralSSHKey()
	if err != nil {
		t.Fatal(err)
	}
	clientPublic, clientPrivate, err := ephemeralSSHKey()
	if err != nil {
		t.Fatal(err)
	}
	hostKey := write("host-key", hostPrivate)
	clientKey := write("client-key", clientPrivate)
	authorized, err := scopedDirectAuthorizedKey(clientPublic, "integration-token", "printf '%s' AUTHORIZED", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	authorizedPath := write("authorized_keys", []byte(authorized+"\n"))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	config := fmt.Sprintf("ListenAddress 127.0.0.1\nPort %d\nHostKey %s\nAuthorizedKeysFile %s\nPidFile %s\nUsePAM yes\nPasswordAuthentication no\nKbdInteractiveAuthentication no\nPermitRootLogin yes\nStrictModes yes\nLogLevel VERBOSE\n", port, hostKey, authorizedPath, filepath.Join(root, "pid"))
	server := exec.Command(sshd, "-D", "-e", "-f", write("sshd_config", []byte(config)))
	log, err := os.Create(filepath.Join(root, "server-log"))
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	server.Stderr = log
	if err := server.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { server.Process.Kill(); server.Wait() }()
	address := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("isolated sshd did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	knownHosts := write("known_hosts", []byte("integration-host "+strings.TrimSpace(string(hostPublic))+"\n"))
	current, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-F", "/dev/null", "-p", fmt.Sprint(port), "-i", clientKey, "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "StrictHostKeyChecking=yes", "-o", "HostKeyAlias=integration-host", "-o", "UserKnownHostsFile=" + knownHosts, "-o", "ConnectTimeout=3", current.Username + "@127.0.0.1", "printf '%s' UNAUTHORIZED"}
	output, err := exec.Command("ssh", args...).CombinedOutput()
	if err != nil || string(output) != "AUTHORIZED" {
		serverLog, _ := os.ReadFile(filepath.Join(root, "server-log"))
		t.Fatalf("scoped authorization rejected or command not forced: %v %s; server: %s", err, output, serverLog)
	}
	expired, err := scopedDirectAuthorizedKey(clientPublic, "integration-token", "printf '%s' AUTHORIZED", time.Now().Add(-time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	write("authorized_keys", []byte(expired+"\n"))
	output, err = exec.Command("ssh", args...).CombinedOutput()
	if err == nil || !strings.Contains(string(output), "Permission denied") {
		t.Fatalf("expired authorization must be rejected: %v %s", err, output)
	}
}
