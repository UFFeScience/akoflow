package installationhealth

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func Docker(ctx context.Context) Observation {
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", "/var/run/docker.sock")
	}}
	defer transport.CloseIdleConnections()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://docker/_ping", nil)
	if err != nil {
		return Observation{Status: "unknown", Detail: "The Docker check could not be created."}
	}
	response, err := (&http.Client{Transport: transport}).Do(request)
	if err != nil {
		return Observation{Status: "unavailable", Detail: "The daemon could not connect to Docker."}
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return Observation{Status: "degraded", Detail: "Docker answered but rejected the health check."}
	}
	return Observation{Status: "healthy", Detail: "Docker answered its health check."}
}

func BuildKit(ctx context.Context, buildctl string) Observation {
	if buildctl == "" {
		buildctl = "buildctl"
	}
	output, err := exec.CommandContext(ctx, buildctl, "debug", "workers", "--format", "{{len .}}").Output()
	if errors.Is(err, exec.ErrNotFound) {
		return Observation{Status: "unknown", Detail: "The BuildKit client is unavailable in this daemon."}
	}
	if err != nil {
		return Observation{Status: "unavailable", Detail: "The configured BuildKit service could not list workers."}
	}
	count, parseErr := strconv.Atoi(strings.TrimSpace(string(output)))
	if parseErr != nil || count < 0 {
		return Observation{Status: "unknown", Detail: "BuildKit returned an unreadable worker count."}
	}
	if count == 0 {
		return Observation{Status: "degraded", Detail: "BuildKit reported zero available workers."}
	}
	return Observation{Status: "healthy", Detail: "BuildKit reported an available worker."}
}

func Database(ctx context.Context, db *sql.DB) Observation {
	var one int
	if err := db.QueryRowContext(ctx, "SELECT 1 FROM schema_metadata LIMIT 1").Scan(&one); err != nil {
		return Observation{Status: "unavailable", Detail: "The database did not complete a read check."}
	}
	return Observation{Status: "healthy", Detail: "The database completed a read check."}
}

func Storage(ctx context.Context, directory string, readOnly bool) Observation {
	entry, err := os.Stat(directory)
	if errors.Is(err, os.ErrNotExist) {
		return Observation{Status: "unknown", Detail: "The artifact directory has not been initialized."}
	}
	if err != nil || !entry.IsDir() {
		return Observation{Status: "unavailable", Detail: "The artifact directory could not be opened."}
	}
	if readOnly {
		return Observation{Status: "healthy", Detail: "The artifact directory is accessible in this read-only instance."}
	}
	if ctx.Err() != nil {
		return Observation{Status: "unavailable", Detail: "The storage check timed out."}
	}
	file, err := os.CreateTemp(directory, ".akoflow-health-")
	if err != nil {
		return Observation{Status: "degraded", Detail: "The artifact directory is accessible, but writing failed."}
	}
	closeErr := file.Close()
	removeErr := os.Remove(file.Name())
	if closeErr != nil || removeErr != nil {
		return Observation{Status: "degraded", Detail: "The artifact directory failed the write-and-cleanup check."}
	}
	return Observation{Status: "healthy", Detail: "The artifact directory passed a write-and-cleanup check."}
}
