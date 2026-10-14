package sshfilesystem

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/domain"
	"github.com/UFFeScience/akoflow/internal/domain/storagebudget"
)

// WorkspaceBudget reads the mounted filesystem and its current user's quota.
// Unknown quota formats fail closed; filesystem free space alone cannot prove
// the user can write. Project/group quotas require a separate policy observer.
func (d *Driver) WorkspaceBudget(ctx context.Context, storage domain.StorageResource, value string) (storagebudget.Snapshot, error) {
	root, relative, err := browseRoot(storage, value)
	if err != nil {
		return storagebudget.Snapshot{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := d.run(ctx, storage, workspaceBudgetScript, root, relative)
	if err != nil {
		return storagebudget.Snapshot{}, fmt.Errorf("observe filesystem/user quota: %w", err)
	}
	available, filesystem, uid, err := parseWorkspaceBudget(string(output))
	if err != nil {
		return storagebudget.Snapshot{}, err
	}
	// Account identity is shared across logical nodes of the same connection;
	// do not expose a remote filesystem hostname or user name in the snapshot.
	identity := metadataString(storage.Metadata, "connectionId") + "\x00" + filesystem + "\x00" + uid
	return storagebudget.Snapshot{StorageID: fmt.Sprintf("ssh-user-quota-%x", sha256.Sum256([]byte(identity))), AvailableBytes: available, ObservedAt: time.Now().UTC(), Source: "live-posix-filesystem-and-user-quota-v1"}, nil
}

const workspaceBudgetScript = commonScript + "\n" + `
set -eu
safe_existing || exit 65
target="$resolved"
test -d "$target"
usage=$(LC_ALL=C df -Pk -- "$target")
mount=$(printf '%s\n' "$usage" | awk 'NR==2 { print $6 }')
[ -n "$mount" ] || exit 65
printf '__AKO_DF__\n%s\n__AKO_UID__\n' "$usage"
id -u
printf '__AKO_QUOTA__\n'
if LC_ALL=C quota -w -v -p -f "$mount" 2>&1; then
  :
else
  result=$?
  # Over-quota tools may return 1 with valid data. Other errors are unknown.
  [ "$result" -eq 1 ] || exit "$result"
fi
printf '__AKO_END__\n'
`

func parseWorkspaceBudget(output string) (int64, string, string, error) {
	section, filesystem, uid := "", "", ""
	var available int64
	matched := false
	number := func(value string) (int64, error) {
		return strconv.ParseInt(strings.TrimSuffix(value, "*"), 10, 64)
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch line {
		case "__AKO_DF__", "__AKO_UID__", "__AKO_QUOTA__", "__AKO_END__":
			section = line
			continue
		}
		fields := strings.Fields(line)
		switch section {
		case "__AKO_DF__":
			if len(fields) != 6 || fields[0] == "Filesystem" {
				continue
			}
			free, err := number(fields[3])
			if err != nil || free < 0 || free > math.MaxInt64/1024 || filesystem != "" {
				return 0, "", "", fmt.Errorf("invalid or ambiguous filesystem capacity")
			}
			filesystem, available = fields[0], free*1024
		case "__AKO_UID__":
			if _, err := strconv.ParseUint(line, 10, 64); err != nil || uid != "" {
				return 0, "", "", fmt.Errorf("invalid quota account identity")
			}
			uid = line
		case "__AKO_QUOTA__":
			if len(fields) == 0 || fields[0] != filesystem {
				continue
			}
			if len(fields) != 9 || matched {
				return 0, "", "", fmt.Errorf("invalid or ambiguous quota row")
			}
			used, err := number(fields[1])
			if err != nil || used < 0 {
				return 0, "", "", fmt.Errorf("invalid quota usage")
			}
			for _, index := range []int{2, 3} {
				limit, err := number(fields[index])
				if err != nil || limit < 0 {
					return 0, "", "", fmt.Errorf("invalid quota limit")
				}
				if limit == 0 {
					continue
				} // Explicit unlimited, not missing data.
				free := max(int64(0), limit-used)
				if free > math.MaxInt64/1024 {
					return 0, "", "", fmt.Errorf("quota capacity overflow")
				}
				available = min(available, free*1024)
			}
			matched = true
		}
	}
	if !matched || filesystem == "" || uid == "" || section != "__AKO_END__" {
		return 0, "", "", fmt.Errorf("effective user quota unavailable for workspace filesystem")
	}
	return available, filesystem, uid, nil
}
