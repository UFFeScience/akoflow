package transfer

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type workspaceEndpointResolver struct{}

func (workspaceEndpointResolver) ResolveTransferEndpoint(_ context.Context, location domain.TransferLocation) (domain.TransferEndpoint, error) {
	return domain.TransferEndpoint{URI: location.URI}, nil
}

func workspaceURL(path string) string { return (&url.URL{Scheme: "file", Path: path}).String() }

func TestWorkspaceRsyncSSHArgumentsKeepRemotePathAbsolute(t *testing.T) {
	remote := domain.TransferEndpoint{URI: "ssh://akoflow@example.test/akoflow/workspace/runs/run-1/producer"}
	local := domain.TransferEndpoint{URI: workspaceURL(t.TempDir())}
	for _, endpoints := range [][2]domain.TransferEndpoint{{remote, local}, {local, remote}} {
		args, err := rsyncArgs(endpoints[0], endpoints[1])
		if err != nil {
			t.Fatal(err)
		}
		if !containsArgument(args, "--secluded-args") {
			t.Fatalf("rsync SSH arguments lack --secluded-args: %q", args)
		}
		want := "akoflow@example.test:/akoflow/workspace/runs/run-1/producer/"
		if !containsArgument(args, want) {
			t.Fatalf("remote path was quoted or changed: %q", args)
		}
	}
}

func TestWorkspaceRsyncSSHRemoteShellKeepsProxyCommandInOneArgument(t *testing.T) {
	remote := domain.TransferEndpoint{URI: "ssh://researcher@plafrim/scratch/run?" + url.Values{
		"identityFile":   {"storage/credentials/ssh/ovvesley-personal"},
		"knownHostsFile": {"storage/credentials/ssh/known_hosts"},
		"port":           {"22"},
		"proxyCommand":   {"ssh -A -l wferreir ssh.plafrim.fr -W plafrim:22"},
		"forwardAgent":   {"true"},
	}.Encode()}
	local := domain.TransferEndpoint{URI: workspaceURL(t.TempDir())}
	args, err := rsyncArgs(remote, local)
	if err != nil {
		t.Fatal(err)
	}
	var remoteShell string
	for index, argument := range args {
		if argument == "-e" && index+1 < len(args) {
			remoteShell = args[index+1]
			break
		}
	}
	if remoteShell == "" {
		t.Fatalf("rsync arguments lack remote shell: %q", args)
	}
	for _, expected := range []string{
		"ssh -o BatchMode=yes -i storage/credentials/ssh/ovvesley-personal",
		`-o "ProxyCommand=ssh `,
		` -W plafrim:22" -A`,
	} {
		if !strings.Contains(remoteShell, expected) {
			t.Fatalf("remote shell %q does not contain %q", remoteShell, expected)
		}
	}
	if strings.Contains(remoteShell, `' -A'`) {
		t.Fatalf("forward-agent option was absorbed by a quoted argument: %q", remoteShell)
	}
}

