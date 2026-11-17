package operationerror

import (
	"errors"
	"regexp"
)

type Details struct {
	Code          string `json:"code"`
	Stage         string `json:"stage"`
	EntityID      string `json:"entityId,omitempty"`
	Retryable     bool   `json:"retryable"`
	NextAction    string `json:"nextAction"`
	CorrelationID string `json:"correlationId,omitempty"`
}

type Failure struct {
	Details Details
	Cause   error
}

func (e *Failure) Error() string { return Sanitize(e.Cause.Error()) }
func (e *Failure) Unwrap() error { return e.Cause }
func Wrap(cause error, code, stage, entityID string, retryable bool, action string) error {
	if cause == nil {
		return nil
	}
	return &Failure{Details: Details{Code: code, Stage: stage, EntityID: entityID, Retryable: retryable, NextAction: action}, Cause: cause}
}
func Describe(err error) Details {
	var failure *Failure
	if errors.As(err, &failure) {
		return failure.Details
	}
	return Details{Code: "operation_failed", Stage: "request", NextAction: "Review the operation details before retrying."}
}

var privateKey = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?(-----END [A-Z ]*PRIVATE KEY-----|$)`)
var authorization = regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]+`)
var assignment = regexp.MustCompile(`(?i)\b([A-Z0-9_-]*(?:password|passwd|token|api[_-]?key|secret|private[_-]?key|credential)[A-Z0-9_-]*)(["']?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
var signedQuery = regexp.MustCompile(`(?i)([?&](?:X-Goog-Signature|X-Amz-Signature|X-Goog-Credential|X-Amz-Credential|signature|sig)=)[^&#\s]+`)
var userInfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)[^\s/@]+@`)

func Sanitize(value string) string {
	value = privateKey.ReplaceAllString(value, "[REDACTED PRIVATE KEY]")
	value = authorization.ReplaceAllString(value, "${1} [REDACTED]")
	value = assignment.ReplaceAllString(value, "${1}${2}[REDACTED]")
	value = signedQuery.ReplaceAllString(value, "${1}[REDACTED]")
	value = userInfo.ReplaceAllString(value, "${1}[REDACTED]@")
	if len(value) > 4096 {
		value = value[:4096] + "…"
	}
	return value
}
