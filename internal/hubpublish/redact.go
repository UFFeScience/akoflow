package hubpublish

import (
	"sort"
	"strings"
)

var forbiddenKeys = map[string]bool{"password": true,
	"passwd":             true,
	"token":              true,
	"accesstoken":        true,
	"refreshtoken":       true,
	"authorization":      true,
	"privatekey":         true,
	"sshprivatekey":      true,
	"secret":             true,
	"clientsecret":       true,
	"secretaccesskey":    true,
	"awssecretaccesskey": true,
	"credentials":        true,
	"connectionstring":   true,
	"dsn":                true}

type snapshotRedactor struct {
	secrets []string
	paths   []string
}

func normalizedKey(key string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, strings.ToLower(key))
}
func redactSnapshot(value any) (any, []string, error) {
	redactor := &snapshotRedactor{paths: []string{}}
	if err := redactor.collect(value, 0); err != nil {
		return nil, nil, err
	}
	sort.Slice(redactor.secrets, func(i, j int) bool { return len(redactor.secrets[i]) > len(redactor.secrets[j]) })
	result, err := redactor.visit(value,
		"", 0)
	return result, redactor.paths, err
}
func (r *snapshotRedactor) collect(value any, depth int) error {
	if depth > 64 {
		return failure(422, "snapshot_depth", "This snapshot exceeds the supported nesting limit.")
	}
	switch item := value.(type) {
	case map[string]any:
		for key, child := range item {
			if text, ok := child.(string); ok && forbiddenKeys[normalizedKey(key)] && len(text) >= 4 && !strings.HasPrefix(text, "credential://") {
				r.secrets = append(r.secrets, text)
			}
			if err := r.collect(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range item {
			if err := r.collect(child, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}
func (r *snapshotRedactor) visit(value any, path string, depth int) (any, error) {
	if depth > 64 {
		return nil, failure(422, "snapshot_depth", "This snapshot exceeds the supported nesting limit.")
	}
	switch item := value.(type) {
	case map[string]any:
		return r.visitObject(item, path, depth)
	case []any:
		result := make([]any, 0, len(item))
		for index, child := range item {
			cleaned, err := r.visit(child, path+"/"+indexString(index), depth+1)
			if err != nil {
				return nil, err
			}
			result = append(result, cleaned)
		}
		return result, nil
	case string:
		cleaned := redactText(item, r.secrets)
		if cleaned != item {
			r.paths = append(r.paths, path)
		}
		return cleaned, nil
	default:
		return value, nil
	}
}
func (r *snapshotRedactor) visitObject(item map[string]any, path string, depth int) (map[string]any, error) {
	result := map[string]any{}
	keys := make([]string, 0, len(item))
	for key := range item {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		child := item[key]
		reference, ok := child.(string)
		if forbiddenKeys[normalizedKey(key)] && child != nil && (!ok || reference != "" && !strings.HasPrefix(reference, "credential://")) {
			r.paths = append(r.paths, path+"/"+key)
			continue
		}
		cleaned, err := r.visit(child, path+"/"+key, depth+1)
		if err != nil {
			return nil, err
		}
		result[key] = cleaned
	}
	return result, nil
}

func SanitizeArtifactInspection(value map[string]any) (map[string]any, error) {
	cleaned, _, err := redactSnapshot(value)
	if err != nil {
		return nil, err
	}
	return cleaned.(map[string]any), nil
}
