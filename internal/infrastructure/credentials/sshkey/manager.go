package sshkey

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var validID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

type Key struct {
	ID            string `json:"id"`
	CredentialRef string `json:"credentialRef"`
	PublicKey     string `json:"publicKey"`
	Fingerprint   string `json:"fingerprint"`
}

// Manager owns private service keys in an ignored directory. Only public
// material is ever returned to the caller.
type Manager struct{ directory string }

func New(directory string) *Manager { return &Manager{directory: directory} }

func (m *Manager) Generate(id, comment string) (Key, error) {
	if !validID.MatchString(id) {
		return Key{}, fmt.Errorf("invalid SSH key id")
	}
	if err := os.MkdirAll(m.directory, 0700); err != nil {
		return Key{}, err
	}
	privatePath := filepath.Join(m.directory, id)
	if _, err := os.Lstat(privatePath); err == nil {
		return Key{}, fmt.Errorf("SSH key %q already exists", id)
	} else if !os.IsNotExist(err) {
		return Key{}, err
	}
	command := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", strings.TrimSpace(comment), "-f", privatePath)
	if output, err := command.CombinedOutput(); err != nil {
		return Key{}, fmt.Errorf("generate OpenSSH key: %w: %s", err, strings.TrimSpace(string(output)))
	}
	if err := os.Chmod(privatePath, 0600); err != nil {
		return Key{}, err
	}
	publicBytes, err := os.ReadFile(privatePath + ".pub")
	if err != nil {
		return Key{}, err
	}
	publicKey := strings.TrimSpace(string(publicBytes))
	public, err := publicMaterial(publicKey)
	if err != nil {
		_ = os.Remove(privatePath)
		_ = os.Remove(privatePath + ".pub")
		return Key{}, err
	}
	return Key{ID: id, CredentialRef: "file:" + privatePath, PublicKey: publicKey, Fingerprint: fingerprint(public)}, nil
}

// List returns the public metadata for service keys managed by this Engine.
// Private material never leaves the credential directory or this process.
func (m *Manager) List() ([]Key, error) {
	entries, err := os.ReadDir(m.directory)
	if os.IsNotExist(err) {
		return []Key{}, nil
	}
	if err != nil {
		return nil, err
	}
	keys := make([]Key, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pub") {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".pub")
		if !validID.MatchString(id) {
			continue
		}
		privatePath := filepath.Join(m.directory, id)
		if info, statErr := os.Stat(privatePath); statErr != nil || info.IsDir() {
			continue
		}
		publicBytes, readErr := os.ReadFile(filepath.Join(m.directory, entry.Name()))
		if readErr != nil {
			return nil, readErr
		}
		publicKey := strings.TrimSpace(string(publicBytes))
		public, parseErr := publicMaterial(publicKey)
		if parseErr != nil {
			return nil, fmt.Errorf("read SSH key %q: %w", id, parseErr)
		}
		keys = append(keys, Key{
			ID:            id,
			CredentialRef: "file:" + privatePath,
			PublicKey:     publicKey,
			Fingerprint:   fingerprint(public),
		})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
	return keys, nil
}

func publicMaterial(value string) (ed25519.PublicKey, error) {
	fields := strings.Fields(value)
	if len(fields) < 2 || fields[0] != "ssh-ed25519" {
		return nil, fmt.Errorf("generated key is not ssh-ed25519")
	}
	wire, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return nil, err
	}
	if len(wire) < 4 {
		return nil, fmt.Errorf("invalid SSH public key")
	}
	algorithmSize := int(binary.BigEndian.Uint32(wire[:4]))
	offset := 4 + algorithmSize
	if len(wire) < offset+4 {
		return nil, fmt.Errorf("invalid SSH public key")
	}
	keySize := int(binary.BigEndian.Uint32(wire[offset : offset+4]))
	offset += 4
	if keySize != ed25519.PublicKeySize || len(wire) != offset+keySize {
		return nil, fmt.Errorf("invalid ed25519 public key")
	}
	return ed25519.PublicKey(wire[offset:]), nil
}

func sshString(value []byte) []byte {
	encoded := make([]byte, 4+len(value))
	binary.BigEndian.PutUint32(encoded[:4], uint32(len(value)))
	copy(encoded[4:], value)
	return encoded
}

func fingerprint(public ed25519.PublicKey) string {
	value := sha256.Sum256(append(sshString([]byte("ssh-ed25519")), sshString(public)...))
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(value[:])
}
