package transfer

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/provider"
	"golang.org/x/crypto/ssh"
)

// RsyncSSH uses ssh/rsync installed on the gateway. Endpoint URI is
// ssh://user@host/absolute/base/path. A key path or extra SSH options may be
// provided in endpoint configuration as identityFile and sshOptions.
type RsyncSSH struct {
	BufferSize BufferSizeProvider
}

func (RsyncSSH) CanHandle(e domain.TransferEndpoint) bool { return strings.HasPrefix(e.URI, "ssh://") }
func sshTarget(e domain.TransferEndpoint, name string) (string, string, error) {
	uri, err := url.Parse(e.URI)
	if err != nil || uri.Scheme != "ssh" {
		return "", "", fmt.Errorf("invalid SSH endpoint")
	}
	if uri.Host == "" || uri.Path == "" {
		return "", "", fmt.Errorf("ssh endpoint requires absolute path")
	}
	host, base := uri.Host, filepath.Clean(uri.Path)
	if uri.User != nil {
		host = uri.User.Username() + "@" + host
	}
	if host == "" || !filepath.IsAbs(base) {
		return "", "", fmt.Errorf("ssh endpoint requires host and absolute base path")
	}
	key := filepath.Clean(filepath.FromSlash(name))
	if name != "" && (filepath.IsAbs(key) || key == ".." || strings.HasPrefix(key, ".."+string(filepath.Separator))) {
		return "", "", fmt.Errorf("SSH transfer path escapes endpoint")
	}
	full := filepath.Join(base, key)
	if full != base && !strings.HasPrefix(full, base+string(filepath.Separator)) {
		return "", "", fmt.Errorf("SSH transfer path escapes endpoint")
	}
	return host, full, nil
}
func sshArgs(e domain.TransferEndpoint) []string {
	args := []string{}
	identity, knownHosts := "", ""
	if key := e.Configuration["identityFile"]; key != "" {
		identity = key
		args = append(args, "-i", key)
	}
	if options := e.Configuration["sshOptions"]; options != "" {
		args = append(args, strings.Fields(options)...)
	}
	if uri, err := url.Parse(e.URI); err == nil {
		query := uri.Query()
		if queryIdentity := query.Get("identityFile"); queryIdentity != "" {
			identity = queryIdentity
			args = append(args, "-i", queryIdentity)
		}
		if queryKnownHosts := query.Get("knownHostsFile"); queryKnownHosts != "" {
			knownHosts = queryKnownHosts
			policy := "yes"
			if accepted, _ := strconv.ParseBool(query.Get("acceptNewHostKey")); accepted {
				policy = "accept-new"
			}
			args = append(args, "-o", "UserKnownHostsFile="+queryKnownHosts, "-o", "StrictHostKeyChecking="+policy)
		}
		if port := query.Get("port"); port != "" {
			args = append(args, "-p", port)
		}
		if proxy := query.Get("proxyCommand"); proxy != "" {
			args = append(args, "-o", "ProxyCommand="+provider.ProxyCommandWithKnownHosts(proxy, knownHosts, identity))
		}
		if alias := query.Get("hostKeyAlias"); alias != "" {
			args = append(args, "-o", "HostKeyAlias="+alias)
		}
		if forward, _ := strconv.ParseBool(query.Get("forwardAgent")); forward {
			args = append(args, "-A")
		}
	}
	return args
}
func (RsyncSSH) Exists(ctx context.Context, e domain.TransferEndpoint, name string) (bool, error) {
	host, path, err := sshTarget(e, name)
	if err != nil {
		return false, err
	}
	args := append(sshArgs(e), host, "test -f "+shell(path))
	output, err := exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
	if err == nil {
		return true, nil
	}
	if exit, ok := err.(*exec.ExitError); ok && exit.ExitCode() == 1 {
		return false, nil
	}
	message := strings.TrimSpace(string(output))
	if message != "" {
		return false, fmt.Errorf("check SSH location: %w: %s", err, message)
	}
	return false, fmt.Errorf("check SSH location: %w", err)
}
func (RsyncSSH) Open(ctx context.Context, e domain.TransferEndpoint, name string, offset int64) (io.ReadCloser, error) {
	host, path, err := sshTarget(e, name)
	if err != nil {
		return nil, err
	}
	command := "cat -- " + shell(path)
	if offset > 0 {
		command = fmt.Sprintf("tail -c +%d -- %s", offset+1, shell(path))
	}
	args := append(sshArgs(e), host, command)
	cmd := exec.CommandContext(ctx, "ssh", args...)
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	return readCloser{Reader: out, close: func() error {
		_ = out.Close()
		return waitSSHCommand(cmd)
	}}, nil
}

