package machineconfiguration

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/UFFeScience/akoflow/internal/domain"
	"gopkg.in/yaml.v3"
)

func ValidatePlaybook(source string) domain.MachineConfigurationValidation {
	result := domain.MachineConfigurationValidation{}
	source = strings.TrimSpace(source)
	if source == "" {
		result.Errors = append(result.Errors, "playbook YAML is required")
		return result
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(source), &node); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("invalid YAML: %v", err))
		return result
	}
	var plays []map[string]any
	if err := yaml.Unmarshal([]byte(source), &plays); err != nil || len(plays) == 0 {
		result.Errors = append(result.Errors, "playbook must be a non-empty YAML sequence of plays")
		return result
	}
	for index, play := range plays {
		if strings.TrimSpace(fmt.Sprint(play["hosts"])) == "" || play["hosts"] == nil {
			result.Errors = append(result.Errors, fmt.Sprintf("play %d must declare hosts", index+1))
		}
		tasks, ok := play["tasks"].([]any)
		if !ok || len(tasks) == 0 {
			result.Errors = append(result.Errors, fmt.Sprintf("play %d must declare at least one task", index+1))
		}
	}
	sum := sha256.Sum256([]byte(source))
	result.SHA256 = "sha256:" + hex.EncodeToString(sum[:])
	result.Valid = len(result.Errors) == 0
	return result
}
