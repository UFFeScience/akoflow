package montagetools

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

var targetPattern = regexp.MustCompile(`^montage-gcp-[a-z0-9-]+$`)
var runPattern = regexp.MustCompile(`^montage-[a-z0-9-]+$`)

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

// Export is deliberately explicit and API-only. No command ever exposes the
// token or private console output in an error, and existing archives are safe.
func exportCloud(target, run, output string) {
	check(targetPattern.MatchString(target) && runPattern.MatchString(run), "invalid target or run identity")
	token, ok := os.LookupEnv("AKOFLOW_API_TOKEN")
	check(ok && token != "", "AKOFLOW_API_TOKEN required")
	client := http.Client{Timeout: 660 * time.Second}
	console := func(command string) string {
		payload, _ := json.Marshal(Object{"resourceId": target, "command": command, "timeoutSeconds": 600})
		request, e := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://127.0.0.1:8080/akoflow-api/console-commands/", bytes.NewReader(payload))
		check(e == nil, "console request failed")
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Content-Type", "application/json")
		response, e := client.Do(request)
		check(e == nil, "console transport failed")
		defer response.Body.Close()
		check(response.StatusCode >= 200 && response.StatusCode < 300, fmt.Sprintf("console HTTP %d", response.StatusCode))
		var result Object
		check(json.NewDecoder(io.LimitReader(response.Body, 8<<20)).Decode(&result) == nil, "invalid console response")
		check(result["status"] == "completed" && result["exitCode"] != nil && num(result["exitCode"]) == 0, "remote archive command failed")
		return str(get(result, "stdout"))
	}
	source := shellQuote("/akoflow/workspace/runs/" + run)
	archive := shellQuote("/tmp/akoflow-preserve-" + run + ".tar.gz")
	metadata := strings.Fields(console("set -eu; test -d " + source + "; tar -czf " + archive + " -C " + source + " .; stat -c %s " + archive + "; sha256sum " + archive))
	check(len(metadata) >= 2, "invalid archive metadata")
	size := int64(num(metadata[0]))
	hash := metadata[1]
	check(size > 0 && size <= 512*1024*1024 && imageDigestPattern.MatchString(hash), "invalid archive metadata")
	_, e := os.Stat(output)
	check(os.IsNotExist(e), "output already exists or inaccessible")
	partial := output + ".partial"
	file, e := os.OpenFile(partial, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	check(e == nil, "partial output already exists or cannot be created")
	defer file.Close()
	chunk := int64(4 * 1024 * 1024)
	for offset := int64(0); offset < size; offset += chunk {
		count := chunk
		if count > size-offset {
			count = size - offset
		}
		encoded := console(fmt.Sprintf("dd if=%s bs=1M skip=%d count=4 status=none | base64 -w0", archive, offset/(1024*1024)))
		decoded, e := base64.StdEncoding.Strict().DecodeString(strings.TrimSpace(encoded))
		check(e == nil && int64(len(decoded)) == count, "archive chunk length mismatch")
		_, e = file.Write(decoded)
		check(e == nil, "archive write failed")
		fmt.Printf("%s: %d/%d bytes preserved\n", target, offset+count, size)
	}
	check(file.Close() == nil, "archive close failed")
	payload, e := os.ReadFile(partial)
	check(e == nil && digest(payload) == hash, "archive checksum mismatch; partial retained")
	check(os.Rename(partial, output) == nil, "archive rename failed")
	fmt.Printf("VERIFIED %s: %d bytes SHA256 %s\n", target, size, hash)
}
