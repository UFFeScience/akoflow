package sshfilesystem

import (
	"context"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type connectionStoreFake struct{ connection *domain.EnvironmentConnection }

func (f connectionStoreFake) FindConnection(context.Context, string) (*domain.EnvironmentConnection, error) {
	return f.connection, nil
}

type commandFake struct {
	name  string
	args  []string
	calls int
}

func (f *commandFake) Run(_ context.Context, name string, args []string, _ []byte) ([]byte, error) {
	f.name, f.args, f.calls = name, args, f.calls+1
	if f.calls > 1 {
		return []byte("payload"), nil
	}
	return []byte("result.sif\tf\t7\t1700000000\t\n"), nil
}

func testStorage() domain.StorageResource {
	return domain.StorageResource{ID: "scratch", Type: domain.StorageSSH, Endpoint: "/scratch/project",
		BrowseRoots: []domain.StorageBrowseRoot{{Path: "/scratch/project"}}, Metadata: map[string]any{"connectionId": "ssh-1"}}
}
func testConnection() *domain.EnvironmentConnection {
	return &domain.EnvironmentConnection{ID: "ssh-1", Type: domain.ConnectionSSH, Endpoint: "hpc.example", Username: "researcher"}
}

func TestBrowseUsesConfiguredSSHConnectionAndBoundedRoot(t *testing.T) {
	executor := &commandFake{}
	driver := New(connectionStoreFake{testConnection()}, executor)
	page, err := driver.Browse(context.Background(), testStorage(), domain.BrowseRequest{Path: "runs"})
	if err != nil {
		t.Fatal(err)
	}
	if executor.name != "ssh" || !strings.Contains(strings.Join(executor.args, " "), "researcher@hpc.example") {
		t.Fatalf("expected SSH login-node command, got %s %v", executor.name, executor.args)
	}
	if len(page.Entries) != 1 || page.Entries[0].Path != "runs/result.sif" {
		t.Fatalf("unexpected page: %+v", page)
	}
	for _, arg := range executor.args {
		if strings.Contains(arg, "/scratch/project/runs") {
			t.Fatalf("raw remote path must not be interpolated into SSH arguments: %q", arg)
		}
	}
}

func TestBrowseRejectsPathEscapeBeforeSSH(t *testing.T) {
	executor := &commandFake{}
	driver := New(connectionStoreFake{testConnection()}, executor)
	_, err := driver.Browse(context.Background(), testStorage(), domain.BrowseRequest{Path: "../../etc"})
	if err == nil || executor.name != "" {
		t.Fatalf("err=%v command=%s", err, executor.name)
	}
}

func TestOpenUsesStatThenSafeRemoteCat(t *testing.T) {
	executor := &commandFake{}
	driver := New(connectionStoreFake{testConnection()}, executor)
	body, entry, err := driver.Open(context.Background(), testStorage(), "result.sif")
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	if entry.Type != domain.FileEntryFile {
		t.Fatalf("entry=%+v", entry)
	}
	buffer := make([]byte, 7)
	if _, err = body.Read(buffer); err != nil || string(buffer) != "payload" {
		t.Fatalf("got=%q err=%v", buffer, err)
	}
}
