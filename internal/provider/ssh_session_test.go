package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSSHChannelQueueLimitsConcurrentSessionsPerMaster(t *testing.T) {
	controlPath := filepath.Join(t.TempDir(), "master")
	var active atomic.Int32
	var maximum atomic.Int32
	releaseWorkers := make(chan struct{})
	var workers sync.WaitGroup

	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			release, err := AcquireSSHChannel(context.Background(), controlPath)
			if err != nil {
				t.Errorf("acquire channel: %v", err)
				return
			}
			current := active.Add(1)
			for {
				observed := maximum.Load()
				if current <= observed || maximum.CompareAndSwap(observed, current) {
					break
				}
			}
			<-releaseWorkers
			active.Add(-1)
			release()
		}()
	}

	deadline := time.Now().Add(time.Second)
	for maximum.Load() < SSHChannelLimit && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := maximum.Load(); got != SSHChannelLimit {
		t.Fatalf("maximum concurrent channels = %d, want %d", got, SSHChannelLimit)
	}
	close(releaseWorkers)
	workers.Wait()
}

func TestSSHChannelQueueHonorsContextCancellation(t *testing.T) {
	controlPath := filepath.Join(t.TempDir(), "master")
	releases := make([]func(), 0, SSHChannelLimit)
	for range SSHChannelLimit {
		release, err := AcquireSSHChannel(context.Background(), controlPath)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := AcquireSSHChannel(ctx, controlPath); err == nil {
		t.Fatal("cancelled waiter acquired an SSH channel")
	}
	for _, release := range releases {
		release()
	}
}

func TestSSHSessionPathUsesAllEffectiveConnectionSettings(t *testing.T) {
	directory := t.TempDir()
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", directory)
	base := SSHSessionKey{
		ConnectionID: "hpc", Username: "researcher", Host: "secret.example", Port: 22,
		IdentityFile: "/keys/private", ProxyCommand: "ssh bastion", KnownHostsFile: "/keys/known",
		HostKeyAlias: "cluster", ForwardAgent: true,
	}
	arguments, first, err := SSHMultiplexArguments(base)
	if err != nil {
		t.Fatal(err)
	}
	_, same, _ := SSHMultiplexArguments(base)
	changed := base
	changed.ProxyCommand = "ssh another-bastion"
	_, otherProxy, _ := SSHMultiplexArguments(changed)
	changed = base
	changed.IdentityFile = "/keys/other"
	_, otherIdentity, _ := SSHMultiplexArguments(changed)
	if first != same || first == otherProxy || first == otherIdentity {
		t.Fatalf("paths first=%q same=%q proxy=%q identity=%q", first, same, otherProxy, otherIdentity)
	}
	name := filepath.Base(first)
	if len(name) != 32 || strings.Contains(first, base.Username) || strings.Contains(first, base.Host) || strings.Contains(first, "private") {
		t.Fatalf("control path exposes connection data: %q", first)
	}
	joined := strings.Join(arguments, " ")
	for _, expected := range []string{"ControlMaster=auto", "ControlPath=" + first, "ControlPersist=180"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("arguments %q lack %q", joined, expected)
		}
	}
}

func TestSSHMultiplexingCanBeDisabled(t *testing.T) {
	t.Setenv("AKOFLOW_SSH_MULTIPLEXING_ENABLED", "false")
	arguments, path, err := SSHMultiplexArguments(SSHSessionKey{Host: "host"})
	if err != nil || len(arguments) != 0 || path != "" {
		t.Fatalf("arguments=%v path=%q err=%v", arguments, path, err)
	}
}

func TestSSHMultiplexingDefaultsToDaemonLocalAbsolutePath(t *testing.T) {
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", "")
	_, path, err := SSHMultiplexArguments(SSHSessionKey{Host: "host"})
	if err != nil {
		t.Fatal(err)
	}
	wantDirectory := filepath.Join(os.TempDir(), "akoflow-ssh-control")
	if !filepath.IsAbs(path) || filepath.Dir(path) != wantDirectory {
		t.Fatalf("control path=%q, want directory %q", path, wantDirectory)
	}
}

func TestSSHControlSocketErrorRecognizesMuxListenerFailure(t *testing.T) {
	err := fmt.Errorf("ssh: exit status 255: muxserver_listen: link mux listener /tmp/a.tmp => /tmp/a: Bad file descriptor")
	if !isSSHControlSocketError(err, nil) {
		t.Fatalf("mux listener failure was not recognized: %v", err)
	}
}
