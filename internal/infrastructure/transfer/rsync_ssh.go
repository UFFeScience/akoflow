package transfer

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"os"
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
	args := []string{"-o", "BatchMode=yes", "-o", "ConnectionAttempts=1", "-o", "ConnectTimeout=10"}
	identity, knownHosts, proxy, alias := "", "", "", ""
	extraOptions := e.Configuration["sshOptions"]
	port, forward := 0, false
	if key := e.Configuration["identityFile"]; key != "" {
		identity = key
		args = append(args, "-i", key)
	}
	if options := e.Configuration["sshOptions"]; options != "" {
		args = append(args, strings.Fields(options)...)
	}
	if uri, err := url.Parse(e.URI); err == nil && uri.Scheme == "ssh" && uri.Host != "" {
		query := uri.Query()
		if strict, _ := strconv.ParseBool(query.Get("strictHostKeyChecking")); strict {
			args = append(args, "-o", "HostKeyAlgorithms=ssh-ed25519")
		}
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
			parsedPort, _ := strconv.Atoi(port)
			if parsedPort > 0 {
				args = append(args, "-p", port)
			}
		}
		port, _ = strconv.Atoi(query.Get("port"))
		if proxy = query.Get("proxyCommand"); proxy != "" {
			args = append(args, "-o", "ProxyCommand="+provider.ProxyCommandWithKnownHosts(proxy, knownHosts, identity))
		}
		if alias = query.Get("hostKeyAlias"); alias != "" {
			args = append(args, "-o", "HostKeyAlias="+alias)
		}
		forward, _ = strconv.ParseBool(query.Get("forwardAgent"))
		if forward {
			args = append(args, "-A")
		}
		username := ""
		if uri.User != nil {
			username = uri.User.Username()
		}
		connectionID := e.Configuration["connectionId"]
		if connectionID == "" {
			connectionID = query.Get("connectionId")
		}
		if connectionID == "" {
			connectionID = e.ConnectionID
		}
		multiplex, _, err := provider.SSHMultiplexArguments(provider.SSHSessionKey{
			ConnectionID: connectionID, Username: username, Host: uri.Hostname(), Port: port,
			IdentityFile: identity, ProxyCommand: proxy, KnownHostsFile: knownHosts,
			HostKeyAlias: alias, ForwardAgent: forward, ExtraOptions: extraOptions,
		})
		if err == nil {
			args = append(args, multiplex...)
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
	output, err := retrySSHRead(ctx, time.Second, func() ([]byte, error) {
		return runSSHCombinedOutput(ctx, e, args, nil)
	})
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
	cmd, release, err := queuedSSHCommand(ctx, e, args)
	if err != nil {
		return nil, err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		release()
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		release()
		return nil, err
	}
	return readCloser{Reader: out, close: func() error {
		_ = out.Close()
		err := waitSSHCommand(cmd)
		release()
		return err
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

// VerifyDigest avoids downloading an intact artifact just to validate its cache.
func (RsyncSSH) VerifyDigest(ctx context.Context, e domain.TransferEndpoint, name, expected string) (bool, error) {
	host, path, err := sshTarget(e, name)
	if err != nil {
		return false, err
	}
	output, err := retrySSHRead(ctx, time.Second, func() ([]byte, error) {
		return runSSHCombinedOutput(ctx, e, append(sshArgs(e), host, "sha256sum -- "+shell(path)), nil)
	})
	if err != nil {
		return false, fmt.Errorf("verify SSH artifact checksum: %w: %s", err, strings.TrimSpace(string(output)))
	}
	fields := strings.Fields(string(output))
	if len(fields) < 2 || len(fields[0]) != 64 {
		return false, fmt.Errorf("invalid remote SHA-256 response")
	}
	return fields[0] == strings.TrimPrefix(expected, "sha256:"), nil
}

func (connector RsyncSSH) Put(ctx context.Context, e domain.TransferEndpoint, name string, input io.Reader, offset int64) error {
	host, path, err := sshTarget(e, name)
	if err != nil {
		return err
	}
	if output, mkdirErr := runSSHCombinedOutput(ctx, e, append(sshArgs(e), host, "mkdir -p -- "+shell(filepath.Dir(path))), nil); mkdirErr != nil {
		return fmt.Errorf("create SSH staging directory: %w: %s", mkdirErr, strings.TrimSpace(string(output)))
	}
	command := "cat > " + shell(path)
	if offset > 0 {
		command = fmt.Sprintf("test $(wc -c < %s) -eq %d && cat >> %s", shell(path), offset, shell(path))
	}
	cmd, release, err := queuedSSHCommand(ctx, e, append(sshArgs(e), host, command))
	if err != nil {
		return err
	}
	defer release()
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
		return fmt.Errorf("stream to SSH staging: %w (SSH exit: %v): %s", copyErr, waitErr, strings.TrimSpace(stderr.String()))
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
	output, err := runSSHCombinedOutput(ctx, e, args, nil)
	if err != nil {
		return fmt.Errorf("commit SSH artifact: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
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
		output, runErr := runSSHCombinedOutput(ctx, source, append(sshArgs(source), sourceHost, remoteCommand), nil)
		if runErr != nil {
			return 0, fmt.Errorf("copy workspace inside cloud instance: %w: %s", runErr, strings.TrimSpace(string(output)))
		}
		return 0, nil
	case domain.TransferDirectRuntime:
		if address := strings.TrimSpace(destination.Configuration["directAddress"]); address != "" {
			destinationHost = destinationUserHost(destination, address)
		}
		identity := destination.Configuration["directIdentityFile"]
		knownHosts := destination.Configuration["directKnownHostsFile"]
		writeCommand := "mkdir -p -- " + shell(filepath.Dir(destinationPath)) + " && cat > " + shell(destinationPath)
		if offset > 0 {
			writeCommand = fmt.Sprintf("test $(wc -c < %s) -eq %d && cat >> %s", shell(destinationPath), offset, shell(destinationPath))
		}
		cleanup := func() {}
		if identity == "" || knownHosts == "" {
			identity, knownHosts, cleanup, err = prepareDirectCredential(ctx, source, destination, writeCommand)
			if err != nil {
				return 0, err
			}
			defer cleanup()
		}
		remoteSSH := []string{"ssh", "-F", "/dev/null", "-i", shell(identity), "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes", "-o", "ForwardAgent=no", "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=5", "-o", "ServerAliveCountMax=2"}
		remoteSSH = append(remoteSSH, "-o", shell("UserKnownHostsFile="+knownHosts), "-o", "StrictHostKeyChecking=yes")
		if uri, parseErr := url.Parse(destination.URI); parseErr == nil && uri.Query().Get("strictHostKeyChecking") == "true" {
			remoteSSH = append(remoteSSH, "-o", "HostKeyAlgorithms=ssh-ed25519", "-o", shell("HostKeyAlias="+uri.Query().Get("hostKeyAlias")))
		}
		if uri, parseErr := url.Parse(destination.URI); parseErr == nil && uri.Port() != "" {
			remoteSSH = append(remoteSSH, "-p", shell(uri.Port()))
		}
		readCommand := "cat -- " + shell(sourcePath)
		if offset > 0 {
			readCommand = fmt.Sprintf("tail -c +%d -- %s", offset+1, shell(sourcePath))
		}
		remoteCommand = "bash -o pipefail -c " + shell(readCommand+" | "+strings.Join(remoteSSH, " ")+" "+shell(destinationHost)+" "+shell(writeCommand))
		output, runErr := runSSHCombinedOutput(ctx, source, append(sshArgs(source), sourceHost, remoteCommand), nil)
		if runErr != nil {
			return 0, fmt.Errorf("stream directly between cloud instances: %w: %s", runErr, strings.TrimSpace(string(output)))
		}
		return -1, nil // caller replaces this with the known remaining blob size
	default:
		return 0, fmt.Errorf("SSH connector does not implement %q route", strategy)
	}
}

func destinationUserHost(endpoint domain.TransferEndpoint, address string) string {
	u, err := url.Parse(endpoint.URI)
	if err == nil && u.User != nil && u.User.Username() != "" {
		return u.User.Username() + "@" + address
	}
	return address
}

func prepareDirectCredential(ctx context.Context, source, destination domain.TransferEndpoint, forcedCommand string) (string, string, func(), error) {
	publicKey, privateKey, err := ephemeralSSHKey()
	if err != nil {
		return "", "", func() {}, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return "", "", func() {}, fmt.Errorf("generate transfer credential identity: %w", err)
	}
	token := fmt.Sprintf("akoflow-transfer-%x", nonce)
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
	authorizedLine, err := scopedDirectAuthorizedKey(publicKey, token, forcedCommand, time.Now().Add(30*time.Minute))
	if err != nil {
		return "", "", func() {}, err
	}
	installDestination := "umask 077; mkdir -p ~/.ssh; touch ~/.ssh/authorized_keys; flock ~/.ssh/.akoflow-transfer-lock sh -c " + shell("printf '%s\\n' "+shell(authorizedLine)+" >> ~/.ssh/authorized_keys")
	if output, installErr := runSSHCombinedOutput(ctx, destination, append(sshArgs(destination), destinationHost, installDestination), nil); installErr != nil {
		return "", "", func() {}, fmt.Errorf("install temporary destination credential: %w: %s", installErr, strings.TrimSpace(string(output)))
	}
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		removeDestination := directCredentialRemovalCommand(authorizedLine)
		_, _ = runSSHCombinedOutput(cleanupCtx, destination, append(sshArgs(destination), destinationHost, removeDestination), nil)
		_, _ = runSSHCombinedOutput(cleanupCtx, source, append(sshArgs(source), sourceHost, "rm -f -- "+shell(identity)+" "+shell(knownHosts)), nil)
	}
	installSource := "umask 077; cat > " + shell(identity)
	if err := runSSHInput(ctx, source, sourceHost, installSource, privateKey); err != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("install temporary source credential: %w", err)
	}
	destinationURL, _ := url.Parse(destination.URI)
	directAddress := strings.TrimSpace(destination.Configuration["directAddress"])
	if directAddress == "" {
		directAddress = destinationURL.Hostname()
	}
	port := destinationURL.Port()
	if port == "" {
		port = "22"
	}
	if destinationURL.Query().Get("strictHostKeyChecking") == "true" {
		pinned, readErr := os.ReadFile(destinationURL.Query().Get("knownHostsFile"))
		if readErr != nil || len(pinned) == 0 {
			cleanup()
			return "", "", func() {}, fmt.Errorf("read verified direct destination host key: %v", readErr)
		}
		if writeErr := runSSHInput(ctx, source, sourceHost, "umask 077; cat > "+shell(knownHosts), pinned); writeErr != nil {
			cleanup()
			return "", "", func() {}, fmt.Errorf("install verified direct destination host key: %w", writeErr)
		}
		return identity, knownHosts, cleanup, nil
	}
	scan := "ssh-keyscan -p " + shell(port) + " -- " + shell(directAddress) + " > " + shell(knownHosts)
	if output, scanErr := runSSHCombinedOutput(ctx, source, append(sshArgs(source), sourceHost, scan), nil); scanErr != nil {
		cleanup()
		return "", "", func() {}, fmt.Errorf("capture direct destination host key: %w: %s", scanErr, strings.TrimSpace(string(output)))
	}
	return identity, knownHosts, cleanup, nil
}

func scopedDirectAuthorizedKey(publicKey []byte, token, forcedCommand string, expires time.Time) (string, error) {
	key, _, _, rest, err := ssh.ParseAuthorizedKey(publicKey)
	if err != nil || key.Type() != "ssh-ed25519" || len(strings.TrimSpace(string(rest))) != 0 || strings.TrimSpace(forcedCommand) == "" || token == "" || strings.ContainsAny(forcedCommand, "\r\n\x00") || strings.ContainsAny(token, " \t\r\n\x00") {
		return "", fmt.Errorf("invalid scoped direct transfer authorization")
	}
	return "restrict,command=" + strconv.Quote(forcedCommand) + ",expiry-time=" + strconv.Quote(expires.UTC().Format("20060102150405Z")) + " " + strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))) + " " + token, nil
}

