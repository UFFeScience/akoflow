package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const defaultSSHControlPersistSeconds = 180
const SSHChannelLimit = 4

// SSHSessionKey contains every setting that changes the effective SSH
// transport. Its digest is safe to use as a Unix socket name and does not
// expose hosts, users, credentials or proxy commands.
type SSHSessionKey struct {
	ConnectionID   string
	Username       string
	Host           string
	Port           int
	IdentityFile   string
	ProxyCommand   string
	KnownHostsFile string
	HostKeyAlias   string
	ForwardAgent   bool
	ExtraOptions   string
}

type sshSessionManager struct {
	mu       sync.Mutex
	locks    map[string]*sync.Mutex
	channels map[string]chan struct{}
}

var sharedSSHSessions = sshSessionManager{
	locks:    make(map[string]*sync.Mutex),
	channels: make(map[string]chan struct{}),
}

// AcquireSSHChannel reserves one remote command session on a multiplexed SSH
// transport. HPC installations commonly enforce a small MaxSessions value.
// Queueing here prevents OpenSSH from silently falling back to additional TCP
// connections when the shared master is full.
func AcquireSSHChannel(ctx context.Context, controlPath string) (func(), error) {
	if strings.TrimSpace(controlPath) == "" {
		return func() {}, nil
	}
	queue := sharedSSHSessions.channelQueue(controlPath)
	select {
	case queue <- struct{}{}:
		var once sync.Once
		return func() { once.Do(func() { <-queue }) }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *sshSessionManager) channelQueue(path string) chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	queue := m.channels[path]
	if queue == nil {
		queue = make(chan struct{}, SSHChannelLimit)
		m.channels[path] = queue
	}
	return queue
}

func SSHMultiplexArguments(key SSHSessionKey) ([]string, string, error) {
	if !sshMultiplexingEnabled() {
		return nil, "", nil
	}
	directory := strings.TrimSpace(os.Getenv("AKOFLOW_SSH_CONTROL_DIRECTORY"))
	if directory == "" {
		// Control sockets must live on a daemon-local filesystem. Repository
		// bind mounts (notably Docker Desktop/virtiofs) may support regular files
		// but reject OpenSSH's atomic link of a temporary Unix socket.
		directory = filepath.Join(os.TempDir(), "akoflow-ssh-control")
	}
	directory, err := filepath.Abs(directory)
	if err != nil {
		return nil, "", fmt.Errorf("resolve SSH control directory: %w", err)
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, "", fmt.Errorf("create SSH control directory: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, "", fmt.Errorf("secure SSH control directory: %w", err)
	}
	digest := sha256.Sum256([]byte(sshSessionKeyMaterial(key)))
	path := filepath.Join(directory, hex.EncodeToString(digest[:16]))
	persist := defaultSSHControlPersistSeconds
	if value, err := strconv.Atoi(strings.TrimSpace(os.Getenv("AKOFLOW_SSH_CONTROL_PERSIST_SECONDS"))); err == nil && value > 0 {
		persist = value
	}
	return []string{
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + path,
		"-o", "ControlPersist=" + strconv.Itoa(persist),
	}, path, nil
}

func sshMultiplexingEnabled() bool {
	value := strings.TrimSpace(os.Getenv("AKOFLOW_SSH_MULTIPLEXING_ENABLED"))
	if value == "" {
		return true
	}
	enabled, err := strconv.ParseBool(value)
	return err == nil && enabled
}

func sshSessionKeyMaterial(key SSHSessionKey) string {
	values := []string{
		key.ConnectionID, key.Username, key.Host, strconv.Itoa(key.Port),
		key.IdentityFile, key.ProxyCommand, key.KnownHostsFile,
		key.HostKeyAlias, strconv.FormatBool(key.ForwardAgent),
		key.ExtraOptions,
	}
	var result strings.Builder
	for _, value := range values {
		result.WriteString(strconv.Itoa(len(value)))
		result.WriteByte(':')
		result.WriteString(value)
		result.WriteByte('|')
	}
	return result.String()
}

func (m *sshSessionManager) lockForCreation(path string) func() {
	if path == "" {
		return func() {}
	}
	if _, err := os.Lstat(path); err == nil {
		return func() {}
	}
	unlock := m.lock(path)
	if _, err := os.Lstat(path); err == nil {
		unlock()
		return func() {}
	}
	return unlock
}

func (m *sshSessionManager) lock(path string) func() {
	m.mu.Lock()
	lock := m.locks[path]
	if lock == nil {
		lock = &sync.Mutex{}
		m.locks[path] = lock
	}
	m.mu.Unlock()
	lock.Lock()
	return lock.Unlock
}

func isSSHControlSocketError(err error, output []byte) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error() + " " + string(output))
	return strings.Contains(message, "control socket") ||
		strings.Contains(message, "mux_client") ||
		strings.Contains(message, "muxserver_listen") ||
		strings.Contains(message, "master is dead") ||
		(strings.Contains(message, "mux listener") && strings.Contains(message, "bad file descriptor")) ||
		(strings.Contains(message, "controlpath") && strings.Contains(message, "refused"))
}
