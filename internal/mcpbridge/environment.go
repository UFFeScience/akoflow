package mcpbridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

// Validate the exact connection payload through the daemon before persistence.
// Simulated definitions without connections never trigger connectivity checks.
func (b *Bridge) validateEnvironment(ctx context.Context, op Operation, args arguments) error {
	applicable := op.Name == "create_environment" || op.Name == "replace_environment" ||
		op.Name == "update_environment_connection" || op.Name == "test_environment_connection"
	if !applicable {
		return nil
	}
	data, value, err := b.environmentPayload(args)
	if err != nil {
		return err
	}
	if containsSecret(value) {
		return errors.New("environment definitions must use credential references; send secret values to dedicated credential tools")
	}
	if op.Name == "test_environment_connection" {
		return nil
	}
	if op.Name == "update_environment_connection" {
		return b.validateConnection(ctx, data)
	}
	var definition struct {
		Connections []json.RawMessage `json:"connections"`
	}
	if len(data) > 0 && json.Unmarshal(data, &definition) != nil {
		return errors.New("invalid environment connections")
	}
	for _, connection := range definition.Connections {
		if err := b.validateConnection(ctx, connection); err != nil {
			return err
		}
	}
	return nil
}
func (b *Bridge) environmentPayload(args arguments) ([]byte, map[string]any, error) {
	data := []byte(args.Body)
	if args.UploadID != "" {
		file, err := b.transfers.open(args.UploadID)
		if err != nil {
			return nil, nil, err
		}
		data, err = io.ReadAll(io.LimitReader(file, (32<<20)+1))
		file.Close()
		if err != nil || len(data) > 32<<20 {
			return nil, nil, errors.New("environment input exceeds the API's 32 MiB limit")
		}
	}
	if len(data) == 0 {
		return nil, nil, nil
	}
	var value map[string]any
	if json.Unmarshal(data, &value) != nil {
		if yaml.Unmarshal(data, &value) != nil {
			return nil, nil, errors.New("invalid environment definition")
		}
		var err error
		data, err = json.Marshal(value)
		if err != nil {
			return nil, nil, errors.New("invalid environment YAML")
		}
	}
	return data, value, nil
}
func containsSecret(value any) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if sensitive(key) && item != nil && item != "" {
				return true
			}
			if containsSecret(item) {
				return true
			}
		}
	case []any:
		for _, item := range value {
			if containsSecret(item) {
				return true
			}
		}
	}
	return false
}
func (b *Bridge) validateConnection(ctx context.Context, data []byte) error {
	target := *b.base
	target.Path = "/akoflow-api/connection-tests/"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.String(), strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+b.options.APIToken)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Akoflow-Client", "mcp/"+identityFromContext(ctx).client)
	response, err := b.client.Do(request)
	if err != nil {
		return errors.New("daemon connection validation failed; registration was not sent")
	}
	defer response.Body.Close()
	var result struct {
		Healthy bool   `json:"healthy"`
		Message string `json:"message"`
	}
	err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result)
	if err != nil || response.StatusCode != http.StatusOK || !result.Healthy {
		return errors.New("connection validation failed; registration was not sent: " + b.redactor.text(result.Message))
	}
	return nil
}
