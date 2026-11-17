package operationerror

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSanitizeOperationalDiagnostics(t *testing.T) {
	for _, input := range []string{
		"API_TOKEN=unique-secret", `password="unique-secret"`, "Authorization: Bearer unique-secret",
		"https://user:unique-secret@registry.example/image", "ssh://user:unique-secret@host/path",
		"https://example/image?X-Amz-Signature=unique-secret", "-----BEGIN PRIVATE KEY-----\nunique-secret\n-----END PRIVATE KEY-----",
		"-----BEGIN RSA PRIVATE KEY-----\nunique-secret",
	} {
		if strings.Contains(Sanitize(input), "unique-secret") {
			t.Fatalf("credential exposed: %q", input)
		}
	}
}
func TestStructuredFailurePreservesCauseAndDetails(t *testing.T) {
	cause := errors.New("token=unique-secret")
	err := fmt.Errorf("activity a: %w", Wrap(cause, "registry_pull_failed", "artifact_preparation", "a", true, "Check registry access."))
	if !errors.Is(err, cause) || Describe(err).EntityID != "a" || !Describe(err).Retryable || strings.Contains(err.Error(), "unique-secret") {
		t.Fatalf("invalid failure: %v %+v", err, Describe(err))
	}
}