func waitSSHCommand(command *exec.Cmd) error {
	result := make(chan error, 1)
	go func() { result <- command.Wait() }()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		if command.Process != nil {
			_ = command.Process.Kill()
		}
		<-result
		// The caller already consumed the complete stdout stream. A lingering
		// ProxyCommand is transport cleanup, not a failed content read.
		return nil
	}
}
func (connector RsyncSSH) Put(ctx context.Context, e domain.TransferEndpoint, name string, input io.Reader, offset int64) error {
	host, path, err := sshTarget(e, name)
	if err != nil {
		return err
	}
	if output, mkdirErr := exec.CommandContext(ctx, "ssh", append(sshArgs(e), host, "mkdir -p -- "+shell(filepath.Dir(path)))...).CombinedOutput(); mkdirErr != nil {
		return fmt.Errorf("create SSH staging directory: %w: %s", mkdirErr, strings.TrimSpace(string(output)))
	}
	command := "cat > " + shell(path)
	if offset > 0 {
		command = fmt.Sprintf("test $(wc -c < %s) -eq %d && cat >> %s", shell(path), offset, shell(path))
	}
	cmd := exec.CommandContext(ctx, "ssh", append(sshArgs(e), host, command)...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err = cmd.Start(); err != nil {
		return err
	}
	_, copyErr := copyWithBuffer(ctx, stdin, input, connector.BufferSize)
	closeErr := stdin.Close()
	waitErr := cmd.Wait()
	if copyErr != nil {
		return fmt.Errorf("stream to SSH staging: %w", copyErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close SSH stream: %w", closeErr)
	}
	if waitErr != nil {
		return fmt.Errorf("stream to SSH staging: %w: %s", waitErr, strings.TrimSpace(stderr.String()))
	}
	return nil
}
func (RsyncSSH) Commit(ctx context.Context, e domain.TransferEndpoint, partial, final string) error {
	host, p, err := sshTarget(e, partial)
	if err != nil {
		return err
	}
	_, f, err := sshTarget(e, final)
	if err != nil {
		return err
	}
	args := append(sshArgs(e), host, "mkdir -p -- "+shell(filepath.Dir(f))+" && mv -- "+shell(p)+" "+shell(f))
	return exec.CommandContext(ctx, "ssh", args...).Run()
}

// TransferRoute keeps payload bytes on the runtime side. For different VMs a
// short-lived destination key must already be exposed to the source through
// directIdentityFile; Akoflow never copies its permanent platform key.
func (RsyncSSH) TransferRoute(ctx context.Context, strategy domain.TransferStrategy, source, destination domain.TransferEndpoint, sourceName, destinationName string, offset int64) (int64, error) {
	sourceHost, sourcePath, err := sshTarget(source, sourceName)
	if err != nil {
		return 0, err
	}
	destinationHost, destinationPath, err := sshTarget(destination, destinationName)
	if err != nil {
		return 0, err
	}
	var remoteCommand string
	switch strategy {
	case domain.TransferRuntimeLocal:
		if source.CloudInstanceID == "" || source.CloudInstanceID != destination.CloudInstanceID || sourceHost != destinationHost {
			return 0, fmt.Errorf("runtime-local requires the same concrete cloud instance")
		}
		if offset > 0 {
			remoteCommand = fmt.Sprintf("test $(wc -c < %s) -eq %d && tail -c +%d -- %s >> %s", shell(destinationPath), offset, offset+1, shell(sourcePath), shell(destinationPath))
		} else {
			remoteCommand = "mkdir -p -- " + shell(filepath.Dir(destinationPath)) + " && (cp --reflink=auto -- " + shell(sourcePath) + " " + shell(destinationPath) + " 2>/dev/null || cp -- " + shell(sourcePath) + " " + shell(destinationPath) + ")"
		}
		output, runErr := exec.CommandContext(ctx, "ssh", append(sshArgs(source), sourceHost, remoteCommand)...).CombinedOutput()
		if runErr != nil {
			return 0, fmt.Errorf("copy workspace inside cloud instance: %w: %s", runErr, strings.TrimSpace(string(output)))
		}
		return 0, nil
	case domain.TransferDirectRuntime:
		identity := destination.Configuration["directIdentityFile"]
		knownHosts := destination.Configuration["directKnownHostsFile"]
		cleanup := func() {}
		if identity == "" || knownHosts == "" {
			identity, knownHosts, cleanup, err = prepareDirectCredential(ctx, source, destination)
			if err != nil {
				return 0, err
			}
			defer cleanup()
		}
		remoteSSH := []string{"ssh", "-i", shell(identity), "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes"}
		remoteSSH = append(remoteSSH, "-o", shell("UserKnownHostsFile="+knownHosts), "-o", "StrictHostKeyChecking=yes")
		if uri, parseErr := url.Parse(destination.URI); parseErr == nil && uri.Port() != "" {
			remoteSSH = append(remoteSSH, "-p", shell(uri.Port()))
		}
		writeCommand := "mkdir -p -- " + shell(filepath.Dir(destinationPath)) + " && cat > " + shell(destinationPath)
		if offset > 0 {
			writeCommand = fmt.Sprintf("test $(wc -c < %s) -eq %d && cat >> %s", shell(destinationPath), offset, shell(destinationPath))
		}
		readCommand := "cat -- " + shell(sourcePath)
		if offset > 0 {
			readCommand = fmt.Sprintf("tail -c +%d -- %s", offset+1, shell(sourcePath))
		}
		remoteCommand = readCommand + " | " + strings.Join(remoteSSH, " ") + " " + shell(destinationHost) + " " + shell(writeCommand)
		output, runErr := exec.CommandContext(ctx, "ssh", append(sshArgs(source), sourceHost, remoteCommand)...).CombinedOutput()
		if runErr != nil {
			return 0, fmt.Errorf("stream directly between cloud instances: %w: %s", runErr, strings.TrimSpace(string(output)))
		}
		return -1, nil // caller replaces this with the known remaining blob size
	default:
		return 0, fmt.Errorf("SSH connector does not implement %q route", strategy)
	}
}

func prepareDirectCredential(ctx context.Context, source, destination domain.TransferEndpoint) (string, string, func(), error) {
	publicKey, privateKey, err := ephemeralSSHKey()
	if err != nil {
		return "", "", func() {}, err
	}
	token := fmt.Sprintf("akoflow-transfer-%d", time.Now().UnixNano())
	identity := "/tmp/" + token
	knownHosts := identity + ".known_hosts"
	destinationHost, _, err := sshTarget(destination, "")
	if err != nil {
		return "", "", func() {}, err
	}
	sourceHost, _, err := sshTarget(source, "")
	if err != nil {
		return "", "", func() {}, err
	}
	authorizedLine := "no-agent-forwarding,no-port-forwarding,no-X11-forwarding,no-pty " + strings.TrimSpace(string(publicKey)) + " " + token
	installDestination := "umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; printf '%s\\n' " + shell(authorizedLine) + " >> ~/.ssh/authorized_keys"
	if output, installErr := exec.CommandContext(ctx, "ssh", append(sshArgs(destination), destinationHost, installDestination)...).CombinedOutput(); installErr != nil {
		return "", "", func() {}, fmt.Errorf("install temporary destination credential: %w: %s", installErr, strings.TrimSpace(string(output)))
	}
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		removeDestination := "grep -v -- " + shell(token) + " ~/.ssh/authorized_keys > ~/.ssh/authorized_keys.akoflow && mv ~/.ssh/authorized_keys.akoflow ~/.ssh/authorized_keys"
		_, _ = exec.CommandContext(cleanupCtx, "ssh", append(sshArgs(destination), destinationHost, removeDestination)...).CombinedOutput()
		_, _ = exec.CommandContext(cleanupCtx, "ssh", append(sshArgs(source), sourceHost, "rm -f -- "+shell(identity)+" "+shell(knownHosts))...).CombinedOutput()
	}
	installSource := "umask 077; cat > " + shell(identity)
	if err := runSSHInput(ctx, source, sourceHost, installSource, privateKey); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("install temporary source credential: %w", err)
	}
	destinationURL, _ := url.Parse(destination.URI)
	port := destinationURL.Port()
	if port == "" {
		port = "22"
	}
	scan := "ssh-keyscan -p " + shell(port) + " -- " + shell(destinationURL.Hostname()) + " > " + shell(knownHosts)
	if output, scanErr := exec.CommandContext(ctx, "ssh", append(sshArgs(source), sourceHost, scan)...).CombinedOutput(); scanErr != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("capture direct destination host key: %w: %s", scanErr, strings.TrimSpace(string(output)))
	}
	return identity, knownHosts, cleanup, nil
}

func ephemeralSSHKey() ([]byte, []byte, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate temporary transfer key: %w", err)
	}
	sshPublic, err := ssh.NewPublicKey(public)
	if err != nil {
		return nil, nil, fmt.Errorf("encode temporary transfer public key: %w", err)
	}
	encodedPrivate, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return nil, nil, fmt.Errorf("encode temporary transfer private key: %w", err)
	}
	return ssh.MarshalAuthorizedKey(sshPublic), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: encodedPrivate}), nil
}

func runSSHInput(ctx context.Context, endpoint domain.TransferEndpoint, host, command string, input []byte) error {
	cmd := exec.CommandContext(ctx, "ssh", append(sshArgs(endpoint), host, command)...)
	cmd.Stdin = strings.NewReader(string(input))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh command: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

type readCloser struct {
	io.Reader
	close func() error
}

func (r readCloser) Close() error { return r.close() }
func shell(v string) string       { return "'" + strings.ReplaceAll(v, "'", "'\\\"'\\\"'") + "'" }
