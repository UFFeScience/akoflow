package sshfilesystem

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/UFFeScience/akoflow/internal/domain"
)

type connectionStoreFake struct {
	connection *domain.EnvironmentConnection
	err        error
}

func (f connectionStoreFake) FindConnection(context.Context, string) (*domain.EnvironmentConnection, error) {
	return f.connection, f.err
}

type commandFake struct {
	name   string
	args   []string
	calls  int
	input  []byte
	err    error
	output []byte
}

func (f *commandFake) Run(_ context.Context, name string, args []string, input []byte) ([]byte, error) {
	f.name, f.args, f.calls = name, args, f.calls+1
	f.input = append([]byte(nil), input...)
	if f.err != nil {
		return nil, f.err
	}
	if f.output != nil {
		return f.output, nil
	}
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
	driver := New(connectionStoreFake{connection: testConnection()}, executor)
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
	driver := New(connectionStoreFake{connection: testConnection()}, executor)
	_, err := driver.Browse(context.Background(), testStorage(), domain.BrowseRequest{Path: "../../etc"})
	if err == nil || executor.name != "" {
		t.Fatalf("err=%v command=%s", err, executor.name)
	}
}

func TestOpenUsesStatThenSafeRemoteCat(t *testing.T) {
	executor := &commandFake{}
	driver := New(connectionStoreFake{connection: testConnection()}, executor)
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

func TestBrowsePaginationAndEntryTypes(t *testing.T) {
	executor := &commandFake{output: []byte("zeta\td\t0\t1700000000\t\nalpha\tl\t4\t1700000001\ttarget\nbeta\tf\t5\t1700000002\t\n")}
	driver := New(connectionStoreFake{connection: testConnection()}, executor)
	page, err := driver.Browse(context.Background(), testStorage(), domain.BrowseRequest{Limit: 2})
	if err != nil || len(page.Entries) != 2 || page.Entries[0].Name != "alpha" || page.Entries[0].Type != domain.FileEntrySymlink || page.NextCursor != "2" {
		t.Fatalf("page=%+v err=%v", page, err)
	}
	next, err := driver.Browse(context.Background(), testStorage(), domain.BrowseRequest{Cursor: page.NextCursor, Limit: 2})
	if err != nil || len(next.Entries) != 1 || next.Entries[0].Type != domain.FileEntryDirectory || next.NextCursor != "" {
		t.Fatalf("next=%+v err=%v", next, err)
	}
	if _, err := driver.Browse(context.Background(), testStorage(), domain.BrowseRequest{Cursor: "invalid"}); err == nil {
		t.Fatal("nonnumeric cursor must fail")
	}
	if _, err := driver.Browse(context.Background(), testStorage(), domain.BrowseRequest{Cursor: "-1"}); err == nil {
		t.Fatal("negative cursor must fail")
	}
	last, err := driver.Browse(context.Background(), testStorage(), domain.BrowseRequest{Cursor: "99"})
	if err != nil || len(last.Entries) != 0 {
		t.Fatalf("last=%+v err=%v", last, err)
	}
}

func TestLocalWriteRemoveAndReadOnlyGuards(t *testing.T) {
	executor := &commandFake{output: []byte("file.txt\tf\t7\t1700000000\t\n")}
	connection := &domain.EnvironmentConnection{ID: "local", Type: domain.ConnectionLocal}
	storage := testStorage()
	storage.Metadata = nil
	storage.Configuration = map[string]any{"connectionId": "local"}
	driver := New(connectionStoreFake{connection: connection}, executor)
	if err := driver.Write(context.Background(), storage, "folder/file.txt", strings.NewReader("payload"), 7); err != nil {
		t.Fatal(err)
	}
	if executor.name != "sh" || string(executor.input) != "payload" {
		t.Fatalf("command=%s input=%q", executor.name, executor.input)
	}
	if err := driver.Remove(context.Background(), storage, "folder/file.txt"); err != nil {
		t.Fatal(err)
	}
	storage.ReadOnly = true
	if err := driver.Write(context.Background(), storage, "file", strings.NewReader("x"), 1); err == nil {
		t.Fatal("read-only write must fail")
	}
	if err := driver.Remove(context.Background(), storage, "file"); err == nil {
		t.Fatal("read-only remove must fail")
	}
}

func TestConnectionAndRootValidation(t *testing.T) {
	storage := testStorage()
	tests := []struct {
		name   string
		driver *Driver
		store  domain.StorageResource
	}{
		{"missing store", New(nil, &commandFake{}), storage},
		{"missing id", New(connectionStoreFake{connection: testConnection()}, &commandFake{}), domain.StorageResource{ID: "storage", Endpoint: "/root"}},
		{"lookup error", New(connectionStoreFake{err: errors.New("lookup")}, &commandFake{}), storage},
		{"missing connection", New(connectionStoreFake{}, &commandFake{}), storage},
		{"unsupported connection", New(connectionStoreFake{connection: &domain.EnvironmentConnection{Type: domain.ConnectionKubernetes}}, &commandFake{}), storage},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := test.driver.Browse(context.Background(), test.store, domain.BrowseRequest{}); err == nil {
				t.Fatal("connection error expected")
			}
		})
	}
	if _, _, err := browseRoot(domain.StorageResource{}, ""); err == nil {
		t.Fatal("missing root must fail")
	}
	storage.BrowseRoots = []domain.StorageBrowseRoot{{Path: "/other"}}
	if _, _, err := browseRoot(storage, ""); err == nil {
		t.Fatal("unlisted root must fail")
	}
}

func TestParsingAndHelpers(t *testing.T) {
	storage := testStorage()
	if _, err := parseEntries(storage, "", "invalid"); err == nil {
		t.Fatal("invalid listing must fail")
	}
	if _, err := parseEntries(storage, "", "file\tf\tnan\t0"); err == nil {
		t.Fatal("invalid size must fail")
	}
	encoded := encode("value")
	decoded, err := decodeField(encoded)
	if err != nil || decoded != "value" {
		t.Fatalf("decoded=%q err=%v", decoded, err)
	}
	if _, err := decodeField("%%%"); err == nil {
		t.Fatal("invalid base64 must fail")
	}
	values := map[string]any{"int": 1, "float": float64(2), "string": "3", "bool": true}
	if integer(values, "int") != 1 || integer(values, "float") != 2 || integer(values, "string") != 3 || integer(values, "missing") != 0 || !boolean(values, "bool") {
		t.Fatal("helper conversion failed")
	}
	if credentialFile(" file:/tmp/key ") != "/tmp/key" || New(nil, nil).Type() != domain.StorageSSH {
		t.Fatal("driver helpers failed")
	}
	var _ io.Reader = strings.NewReader("")
}
