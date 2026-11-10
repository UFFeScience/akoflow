package hubpublish

import (
	"regexp"
	"strconv"
	"strings"
)

var privateKeyPattern = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
var bearerPattern = regexp.MustCompile(`(?i)\b(Bearer|Basic)\s+[A-Za-z0-9._~+/=-]+`)
var assignmentPattern = regexp.MustCompile(`(?i)\b((?:[A-Z][A-Z0-9_]*_)?(?:password|passwd|token|api[_-]?key|secret|client[_-]?secret|access[_-]?key))(["']?\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`)
var signaturePattern = regexp.MustCompile(`(?i)([?&](?:X-Goog-Signature|X-Amz-Signature|X-Goog-Credential|X-Amz-Credential|signature|sig)=)[^&#\s]+`)
var embeddedCredentialPattern = regexp.MustCompile(`(?i)(https?://)[^\s/:@]+:[^\s/@]+@`)

func indexString(value int) string { return strconv.Itoa(value) }
func redactText(value string, secrets []string) string {
	for _, secret := range secrets {
		value = strings.ReplaceAll(value, secret, "[REDACTED]")
	}
	value = privateKeyPattern.ReplaceAllString(value, "[REDACTED PRIVATE KEY]")
	value = bearerPattern.ReplaceAllString(value, "${1} [REDACTED]")
	value = assignmentPattern.ReplaceAllString(value, "${1}${2}[REDACTED]")
	value = signaturePattern.ReplaceAllString(value, "${1}[REDACTED]")
	return embeddedCredentialPattern.ReplaceAllString(value, "${1}[REDACTED]@")
}
