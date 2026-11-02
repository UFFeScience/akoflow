package transfer

import (
	"math"
	"strconv"
	"strings"
)

func remoteMergeSeconds(output, stage string) float64 {
	prefix := "__AKOFLOW_MERGE_" + stage + "_SECONDS__="
	for _, line := range strings.Split(output, "\n") {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			n, err := strconv.ParseFloat(value, 64)
			if err == nil && n >= 0 && !math.IsNaN(n) && !math.IsInf(n, 0) {
				return n
			}
		}
	}
	return 0
}

// AKO-131: retain the whole-workspace conflict/publication contract without
// forking sha256sum/cat and creating three metadata files for every path.
func remoteWorkspaceScript(sources []string, destination string) string {
	args := []string{"python3", "-", destination}
	args = append(args, sources...)
	for i := range args {
		args[i] = shell(args[i])
	}
	return strings.Join(args, " ") + " <<'AKOFLOW_MERGE_PY'\n" + remoteWorkspaceMergePython + "\nAKOFLOW_MERGE_PY\n"
}

const remoteWorkspaceMergePython = `import hashlib, os, stat, subprocess, sys, time
destination, sources = sys.argv[1], sys.argv[2:]
expected = {}
directories = set()

def digest(path):
    h = hashlib.sha256()
    with open(path, 'rb') as f:
        for chunk in iter(lambda: f.read(1024 * 1024), b''):
            h.update(chunk)
    return h.digest()

def tree(root):
    if not os.path.isdir(root) or os.path.islink(root):
        raise RuntimeError('source workspace disappeared or is a symlink: ' + root)
    for parent, dirs, files in os.walk(root, followlinks=False):
        for name in dirs:
            if os.path.islink(os.path.join(parent, name)):
                raise RuntimeError('workspace contains directory symlink: ' + name)
            directories.add(os.path.relpath(os.path.join(parent, name), root))
        for name in files:
            path = os.path.join(parent, name)
            if not stat.S_ISREG(os.lstat(path).st_mode):
                raise RuntimeError('workspace contains symlink or special file: ' + name)
            yield os.path.relpath(path, root), path

try:
    os.makedirs(destination, exist_ok=True)
    started = time.monotonic()
    for root in [destination] + sources:
        for rel, path in tree(root):
            value = digest(path)
            previous = expected.get(rel)
            if previous is not None and previous[0] != value:
                raise RuntimeError('workspace content conflict at ' + rel + ' between ' + previous[1] + ' and ' + root)
            expected.setdefault(rel, (value, root))
    if directories.intersection(expected):
        raise RuntimeError('workspace file/directory conflict at ' + sorted(directories.intersection(expected))[0])
    print('__AKOFLOW_MERGE_CHECK_SECONDS__=' + str(time.monotonic() - started), flush=True)
    started = time.monotonic()
    for index, source in enumerate(sources):
        print('__AKOFLOW_WORKSPACE_BEGIN_' + str(index) + '__', flush=True)
        subprocess.run(['rsync', '--ignore-existing', '-r', '--checksum', '--stats', '--out-format=%i %n',
                        '--no-perms', '--no-owner', '--no-group', '--no-times', '--',
                        os.path.join(source, ''), os.path.join(destination, '')], check=True)
        print('__AKOFLOW_WORKSPACE_END_' + str(index) + '__', flush=True)
    print('__AKOFLOW_MERGE_COPY_SECONDS__=' + str(time.monotonic() - started), flush=True)
    started = time.monotonic()
    actual_paths = set()
    for rel, path in tree(destination):
        actual_paths.add(rel)
        if rel not in expected or digest(path) != expected[rel][0]:
            raise RuntimeError('workspace verification failed at ' + rel)
    if actual_paths != set(expected):
        raise RuntimeError('workspace verification failed: missing destination paths')
    print('__AKOFLOW_MERGE_VERIFIED_FILES__=' + str(len(actual_paths)), flush=True)
    print('__AKOFLOW_MERGE_VERIFY_SECONDS__=' + str(time.monotonic() - started), flush=True)
except Exception as error:
    print(str(error), file=sys.stderr)
    sys.exit(45)
`
