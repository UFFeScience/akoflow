package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type snapshot struct {
	UpdatedAt string    `json:"updatedAt"`
	Releases  []release `json:"releases"`
}

func synchronize(ctx context.Context, client *http.Client, endpoint, token, output string, warnings io.Writer) error {
	history, err := fetchHistory(ctx, client, endpoint, token)
	if err != nil {
		if validSnapshot(output) {
			fmt.Fprintf(warnings, "GitHub unavailable; retaining the saved public release history: %v\n", err)
			return nil
		}
		return err
	}
	value := snapshot{UpdatedAt: time.Now().UTC().Format(time.RFC3339), Releases: history}
	if err := writeSnapshot(output, value); err != nil {
		return err
	}
	fmt.Fprintf(warnings, "Synchronized %d public GitHub releases.\n", len(history))
	return nil
}

func validSnapshot(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var value snapshot
	if json.Unmarshal(data, &value) != nil || value.Releases == nil || value.UpdatedAt == "" {
		return false
	}
	for _, item := range value.Releases {
		if item.Draft || item.Tag == "" {
			return false
		}
	}
	return true
}

func writeSnapshot(path string, value snapshot) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".releases-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0644); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(append(data, '\n')); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
