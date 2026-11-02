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
if [ "$(findmnt -rn -T "$target" -o FSTYPE)" = beegfs ]; then
  # Bind the query to the mounted client configuration. Fail on ambiguity.
  cfg=$(findmnt -rn -T "$target" -o OPTIONS | tr ',' '\n' | sed -n 's/^cfgFile=//p')
  [ -n "$cfg" ] || cfg=/etc/beegfs/beegfs-client.conf
  for account in "u:$(id -u)" $(id -G | tr ' ' '\n' | sed 's/^/g:/'); do
    type="${account%%:*}"; account_id="${account#*:}"
    printf '__AKO_BEE_QUOTA__ %s\n' "$account_id"
    if [ "$type" = u ]; then
      beegfs-ctl --getquota --uid "$account_id" --csv --cfgFile="$cfg"
    else
      beegfs-ctl --getquota --gid "$account_id" --csv --cfgFile="$cfg"
    fi
  done
elif LC_ALL=C quota -w -v -p -f "$mount" 2>&1; then
  :
else
  result=$?
  # Over-quota tools may return 1 with valid data. Other errors are unknown.
  [ "$result" -eq 1 ] || exit "$result"
fi
printf '__AKO_END__\n'
`

func parseWorkspaceBudget(output string) (int64, string, string, error) {
	if strings.Contains(output, "__AKO_BEE_QUOTA__") {
		var err error
		output, err = normalizeBeeGFSQuota(output)
		if err != nil {
			return 0, "", "", err
		}
	}
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

// BeeGFS CSV uses bytes; require one exact account row per queried UID/GID.
// Multiple pools or missing/unparseable limits fail closed.
func normalizeBeeGFSQuota(output string) (string, error) {
	start := strings.Index(output, "__AKO_QUOTA__\n")
	end := strings.Index(output, "__AKO_END__")
	if start < 0 || end < start {
		return "", fmt.Errorf("truncated BeeGFS quota")
	}
	prefix := output[:start]
	filesystem := ""
	for _, line := range strings.Split(prefix, "\n") {
		f := strings.Fields(line)
		if len(f) == 6 && f[0] != "Filesystem" {
			filesystem = f[0]
		}
	}
	if filesystem == "" {
		return "", fmt.Errorf("BeeGFS filesystem missing")
	}
	account := ""
	matched := false
	queries := 0
	free := int64(math.MaxInt64)
	for _, line := range strings.Split(output[start:end], "\n") {
		if strings.HasPrefix(line, "__AKO_BEE_QUOTA__ ") {
			if account != "" && !matched {
				return "", fmt.Errorf("missing BeeGFS account quota")
			}
			account = strings.TrimPrefix(line, "__AKO_BEE_QUOTA__ ")
			if _, e := strconv.ParseUint(account, 10, 64); e != nil {
				return "", e
			}
			matched = false
			queries++
			continue
		}
		f := strings.Split(strings.TrimSpace(line), ",")
		if len(f) != 6 || f[1] != account || account == "" {
			continue
		}
		if matched {
			return "", fmt.Errorf("ambiguous BeeGFS quota pools")
		}
		matched = true
		used, e := strconv.ParseInt(f[2], 10, 64)
		if e != nil || used < 0 {
			return "", fmt.Errorf("invalid BeeGFS usage")
		}
		if f[3] == "unlimited" {
			continue
		}
		limit, e := strconv.ParseInt(f[3], 10, 64)
		if e != nil || limit < 0 {
			return "", fmt.Errorf("invalid BeeGFS hard quota")
		}
		free = min(free, max(int64(0), limit-used))
	}
	if queries < 2 || !matched {
		return "", fmt.Errorf("incomplete BeeGFS user/group quota")
	}
	// Normalize to the existing POSIX parser, rounding remaining bytes down.
	return prefix + "__AKO_QUOTA__\n" + fmt.Sprintf("%s 1 %d %d 0 0 0 0 0\n", filesystem, free/1024+1, free/1024+1) + "__AKO_END__\n", nil
}
