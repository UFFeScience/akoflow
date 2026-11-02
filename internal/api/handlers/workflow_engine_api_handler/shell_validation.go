package workflow_engine_api_handler

import (
	"errors"
	"net/http"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

const maxShellCommandBytes = 64 * 1024

type shellDiagnostic struct {
	Message string `json:"message"`
	Line    uint   `json:"line,omitempty"`
	Column  uint   `json:"column,omitempty"`
}

type shellValidation struct {
	Valid       bool              `json:"valid"`
	Diagnostics []shellDiagnostic `json:"diagnostics"`
}

// ValidateShellCommand parses POSIX sh without invoking an interpreter or
// expanding substitutions. Invalid scripts are a successful validation response.
func (h *Handler) ValidateShellCommand(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8*maxShellCommandBytes)
	var request struct {
		Command string `json:"command"`
	}
	if !decode(w, r, &request) {
		return
	}
	result := shellValidation{Diagnostics: []shellDiagnostic{}}
	switch {
	case strings.TrimSpace(request.Command) == "":
		result.Diagnostics = append(result.Diagnostics, shellDiagnostic{Message: "Enter a shell command."})
	case len(request.Command) > maxShellCommandBytes:
		result.Diagnostics = append(result.Diagnostics, shellDiagnostic{Message: "Shell commands must not exceed 64 KiB."})
	case strings.ContainsRune(request.Command, 0):
		result.Diagnostics = append(result.Diagnostics, shellDiagnostic{Message: "Shell commands cannot contain NUL bytes."})
	default:
		parser := syntax.NewParser(syntax.Variant(syntax.LangPOSIX))
		file, err := parser.Parse(strings.NewReader(request.Command), "")
		if err != nil {
			diagnostic := shellDiagnostic{Message: err.Error()}
			var parseError syntax.ParseError
			var langError syntax.LangError
			if errors.As(err, &parseError) {
				diagnostic = shellDiagnostic{Message: parseError.Text, Line: parseError.Pos.Line(), Column: parseError.Pos.Col()}
			} else if errors.As(err, &langError) {
				diagnostic.Line, diagnostic.Column = langError.Pos.Line(), langError.Pos.Col()
			}
			result.Diagnostics = append(result.Diagnostics, diagnostic)
		} else if len(file.Stmts) == 0 {
			result.Diagnostics = append(result.Diagnostics, shellDiagnostic{Message: "Enter a shell command, not only comments."})
		} else {
			result.Valid = true
		}
	}
	writeJSON(w, http.StatusOK, result)
}
