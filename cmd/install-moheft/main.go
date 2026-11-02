// install-moheft exports the reviewed plugin and optionally registers it through
// the authenticated daemon API. It never overwrites an existing algorithm.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UFFeScience/akoflow/internal/planning/library"
	"github.com/UFFeScience/akoflow/internal/planning/moheftplugin"
)

func main() {
	api := flag.String("api", "", "Akoflow API base URL; omitted exports only")
	output := flag.String("output", "outputs/scheduling-plugins", "Directory for Go source and portable JSON")
	flag.Parse()
	if err := run(*api, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(api, output string) error {
	p, err := moheftplugin.Package()
	if err != nil {
		return err
	}
	body, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(output, 0700); err != nil {
		return err
	}
	for name, data := range map[string][]byte{"moheft.go": []byte(p.Source), "moheft.akoflow-scheduler.json": body, "contract.go": []byte(library.Contract), "README.md": []byte(p.Documentation)} {
		if err = os.WriteFile(filepath.Join(output, name), data, 0600); err != nil {
			return err
		}
	}
	fmt.Println("MOHEFT source and package exported to", output)
	if api == "" {
		return nil
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(api, "/")+"/scheduling-algorithms/", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := os.Getenv("AKOFLOW_API_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("daemon returned %s: %s", response.Status, payload)
	}
	var saved library.Package
	if err = json.Unmarshal(payload, &saved); err != nil {
		return err
	}
	if saved.Status != "ready" {
		return fmt.Errorf("plugin source saved but compilation failed: %s", saved.CompileError)
	}
	fmt.Printf("MOHEFT installed and compiled: revision %d (%s)\n", saved.ActiveRevision, saved.SourceHash)
	return nil
}
