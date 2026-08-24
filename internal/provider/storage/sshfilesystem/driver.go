// Package sshfilesystem browses an explicitly discovered POSIX root through
// the environment's login-node SSH connection. It never accepts an absolute
// path from the caller: every operation is relative to a configured root.
package sshfilesystem

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/application/ports"
	"github.com/UFFeScience/akoflow/internal/domain"
	provider "github.com/UFFeScience/akoflow/internal/provider"
)

type ConnectionStore interface {
	FindConnection(context.Context, string) (*domain.EnvironmentConnection, error)
}

type Driver struct {
	connections ConnectionStore
	executor    provider.CommandExecutor
}

func New(connections ConnectionStore, executor provider.CommandExecutor) *Driver {
	if executor == nil {
		executor = provider.OSCommandExecutor{}
	}
	return &Driver{connections: connections, executor: executor}
}

func (*Driver) Type() domain.StorageType { return domain.StorageSSH }

func (d *Driver) Browse(ctx context.Context, storage domain.StorageResource, request domain.BrowseRequest) (domain.BrowsePage, error) {
	if request.Cursor != "" {
		if _, err := strconv.Atoi(request.Cursor); err != nil {
			return domain.BrowsePage{}, fmt.Errorf("invalid browse cursor")
		}
	}
	root, relative, err := browseRoot(storage, request.Path)
	if err != nil {
		return domain.BrowsePage{}, err
	}
	output, err := d.run(ctx, storage, listScript, root, relative)
	if err != nil {
		return domain.BrowsePage{}, err
	}
	items, err := parseEntries(storage, relative, string(output))
	if err != nil {
		return domain.BrowsePage{}, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	start, _ := strconv.Atoi(request.Cursor)
	if start < 0 {
		return domain.BrowsePage{}, fmt.Errorf("invalid browse cursor")
	}
	limit := request.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if start > len(items) {
		start = len(items)
	}
	end := start + limit
	if end > len(items) {
		end = len(items)
	}
	page := domain.BrowsePage{StorageID: storage.ID, Path: relative, Entries: items[start:end]}
	if end < len(items) {
		page.NextCursor = strconv.Itoa(end)
	}
	return page, nil
}

func (d *Driver) BrowseStat(ctx context.Context, storage domain.StorageResource, value string) (domain.FileEntry, error) {
	root, relative, err := browseRoot(storage, value)
	if err != nil {
		return domain.FileEntry{}, err
	}
	output, err := d.run(ctx, storage, statScript, root, relative)
	if err != nil {
		return domain.FileEntry{}, err
	}
	entries, err := parseEntries(storage, path.Dir(relative), string(output))
	if err != nil || len(entries) != 1 {
		if err == nil {
			err = fmt.Errorf("storage entry not found")
		}
		return domain.FileEntry{}, err
	}
	entries[0].Path, entries[0].Name = relative, path.Base(relative)
	return entries[0], nil
}

func (d *Driver) Open(ctx context.Context, storage domain.StorageResource, value string) (io.ReadCloser, domain.FileEntry, error) {
	entry, err := d.BrowseStat(ctx, storage, value)
	if err != nil {
		return nil, domain.FileEntry{}, err
	}
	if entry.Type != domain.FileEntryFile {
		return nil, domain.FileEntry{}, fmt.Errorf("only files can be downloaded")
	}
	root, relative, _ := browseRoot(storage, value)
	content, err := d.run(ctx, storage, catScript, root, relative)
	if err != nil {
		return nil, domain.FileEntry{}, err
	}
	return io.NopCloser(strings.NewReader(string(content))), entry, nil
}

func (d *Driver) Remove(ctx context.Context, storage domain.StorageResource, value string) error {
	if storage.ReadOnly {
		return fmt.Errorf("storage is read-only")
	}
	root, relative, err := browseRoot(storage, value)
	if err != nil {
		return err
	}
	_, err = d.run(ctx, storage, removeScript, root, relative)
	return err
}

func (d *Driver) Write(ctx context.Context, storage domain.StorageResource, value string, source io.Reader, _ int64) error {
	if storage.ReadOnly {
		return fmt.Errorf("storage is read-only")
	}
	root, relative, err := browseRoot(storage, value)
	if err != nil {
		return err
	}
	bytes, err := io.ReadAll(source)
	if err != nil {
		return err
	}
	_, err = d.runInput(ctx, storage, writeScript, bytes, root, relative)
	return err
}

func (d *Driver) run(ctx context.Context, storage domain.StorageResource, script string, root, relative string) ([]byte, error) {
	return d.runInput(ctx, storage, script, nil, root, relative)
}
func (d *Driver) runInput(ctx context.Context, storage domain.StorageResource, script string, input []byte, root, relative string) ([]byte, error) {
	exec, err := d.connectionExecutor(ctx, storage)
	if err != nil {
		return nil, err
	}
	// Values are base64-encoded and assigned inside the remote script. Keeping
	// them out of `sh -c` positional arguments makes the command portable
	// across SSH servers, which may reassemble remote arguments differently.
	prefix := "set -- '" + encode(root) + "' '" + encode(relative) + "'\n"
	// Transport the complete program as a base64 token. ssh servers invoke the
	// remote command through a shell, so this avoids a second round of parsing
	// for function declarations, quotes, and newlines in the fixed program.
	program := base64.StdEncoding.EncodeToString([]byte(prefix + "\n" + script))
	return exec.Run(ctx, "sh", []string{"-c", "printf %s " + program + " | base64 -d | sh"}, input)
}

func (d *Driver) connectionExecutor(ctx context.Context, storage domain.StorageResource) (provider.CommandExecutor, error) {
	if d.connections == nil {
		return nil, fmt.Errorf("SSH storage connection store is required")
	}
	id := metadataString(storage.Metadata, "connectionId")
	if id == "" {
		id = configString(storage.Configuration, "connectionId")
	}
	if id == "" {
		return nil, fmt.Errorf("SSH storage %q has no connectionId", storage.ID)
	}
	connection, err := d.connections.FindConnection(ctx, id)
	if err != nil {
		return nil, err
	}
	if connection == nil {
		return nil, fmt.Errorf("SSH storage connection %q not found", id)
	}
	if connection.Type == domain.ConnectionLocal {
		return d.executor, nil
	}
	if connection.Type != domain.ConnectionSSH && connection.Type != domain.ConnectionAgent {
		return nil, fmt.Errorf("connection %q is not SSH-capable", id)
	}
	return provider.SSHCommandExecutor{
		Executor: d.executor, Endpoint: connection.Endpoint, Username: connection.Username,
		Port: integer(connection.Configuration, "port"), IdentityFile: credentialFile(connection.CredentialRef),
		ProxyCommand:   configString(connection.Configuration, "proxyCommand"),
		HostKeyAlias:   configString(connection.Configuration, "hostKeyAlias"),
		KnownHostsFile: configString(connection.Configuration, "knownHostsFile"),
		ForwardAgent:   boolean(connection.Configuration, "forwardAgent"),
	}, nil
}

func browseRoot(storage domain.StorageResource, value string) (string, string, error) {
	root := strings.TrimSpace(storage.Endpoint)
	if root == "" {
		return "", "", fmt.Errorf("SSH storage root is required")
	}
	allowed := len(storage.BrowseRoots) == 0
	for _, candidate := range storage.BrowseRoots {
		if path.Clean(candidate.Path) == path.Clean(root) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", "", fmt.Errorf("storage endpoint is not an allowed browse root")
	}
	relative := path.Clean(strings.TrimPrefix(strings.TrimSpace(value), "/"))
	if relative == "." {
		relative = ""
	}
	if relative == ".." || strings.HasPrefix(relative, "../") || path.IsAbs(relative) {
		return "", "", fmt.Errorf("browse path escapes its root")
	}
	return path.Clean(root), relative, nil
}
func encode(v string) string { return base64.StdEncoding.EncodeToString([]byte(v)) }
func decodeField(v string) (string, error) {
	b, e := base64.StdEncoding.DecodeString(v)
	return string(b), e
}
func metadataString(values map[string]any, key string) string {
	if v, ok := values[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}
func configString(values map[string]any, key string) string { return metadataString(values, key) }
func integer(values map[string]any, key string) int {
	switch v := values[key].(type) {
	case int:
		return v
	case float64:
		return int(v)
	case string:
		i, _ := strconv.Atoi(v)
		return i
	}
	return 0
}
func boolean(values map[string]any, key string) bool { v, _ := values[key].(bool); return v }
func credentialFile(reference string) string {
	return strings.TrimPrefix(strings.TrimSpace(reference), "file:")
}

func parseEntries(storage domain.StorageResource, parent, output string) ([]domain.FileEntry, error) {
	entries := []domain.FileEntry{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 4 {
			return nil, fmt.Errorf("invalid SSH storage listing")
		}
		size, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, err
		}
		seconds, _ := strconv.ParseFloat(fields[3], 64)
		typ := domain.FileEntryFile
		if fields[1] == "d" {
			typ = domain.FileEntryDirectory
		}
		if fields[1] == "l" {
			typ = domain.FileEntrySymlink
		}
		linkTarget := ""
		if len(fields) > 4 {
			linkTarget = fields[4]
		}
		entries = append(entries, domain.FileEntry{
			StorageID: storage.ID, Path: path.Join(parent, fields[0]), Name: fields[0], Type: typ,
			SizeBytes: size, ModifiedAt: time.Unix(int64(seconds), 0).UTC(), Readable: true,
			Writable: !storage.ReadOnly, LinkTarget: linkTarget,
		})
	}
	return entries, nil
}

const commonScript = `decode(){ printf '%s' "$1" | base64 -d; }
root=$(decode "$1") || exit 2
rel=$(decode "$2") || exit 2
root=$(realpath -e -- "$root") || exit 2
candidate="$root${rel:+/$rel}"
safe_existing(){
  resolved=$(realpath -e -- "$candidate") || return 1
  case "$resolved" in "$root"|"$root"/*) ;; *) return 1;; esac
}`
const listScript = commonScript + "\n" + `safe_existing || exit 3; test -d "$resolved" || exit 4; find -P "$resolved" -mindepth 1 -maxdepth 1 -printf '%f\t%y\t%s\t%T@\t%l\n'`
const statScript = commonScript + "\n" + `safe_existing || exit 3; find -P "$resolved" -maxdepth 0 -printf '%f\t%y\t%s\t%T@\t%l\n'`
const catScript = commonScript + "\n" + `safe_existing || exit 3; test -f "$resolved" || exit 4; cat -- "$resolved"`
const removeScript = commonScript + "\n" + `safe_existing || exit 3; test ! -d "$resolved" || exit 4; rm -- "$resolved"`
const writeScript = commonScript + "\n" + `parent=$(dirname -- "$candidate")
parent=$(realpath -e -- "$parent") || exit 3
case "$parent" in "$root"|"$root"/*) ;; *) exit 4;; esac
target="$parent/$(basename -- "$candidate")"
tmp="$target.akoflow-copy-part"
cat > "$tmp" && mv -f -- "$tmp" "$target"`

var _ ports.StorageBrowser = (*Driver)(nil)
