package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var versionTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var checksum = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

type asset struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
}

type manifest struct {
	Schema         int                         `json:"schema"`
	Version        string                      `json:"version"`
	MinimumDesktop string                      `json:"minimumDesktop"`
	RollbackSafe   bool                        `json:"rollbackSafe"`
	Architectures  map[string]map[string]asset `json:"architectures"`
}

type envelope struct {
	Payload   []byte `json:"payload"`
	Signature []byte `json:"signature"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	switch {
	case len(args) == 3 && args[0] == "bundle":
		return bundle(args[1], args[2])
	case len(args) == 8 && args[0] == "sign":
		return signManifest(args[1], args[2], args[3], args[4], args[5], args[6], args[7])
	default:
		return fmt.Errorf("usage: component-release bundle SOURCE DESTINATION | sign DIR RELEASE DESKTOP FRONTEND DAEMON BUILDKIT PUBLIC_KEY")
	}
}

func bundle(root, destination string) error {
	files := make([][2]any, 0)
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported frontend file: %s", file)
		}
		name, err := filepath.Rel(root, file)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		files = append(files, [2]any{filepath.ToSlash(name), data})
		return nil
	})
	if err != nil {
		return err
	}
	return writeJSON(destination, files)
}

func signManifest(directory, release, desktop, frontend, daemon, buildkit, keyPath string) error {
	for _, version := range []string{release, desktop, frontend, daemon, buildkit} {
		if !versionTag.MatchString(version) {
			return fmt.Errorf("stable version tags are required for component updates")
		}
	}
	frontendAsset, err := releaseAsset(directory, release, frontend, "akoflow-frontend-"+frontend+".json")
	if err != nil {
		return err
	}
	metadata := manifest{1, release, desktop, true, make(map[string]map[string]asset)}
	for _, arch := range []string{"amd64", "arm64"} {
		daemonAsset, err := releaseAsset(directory, release, daemon, "akoflow-daemon-"+daemon+"-linux-"+arch+".tar")
		if err != nil {
			return err
		}
		buildkitAsset, err := releaseAsset(directory, release, buildkit, "akoflow-buildkit-"+buildkit+"-linux-"+arch+".tar")
		if err != nil {
			return err
		}
		metadata.Architectures[arch] = map[string]asset{"frontend": frontendAsset, "daemon": daemonAsset, "buildkit": buildkitAsset}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	signature, err := signPayload(payload, keyPath)
	if err != nil {
		return err
	}
	return writeJSON(filepath.Join(directory, "components.json"), envelope{payload, signature})
}

func releaseAsset(directory, release, version, name string) (asset, error) {
	if version == release {
		file, err := os.Open(filepath.Join(directory, name))
		if err != nil {
			return asset{}, err
		}
		defer file.Close()
		hash := sha256.New()
		if _, err = io.Copy(hash, file); err != nil {
			return asset{}, err
		}
		return asset{version, hex.EncodeToString(hash.Sum(nil))}, nil
	}
	data, err := exec.Command("gh", "api", "repos/UFFeScience/akoflow/releases/tags/"+version).Output()
	if err != nil {
		return asset{}, fmt.Errorf("read release %s: %w", version, err)
	}
	var published struct {
		Assets []struct{ Name, Digest string }
	}
	if err := json.Unmarshal(data, &published); err != nil {
		return asset{}, err
	}
	for _, file := range published.Assets {
		if file.Name == name && checksum.MatchString(file.Digest) {
			return asset{version, strings.TrimPrefix(file.Digest, "sha256:")}, nil
		}
	}
	return asset{}, fmt.Errorf("missing published checksum for %s/%s", version, name)
}

func signPayload(payload []byte, keyPath string) ([]byte, error) {
	block, _ := pem.Decode([]byte(os.Getenv("AKOFLOW_COMPONENT_SIGNING_KEY")))
	if block == nil {
		return nil, fmt.Errorf("component signing key is unavailable")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	key, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok {
		return nil, fmt.Errorf("component signing key must be an Ed25519 PKCS#8 key")
	}
	publicPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}
	publicBlock, _ := pem.Decode(publicPEM)
	if publicBlock == nil {
		return nil, fmt.Errorf("invalid Desktop trust anchor")
	}
	public, err := x509.ParsePKIXPublicKey(publicBlock.Bytes)
	trusted, ok := public.(ed25519.PublicKey)
	if err != nil || !ok || !key.Public().(ed25519.PublicKey).Equal(trusted) {
		return nil, fmt.Errorf("the signing key does not match the Desktop trust anchor")
	}
	return ed25519.Sign(key, payload), nil
}

func writeJSON(destination string, value any) error {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	encodeErr := json.NewEncoder(file).Encode(value)
	closeErr := file.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}