func TestWorkspaceRsyncRemoteShellIsParsedIntoExactSSHArguments(t *testing.T) {
	if _, err := exec.LookPath("rsync"); err != nil {
		t.Skip("rsync is unavailable")
	}
	temporary := t.TempDir()
	argumentLog := filepath.Join(temporary, "arguments")
	fakeSSH := filepath.Join(temporary, "ssh")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shell(argumentLog) + "\nexit 1\n"
	if err := os.WriteFile(fakeSSH, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	remote := domain.TransferEndpoint{URI: "ssh://researcher@plafrim/scratch/run?" + url.Values{
		"identityFile":   {"storage/credentials/ssh/ovvesley-personal"},
		"knownHostsFile": {"storage/credentials/ssh/known_hosts"},
		"port":           {"22"},
		"proxyCommand":   {"ssh -A -l wferreir ssh.plafrim.fr -W plafrim:22"},
		"forwardAgent":   {"true"},
	}.Encode()}
	args, err := rsyncArgs(remote, domain.TransferEndpoint{URI: workspaceURL(t.TempDir())})
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("rsync", args...)
	command.Env = append(os.Environ(), "PATH="+temporary+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, runErr := command.CombinedOutput()
	contents, err := os.ReadFile(argumentLog)
	if err != nil {
		t.Skipf("installed rsync did not invoke the PATH-provided remote shell (%v): %v: %s", err, runErr, strings.TrimSpace(string(output)))
	}
	arguments := strings.Split(strings.TrimSpace(string(contents)), "\n")
	for _, expected := range []string{
		"-i",
		"storage/credentials/ssh/ovvesley-personal",
		"ProxyCommand=ssh -o UserKnownHostsFile='storage/credentials/ssh/known_hosts' -o StrictHostKeyChecking=accept-new -i 'storage/credentials/ssh/ovvesley-personal' -A -l wferreir ssh.plafrim.fr -W plafrim:22",
		"-A",
	} {
		if !containsArgument(arguments, expected) {
			t.Fatalf("SSH arguments lack %q: %q", expected, arguments)
		}
	}
}

func containsArgument(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestRemoteSymlinkCheckIgnoresSuccessfulSSHWarnings(t *testing.T) {
	installWorkspaceSSHCheck(t, "", "mux_client_request_session: session request failed: Session open refused by peer\nControlSocket /tmp/control already exists, disabling multiplexing\n", 0)
	endpoint := domain.TransferEndpoint{URI: "ssh://researcher@example.test/workspace"}
	if err := rejectWorkspaceSymlinks(context.Background(), endpoint); err != nil {
		t.Fatalf("successful SSH warning was interpreted as a symlink: %v", err)
	}
}

func TestRemoteSymlinkCheckUsesOnlyFindStdout(t *testing.T) {
	installWorkspaceSSHCheck(t, "/workspace/input-link\n", "SSH diagnostic\n", 0)
	endpoint := domain.TransferEndpoint{URI: "ssh://researcher@example.test/workspace"}
	err := rejectWorkspaceSymlinks(context.Background(), endpoint)
	if err == nil || !strings.Contains(err.Error(), "/workspace/input-link") || strings.Contains(err.Error(), "SSH diagnostic") {
		t.Fatalf("unexpected symlink result: %v", err)
	}
}

func installWorkspaceSSHCheck(t *testing.T, stdout, stderr string, exitCode int) {
	t.Helper()
	directory := t.TempDir()
	script := "#!/bin/sh\nprintf '%s' " + shell(stdout) + "\nprintf '%s' " + shell(stderr) + " >&2\nexit " + fmt.Sprint(exitCode) + "\n"
	if err := os.WriteFile(filepath.Join(directory, "ssh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("AKOFLOW_SSH_CONTROL_DIRECTORY", t.TempDir())
}

func TestWorkspaceRsyncMergesPredecessorsBeforeSuccessorStarts(t *testing.T) {
	root := t.TempDir()
	first, second, target := filepath.Join(root, "first"), filepath.Join(root, "second"), filepath.Join(root, "target")
	for _, dir := range []string{first, second, target} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for file, content := range map[string]string{filepath.Join(first, "a.txt"): "from-first", filepath.Join(second, "b.txt"): "from-second"} {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plans := []domain.DataTransferPlan{
		{ID: "one", ExecutionRunID: "run", ProducerActivityID: "first", ConsumerActivityID: "consumer", Source: domain.TransferLocation{URI: workspaceURL(first)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}},
		{ID: "two", ExecutionRunID: "run", ProducerActivityID: "second", ConsumerActivityID: "consumer", Source: domain.TransferLocation{URI: workspaceURL(second)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}},
	}
	syncer := WorkspaceRsync{Resolver: workspaceEndpointResolver{}}
	runs, err := syncer.Sync(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.txt", "b.txt"} {
		if _, err := os.Stat(filepath.Join(target, name)); err != nil {
			t.Fatal(err)
		}
	}
	for _, run := range runs {
		if run.Status != domain.TransferCompleted || run.FilesTransferred != 1 || run.TransferredBytes == 0 {
			t.Fatalf("unexpected transfer %+v", run)
		}
	}
	runs, err = syncer.Sync(context.Background(), plans)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range runs {
		if run.FilesTransferred != 0 || run.TransferredBytes != 0 {
			t.Fatalf("retry sent unchanged content: %+v", run)
		}
	}
}

func TestWorkspaceRsyncRejectsConflictingPredecessors(t *testing.T) {
	root := t.TempDir()
	first, second, target := filepath.Join(root, "first"), filepath.Join(root, "second"), filepath.Join(root, "target")
	for _, dir := range []string{first, second, target} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for file, content := range map[string]string{filepath.Join(first, "shared.txt"): "one", filepath.Join(second, "shared.txt"): "two"} {
		if err := os.WriteFile(file, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	plans := []domain.DataTransferPlan{
		{ID: "one", Source: domain.TransferLocation{URI: workspaceURL(first)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}},
		{ID: "two", Source: domain.TransferLocation{URI: workspaceURL(second)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}},
	}
	runs, err := (WorkspaceRsync{Resolver: workspaceEndpointResolver{}}).Sync(context.Background(), plans)
	if err == nil || !strings.Contains(err.Error(), "conflicting contents") {
		t.Fatalf("want conflict, got %v", err)
	}
	for _, run := range runs {
		if run.Status != domain.TransferFailed || run.Error == "" {
			t.Fatalf("missing failure: %+v", run)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "shared.txt")); !os.IsNotExist(err) {
		t.Fatalf("published conflicting file: %v", err)
	}
}

func TestWorkspaceRsyncRejectsDifferentExistingSuccessorFile(t *testing.T) {
	root := t.TempDir()
	source, target := filepath.Join(root, "source"), filepath.Join(root, "target")
	for _, dir := range []string{source, target} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "same.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "same.txt"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	plan := domain.DataTransferPlan{ID: "edge", Source: domain.TransferLocation{URI: workspaceURL(source)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}}
	runs, err := (WorkspaceRsync{Resolver: workspaceEndpointResolver{}}).Sync(context.Background(), []domain.DataTransferPlan{plan})
	if err == nil || !strings.Contains(err.Error(), "different content") {
		t.Fatalf("want destination conflict, got %v", err)
	}
	if runs[0].Status != domain.TransferFailed {
		t.Fatalf("unexpected status %+v", runs[0])
	}
	contents, err := os.ReadFile(filepath.Join(target, "same.txt"))
	if err != nil || string(contents) != "old" {
		t.Fatalf("destination overwritten: %q %v", contents, err)
	}
}

func TestWorkspaceRsyncRejectsDestinationSymlink(t *testing.T) {
	root := t.TempDir()
	source, target, outside := filepath.Join(root, "source"), filepath.Join(root, "target"), filepath.Join(root, "outside")
	for _, dir := range []string{source, target, outside} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(source, "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "nested", "file.txt"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(target, "nested")); err != nil {
		t.Fatal(err)
	}
	plan := domain.DataTransferPlan{ID: "edge", Source: domain.TransferLocation{URI: workspaceURL(source)}, Destination: domain.TransferLocation{URI: workspaceURL(target)}}
	_, err := (WorkspaceRsync{Resolver: workspaceEndpointResolver{}}).Sync(context.Background(), []domain.DataTransferPlan{plan})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("want symlink refusal, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "file.txt")); !os.IsNotExist(err) {
		t.Fatalf("wrote outside destination: %v", err)
	}
}