// Serialize append/revocation across simultaneous transfers. Remove the exact
// authorization, not every line containing a substring, and keep user keys.
func directCredentialRemovalCommand(authorizedLine string) string {
	remove := "umask 077; tmp=$(mktemp ~/.ssh/.akoflow-keys-XXXXXX); trap 'rm -f -- \"$tmp\"' EXIT; " +
		"grep -F -x -v -- " + shell(authorizedLine) + " ~/.ssh/authorized_keys > \"$tmp\" || test \"$?\" -eq 1; " +
		"chmod 600 \"$tmp\"; mv -- \"$tmp\" ~/.ssh/authorized_keys"
	return "flock ~/.ssh/.akoflow-transfer-lock sh -c " + shell("set -eu; "+remove)
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
	encodedPrivate, err := ssh.MarshalPrivateKey(private, "temporary-workspace-transfer")
	if err != nil {
		return nil, nil, fmt.Errorf("encode temporary transfer private key: %w", err)
	}
	return ssh.MarshalAuthorizedKey(sshPublic), pem.EncodeToMemory(encodedPrivate), nil
}

func runSSHInput(ctx context.Context, endpoint domain.TransferEndpoint, host, command string, input []byte) error {
	output, err := runSSHCombinedOutput(ctx, endpoint, append(sshArgs(endpoint), host, command), strings.NewReader(string(input)))
	if err != nil {
		return fmt.Errorf("ssh command: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func queuedSSHCommand(ctx context.Context, endpoint domain.TransferEndpoint, args []string) (*exec.Cmd, func(), error) {
	release, err := provider.AcquireSSHBulkChannel(ctx, sshControlPath(args))
	if err != nil {
		return nil, nil, fmt.Errorf("wait for SSH channel: %w", err)
	}
	command := exec.CommandContext(ctx, "ssh", args...)
	command.WaitDelay = 5 * time.Second
	return command, release, nil
}

func runSSHCombinedOutput(ctx context.Context, endpoint domain.TransferEndpoint, args []string, input io.Reader) ([]byte, error) {
	command, release, err := queuedSSHCommand(ctx, endpoint, args)
	if err != nil {
		return nil, err
	}
	defer release()
	command.Stdin = input
	return command.CombinedOutput()
}

// runSSHStdout keeps transport diagnostics out of machine-readable responses.
func runSSHStdout(ctx context.Context, endpoint domain.TransferEndpoint, args []string, input io.Reader) ([]byte, error) {
	command, release, err := queuedSSHCommand(ctx, endpoint, args)
	if err != nil {
		return nil, err
	}
	defer release()
	var stdout, stderr bytes.Buffer
	command.Stdin = input
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("ssh command: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// runSSHProtocol exposes only bounded failure categories, never remote stderr.
// Successful machine-readable stdout remains separate from transport diagnostics.
func runSSHProtocol(ctx context.Context, endpoint domain.TransferEndpoint, args []string) ([]byte, error) {
	output, err := runSSHStdout(ctx, endpoint, args, nil)
	if err == nil {
		return output, nil
	}
	category := peerProbeFailureCategory(err)
	message := strings.ToLower(err.Error())
	for _, diagnostic := range []struct{ text, category string }{
		{"connection closed", "connection_closed"},
		{"connection reset", "connection_reset"},
		{"broken pipe", "broken_pipe"},
		{"no space left", "disk_full"},
		{"sealed snapshot", "manifest_validation_failed"},
	} {
		if strings.Contains(message, diagnostic.text) {
			category = diagnostic.category
			break
		}
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("SSH protocol failure [%s]: %w", category, ctx.Err())
	}
	return nil, fmt.Errorf("SSH protocol failure [%s]", category)
}

func sshControlPath(args []string) string {
	for _, argument := range args {
		if strings.HasPrefix(argument, "ControlPath=") {
			return strings.TrimPrefix(argument, "ControlPath=")
		}
	}
	return ""
}

type readCloser struct {
	io.Reader
	close func() error
}

func (r readCloser) Close() error { return r.close() }
func shell(v string) string       { return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'" }
