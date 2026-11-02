package mcpbridge

import (
	"regexp"
	"strings"
	"sync"
)

type redactor struct {
	mu      sync.RWMutex
	secrets map[string]bool
}

var privatePEM = regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----`)
var bearerPattern = regexp.MustCompile(`(?i)Bearer[ \t]+[A-Za-z0-9._~+/=-]+`)
var assignmentPattern = regexp.MustCompile(`(?i)((?:password|private[_-]?key|client[_-]?secret|access[_-]?token|refresh[_-]?token|authorization|secret[_-]?access[_-]?key)["']?[ \t]*[:=][ \t]*["']?)[^"'\s,}]+`)

func newRedactor(secrets ...string) *redactor {
	r := &redactor{secrets: map[string]bool{}}
	for _, secret := range secrets {
		if secret != "" {
			r.secrets[secret] = true
		}
	}
	return r
}

func sensitive(key string) bool {
	key = strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	if strings.HasSuffix(key, "ref") || strings.HasSuffix(key, "reference") || strings.HasSuffix(key, "id") || strings.HasSuffix(key, "path") {
		return false
	}
	switch key {
	case "apikey",
		"apitoken",
		"bearertoken",
		"secretkey",
		"sessiontoken",
		"password",
		"passphrase",
		"privatekey",
		"privatekeypem",
		"token",
		"accesstoken",
		"refreshtoken",
		"idtoken",
		"clientsecret",
		"secret",
		"secretaccesskey",
		"credential",
		"credentials",
		"authorization",
		"serviceaccountjson":
		return true
	}
	return false
}

func (r *redactor) remember(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if sensitive(key) {
				r.rememberSecrets(item)
			} else {
				r.remember(item)
			}
		}
	case []any:
		for _, item := range v {
			r.remember(item)
		}
	}
}

func (r *redactor) rememberSecrets(value any) {
	switch v := value.(type) {
	case string:
		if len(v) >= 4 {
			r.mu.Lock()
			r.secrets[v] = true
			r.mu.Unlock()
		}
	case map[string]any:
		for _, item := range v {
			r.rememberSecrets(item)
		}
	case []any:
		for _, item := range v {
			r.rememberSecrets(item)
		}
	}
}

func (r *redactor) clean(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			if sensitive(key) {
				out[key] = "[REDACTED]"
			} else {
				out[key] = r.clean(item)
			}
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = r.clean(item)
		}
		return out
	case string:
		return r.text(v)
	default:
		return value
	}
}

func (r *redactor) text(value string) string {
	r.mu.RLock()
	for secret := range r.secrets {
		value = strings.ReplaceAll(value, secret, "[REDACTED]")
	}
	r.mu.RUnlock()
	value = privatePEM.ReplaceAllString(value, "[REDACTED PRIVATE KEY]")
	value = bearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	return assignmentPattern.ReplaceAllString(value, "${1}[REDACTED]")
}
