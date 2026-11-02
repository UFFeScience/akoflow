package workflow_engine_api_handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellCommandValidation(t *testing.T) {
	tests := []struct {
		name, command string
		valid         bool
	}{
		{"simple", "echo hello", true},
		{"pipeline", "cat input | sort > output", true},
		{"multiline", "for x in a b; do\n echo \"$x\"\ndone", true},
		{"heredoc", "cat <<EOF\nhello\nEOF\n", true},
		{"quotes", "echo 'unclosed", false},
		{"conditional", "if true; then echo yes", false},
		{"pipeline missing command", "echo hi |", false},
		{"bash array", "items=(a b)", false},
		{"empty", "  ", false},
		{"comments", "# comment", false},
		{"nul", "echo \x00", false},
		{"oversize", strings.Repeat("a", maxShellCommandBytes+1), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validateShellForTest(t, tt.command)
			if result.Valid != tt.valid {
				t.Fatalf("got %#v, want valid %v", result, tt.valid)
			}
			if !tt.valid && len(result.Diagnostics) == 0 {
				t.Fatal("missing diagnostic")
			}
			if tt.name == "quotes" && (result.Diagnostics[0].Line == 0 || result.Diagnostics[0].Column == 0) {
				t.Fatal("missing error position")
			}
		})
	}
}

func validateShellForTest(t *testing.T, command string) shellValidation {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"command": command})
	request := httptest.NewRequest(http.MethodPost, "/shell-command-validations/", strings.NewReader(string(body)))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	(&Handler{}).ValidateShellCommand(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status %d: %s", recorder.Code, recorder.Body.String())
	}
	var result shellValidation
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestShellValidationNeverExecutes(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	result := validateShellForTest(t, "echo $(touch '"+marker+"'); touch '"+marker+"'")
	if !result.Valid {
		t.Fatalf("unexpected diagnostic: %#v", result)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("validation executed a command")
	}
}

func TestShellValidationRejectsMalformedRequest(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/shell-command-validations/", strings.NewReader(`{"command":`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	(&Handler{}).ValidateShellCommand(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status %d", recorder.Code)
	}
}
