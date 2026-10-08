package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
)

func peerSnapshotKey(source domain.TransferEndpoint) string {
	if !peerArchiveDigest.MatchString(source.WorkspaceSnapshotID) || source.CloudInstanceID == "" || len(source.WorkspaceSnapshotEntries) == 0 {
		return ""
	}
	// Include verified connection URI (host alias, known-host generation and
	// workspace path), not a capacity target or an address alone.
	manifest, err := json.Marshal(source.WorkspaceSnapshotEntries)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256([]byte(source.CloudInstanceID + "\x00" + source.ConnectionID + "\x00" + source.URI + "\x00" + source.WorkspaceSnapshotID + "\x00" + string(manifest)))
	return fmt.Sprintf("%x", digest)
}

func verifyPeerSnapshotSource(ctx context.Context, source domain.TransferEndpoint) error {
	host, origin, err := sshTarget(source, "")
	if err != nil {
		return err
	}
	manifest, err := json.Marshal(source.WorkspaceSnapshotEntries)
	if err != nil {
		return err
	}
	command := "python3 -c " + shell(peerSnapshotManifestScript) + " " + shell(origin) + " " + shell(base64.StdEncoding.EncodeToString(manifest))
	if _, err := runSSHCombinedOutput(ctx, source, append(sshArgs(source), host, command), nil); err != nil {
		return fmt.Errorf("reused peer snapshot source differs from sealed manifest: %w", err)
	}
	return nil
}

func buildPeerSnapshot(ctx context.Context, source domain.TransferEndpoint) (_ *peerSnapshot, err error) {
	host, origin, err := sshTarget(source, "")
	if err != nil {
		return nil, err
	}
	output, err := runSSHProtocol(ctx, source, append(sshArgs(source), host, "mktemp -d /tmp/akoflow-peer-snapshot-XXXXXX"))
	if err != nil {
		return nil, err
	}
	root := strings.TrimSpace(string(output))
	if !filepath.IsAbs(root) || filepath.Dir(root) != "/tmp" || !strings.HasPrefix(filepath.Base(root), "akoflow-peer-snapshot-") {
		return nil, fmt.Errorf("invalid peer snapshot directory")
	}
	cleanup := func() {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		_, _ = runSSHCombinedOutput(cleanupCtx, source, append(sshArgs(source), host, "rm -rf -- "+shell(root)), nil)
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	verify := ""
	if peerSnapshotKey(source) != "" {
		manifest, marshalErr := json.Marshal(source.WorkspaceSnapshotEntries)
		if marshalErr != nil {
			return nil, marshalErr
		}
		verify = "python3 -c " + shell(peerSnapshotManifestScript) + " " + shell(origin) + " " + shell(base64.StdEncoding.EncodeToString(manifest)) + "; "
	}
	archive := filepath.Join(root, "workspace.tar.gz")
	command := "set -eu; umask 077; " + verify + "bad=$(find " + shell(origin) + " ! -type d ! -type f -print -quit); test -z \"$bad\"; " +
		"bash -o pipefail -c " + shell("tar --hard-dereference -C \"$1\" -cf - . | gzip -1 > \"$2\"") + " archive " + shell(origin) + " " + shell(archive) + "; " + verify +
		"sha256sum -- " + shell(archive) + "; wc -c < " + shell(archive)
	metadata, err := runSSHProtocol(ctx, source, append(sshArgs(source), host, command))
	if err != nil {
		return nil, fmt.Errorf("build sealed peer snapshot: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(metadata)), "\n")
	if len(lines) != 2 || len(strings.Fields(lines[0])) < 1 {
		return nil, fmt.Errorf("invalid peer snapshot metadata")
	}
	digest := strings.Fields(lines[0])[0]
	size, parseErr := strconv.ParseInt(strings.TrimSpace(lines[1]), 10, 64)
	if parseErr != nil || size <= 0 || !peerArchiveDigest.MatchString(digest) {
		return nil, fmt.Errorf("invalid peer snapshot digest or size")
	}
	return &peerSnapshot{root: root, digest: digest, size: size, cleanup: cleanup}, nil
}

// Validate the entire file set before AND after snapshot creation. A sealed
// manifest is not permission to reuse bytes from a workspace that has mutated.
const peerSnapshotManifestScript = `
import os, sys, json, base64, hashlib, stat
root = sys.argv[1]
entries = json.loads(base64.b64decode(sys.argv[2], validate=True))
expected = {}
for entry in entries:
    name = entry['path']
    if not name or name == '.' or name.startswith('/') or os.path.normpath(name) != name or name.startswith('../') or name in expected:
        raise RuntimeError('invalid sealed snapshot path')
    expected[name] = entry
actual = set()
for directory, dirs, files in os.walk(root, followlinks=False):
    for name in dirs:
        if not stat.S_ISDIR(os.lstat(os.path.join(directory, name)).st_mode):
            raise RuntimeError('non-directory snapshot entry')
    for name in files:
        file = os.path.join(directory, name)
        if not stat.S_ISREG(os.lstat(file).st_mode):
            raise RuntimeError('non-regular snapshot entry')
        rel = os.path.relpath(file, root)
        entry = expected.get(rel)
        if entry is None or os.path.getsize(file) != entry['sizeBytes']:
            raise RuntimeError('sealed snapshot file set or size changed')
        digest = hashlib.sha256()
        with open(file, 'rb') as stream:
            for block in iter(lambda: stream.read(1048576), b''):
                digest.update(block)
        if 'sha256:' + digest.hexdigest() != entry['digest']:
            raise RuntimeError('sealed snapshot content changed')
        actual.add(rel)
if actual != set(expected):
    raise RuntimeError('sealed snapshot file missing')
`
