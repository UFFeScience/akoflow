// Package validation exercises every AkôFlow example fixture through the
// production request DTOs. It exists to keep the docs and examples aligned with
// the real server contract: every YAML shipped under examples/ must round-trip
// through the same decoders the server uses for HTTP requests.
//
// The package is intentionally lightweight: it does not require a running
// server, a database, or any external runtime. It is the first line of defence
// against drift between docs and code.
package validation

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// FixtureCategory groups a set of YAML files under a logical example. The Name
// is always the directory name (e.g. "direct-hello", "30gb-fanout") so callers
// can map back to a documentation path without ambiguity.
type FixtureCategory struct {
	Name      string
	Directory string
	Files     map[string]string
}

// FixtureMap indexes every fixture shipped in the repository. Keys are the
// leaf directory names of each example under examples/.
type FixtureMap map[string]FixtureCategory

// DiscoverFixtures walks the repository root for example directories and
// returns the canonical mapping of category → files. Each directory that
// contains at least one YAML/JSON file becomes a category whose Name matches
// the directory leaf. Files within the directory are indexed by basename.
//
// The function tries two root layouts in order:
//
//  1. <repositoryRoot>/examples — used for the in-tree examples shipped
//     inside the akoflow repository.
//  2. <repositoryRoot> itself — used for the standalone akoflow-examples
//     repository, whose root directory IS the examples/ tree.
//
// If neither layout exists, the function returns an empty map without an
// error so callers can decide how to react (typically: skip the test).
func DiscoverFixtures(repositoryRoot string) (FixtureMap, error) {
	categories := FixtureMap{}
	candidates := []string{
		filepath.Join(repositoryRoot, "examples"),
		repositoryRoot,
	}
	seen := map[string]bool{}
	for _, root := range candidates {
		if seen[root] {
			continue
		}
		seen[root] = true
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			continue
		}
		if err := walkFixtures(root, categories); err != nil {
			return nil, err
		}
	}
	return categories, nil
}

// walkFixtures walks examples recursively and indexes every directory that
// contains at least one fixture file. The directory name (not the file name)
// becomes the category key.
func walkFixtures(root string, categories FixtureMap) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read %s: %w", root, err)
	}
	// Aggregate all fixture files in this directory.
	own := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !isFixtureFile(entry.Name()) {
			continue
		}
		payload, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			return fmt.Errorf("read %s: %w", filepath.Join(root, entry.Name()), err)
		}
		own[entry.Name()] = string(payload)
	}
	if len(own) > 0 {
		key := filepath.Base(root)
		categories[key] = FixtureCategory{Name: key, Directory: root, Files: own}
	}
	// Recurse into subdirectories.
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		child := filepath.Join(root, entry.Name())
		// Skip fixtures directories that are not examples (e.g. data, docker,
		// expected, scripts) by only recursing into directories that contain
		// a recognised shape.
		if !containsFixture(child) {
			continue
		}
		if err := walkFixtures(child, categories); err != nil {
			return err
		}
	}
	return nil
}

func containsFixture(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			if containsFixture(filepath.Join(dir, entry.Name())) {
				return true
			}
			continue
		}
		if isFixtureFile(entry.Name()) {
			return true
		}
	}
	return false
}

func isFixtureFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".yaml", ".yml", ".json", ".sh", ".md", ".txt":
		return true
	}
	return false
}

// RepositoryRoot walks up from the current working directory until it finds
// a go.mod file. It is used by tests and the server harness to locate the
// repository root without relying on $PWD.
func RepositoryRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find repository root above %s", wd)
		}
		dir = parent
	}
}

// SortedNames returns the fixture categories in deterministic order.
func (m FixtureMap) SortedNames() []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HasFile returns true when a category ships a file with the supplied name.
func (c FixtureCategory) HasFile(name string) bool {
	_, ok := c.Files[name]
	return ok
}

// ReadFile returns the raw file payload for a fixture or an error when the
// file is missing.
func (c FixtureCategory) ReadFile(name string) (string, error) {
	payload, ok := c.Files[name]
	if !ok {
		return "", fmt.Errorf("fixture %s does not contain %s", c.Name, name)
	}
	return payload, nil
}
