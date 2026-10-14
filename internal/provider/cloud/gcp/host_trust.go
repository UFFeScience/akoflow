package gcp

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

// Enroll preserves previous generations and refuses to overwrite a key for an
// existing generation. Callers must obtain identity through VerifyHostIdentity.
func (identity HostIdentity) Enroll(root, logicalID string) (alias, path string, err error) {
	if identity.Generation == "" || logicalID == "" {
		return "", "", fmt.Errorf("host trust requires logical and provider generation identities")
	}
	key, _, options, rest, err := ssh.ParseAuthorizedKey([]byte(identity.PublicKey))
	if err != nil || len(options) != 0 || len(strings.TrimSpace(string(rest))) != 0 || key.Type() != ssh.KeyAlgoED25519 || ssh.FingerprintSHA256(key) != identity.Fingerprint {
		return "", "", fmt.Errorf("invalid verified host identity")
	}
	alias = fmt.Sprintf("akoflow-generation-%x", sha256.Sum256([]byte(logicalID+"\x00"+identity.Generation)))
	path = filepath.Join(root, alias+".known_hosts")
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", "", err
	}
	content := append([]byte(alias+" "), ssh.MarshalAuthorizedKey(key)...)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if os.IsExist(err) {
		info, statErr := os.Lstat(path)
		if statErr != nil || !info.Mode().IsRegular() {
			return "", "", fmt.Errorf("unsafe host trust record")
		}
		previous, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(previous, content) {
			return "", "", fmt.Errorf("host key changed for verified provider generation")
		}
		return alias, path, nil
	}
	if err != nil {
		return "", "", err
	}
	_, writeErr := file.Write(content)
	closeErr := file.Close()
	if writeErr != nil {
		return "", "", writeErr
	}
	if closeErr != nil {
		return "", "", closeErr
	}
	return alias, path, nil
}
