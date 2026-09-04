package ansible

import "testing"

func TestPrivateKeyPathRejectsExternalSchemes(t *testing.T) {
	if _, err := privateKeyPath("vault:key"); err == nil {
		t.Fatal("expected unsupported credential reference")
	}
}
