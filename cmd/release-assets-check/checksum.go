package main

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func verifyFile(directory string, artifact releaseFile) error {
	if artifact.URL == "" || artifact.URL == "." || filepath.Base(artifact.URL) != artifact.URL || strings.ContainsAny(artifact.URL, "/\\") {
		return fmt.Errorf("artifact must be a local basename: %q", artifact.URL)
	}
	want, err := base64.StdEncoding.DecodeString(artifact.SHA512)
	if err != nil || len(want) != sha512.Size {
		return fmt.Errorf("invalid SHA512 for %q", artifact.URL)
	}
	path := filepath.Join(directory, artifact.URL)
	stat, err := os.Lstat(path)
	if err != nil || !stat.Mode().IsRegular() {
		return fmt.Errorf("artifact must exist as a regular non-symlink file: %q", artifact.URL)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha512.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return err
	}
	if size <= 0 || (artifact.Size != nil && size != *artifact.Size) || !bytes.Equal(want, hash.Sum(nil)) {
		return fmt.Errorf("distributed artifact does not match updater size/SHA512: %q", artifact.URL)
	}
	return nil
}
