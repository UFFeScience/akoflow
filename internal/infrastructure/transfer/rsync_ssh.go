package transfer

import (
	"context"
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
			args = append(args, "-o", "UserKnownHostsFile="+queryKnownHosts, "-o", "StrictHostKeyChecking=yes")
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

type readCloser struct {
	io.Reader
	close func() error
}

func (r readCloser) Close() error { return r.close() }
func shell(v string) string       { return "'" + strings.ReplaceAll(v, "'", "'\\\"'\\\"'") + "'" }
