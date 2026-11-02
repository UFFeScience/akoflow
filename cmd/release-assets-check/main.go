package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type releaseManifest struct {
	Version string        `yaml:"version"`
	Files   []releaseFile `yaml:"files"`
	Path    string        `yaml:"path"`
	SHA512  string        `yaml:"sha512"`
}

type releaseFile struct {
	URL    string `yaml:"url"`
	SHA512 string `yaml:"sha512"`
	Size   *int64 `yaml:"size"`
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("release-assets-check", flag.ContinueOnError)
	directory := flags.String("dir", "", "Directory containing packages and updater manifests")
	version := flags.String("version", "", "Expected release version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	want := strings.TrimPrefix(*version, "v")
	if *directory == "" || want == "" || flags.NArg() != 0 {
		return fmt.Errorf("--dir and --version are required; positional arguments are not allowed")
	}
	manifests, err := filepath.Glob(filepath.Join(*directory, "latest*.yml"))
	if err != nil || len(manifests) == 0 {
		return fmt.Errorf("updater manifests are required in %q: %v", *directory, err)
	}
	for _, path := range manifests {
		if err := verifyManifest(path, *directory, want); err != nil {
			return err
		}
		fmt.Fprintln(output, "Verified updater manifest:", filepath.Base(path))
	}
	return nil
}

func verifyManifest(path, directory, version string) error {
	encoded, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var manifest releaseManifest
	if err := yaml.Unmarshal(encoded, &manifest); err != nil {
		return fmt.Errorf("decode updater manifest: %w", err)
	}
	if manifest.Version != version || len(manifest.Files) == 0 {
		return fmt.Errorf("invalid manifest version/files in %q", filepath.Base(path))
	}
	if manifest.Path != "" || manifest.SHA512 != "" {
		if manifest.Path != manifest.Files[0].URL || manifest.SHA512 != manifest.Files[0].SHA512 {
			return fmt.Errorf("legacy updater path/SHA512 disagree with files in %q", filepath.Base(path))
		}
	}
	seen := make(map[string]bool, len(manifest.Files))
	for _, file := range manifest.Files {
		if seen[file.URL] || !strings.Contains(file.URL, version) {
			return fmt.Errorf("duplicate or wrong-version artifact %q", file.URL)
		}
		seen[file.URL] = true
		if err := verifyFile(directory, file); err != nil {
			return err
		}
	}
	return nil
}
