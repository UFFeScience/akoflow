package mcpbridge

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:generate go run ./cataloggen
//go:embed catalog.json
var catalogJSON []byte

type Operation struct {
	Name        string         `json:"name"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Handler     string         `json:"handler"`
	Description string         `json:"description"`
	Body        map[string]any `json:"body,omitempty"`
	Query       []string       `json:"query,omitempty"`
}

type Permission string

const (
	Read  Permission = "read"
	Write Permission = "write"
	Admin Permission = "admin"
)

func Catalog() []Operation {
	var operations []Operation
	if err := json.Unmarshal(catalogJSON, &operations); err != nil {
		panic(err)
	}
	return operations
}

func (op Operation) Permission() Permission {
	if op.Name == "stream_console_session" {
		return Admin
	}
	if op.Method == "GET" || op.Name == "query_provenance_sql" || op.Name == "explain_provenance_sql" {
		return Read
	}
	if op.Method == "DELETE" {
		return Admin
	}
	for _, term := range []string{
		"credential",
		"ssh_key",
		"kubernetes_token",
		"console",
		"factory_reset",
		"archive_instance",
		"destroy_cloud",
		"stop_cloud",
		"activate_archive",
		"replace_environment",
		"save_instance",
		"reconcile_orphan",
	} {
		if strings.Contains(op.Name, term) {
			return Admin
		}
	}
	return Write
}

func (p Permission) Allows(required Permission) bool {
	ranks := map[Permission]int{Read: 1, Write: 2, Admin: 3}
	return ranks[p] > 0 && ranks[p] >= ranks[required]
}

func (op Operation) Schema() map[string]any {
	properties := map[string]any{}
	required := []string{}
	pathProperties := map[string]any{}
	for _, segment := range strings.Split(op.Path, "/") {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			key := strings.Trim(segment, "{}")
			pathProperties[key] = map[string]any{"type": "string", "minLength": 1, "description": "Opaque daemon identifier. Slashes and dot segments are not allowed."}
		}
	}
	if len(pathProperties) > 0 {
		names := []string{}
		for key := range pathProperties {
			names = append(names, key)
		}
		sort.Strings(names)
		properties["path"] = map[string]any{"type": "object", "properties": pathProperties, "required": names, "additionalProperties": false}
		required = append(required, "path")
	}
	queryProps := map[string]any{}
	for _, name := range op.Query {
		queryProps[name] = map[string]any{"type": "string"}
	}
	properties["query"] = map[string]any{
		"type":       "object",
		"properties": queryProps,
		"additionalProperties": map[string]any{
			"type": "string",
		},
		"description": "API query parameters as strings; pagination and filters retain API semantics.",
	}
	if op.Method != "GET" && op.Method != "DELETE" {
		body := op.Body
		if body == nil {
			body = map[string]any{"type": "object"}
		}
		properties["body"] = body
		properties["uploadId"] = map[string]any{"type": "string", "description": "Sealed MCP upload reference, used instead of body for ZIP, tar, JSON or YAML input."}
		properties["mediaType"] = map[string]any{"type": "string", "enum": []string{"application/json", "application/yaml", "application/zip", "application/octet-stream", "application/gzip"}}
	}
	if op.Name == "stream_console_session" {
		properties["input"] = map[string]any{"type": "string", "maxLength": 65536, "description": "Terminal input; empty input only reads output."}
		properties["waitMillis"] = map[string]any{"type": "integer", "minimum": 1, "maximum": 5000}
	}
	if op.Permission() != Read {
		properties["requestId"] = map[string]any{
			"type":        "string",
			"minLength":   1,
			"maxLength":   128,
			"description": "Unique client request ID. Reuse with identical arguments for retry; durable receipts prevent duplicate dispatch.",
		}
		required = append(required, "requestId")
	}
	if op.Permission() == Admin {
		properties["confirmation"] = map[string]any{"type": "string", "description": "Digest returned by prepare_operation for these exact arguments. Obtain explicit user approval before submitting."}
		required = append(required, "confirmation")
	}
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

func validatePermission(p Permission) error {
	if p != Read && p != Write && p != Admin {
		return fmt.Errorf("permissions must be read, write or admin")
	}
	return nil
}
