package flag_validator_service_test

import (
	"os"
	"testing"

	"github.com/ovvesley/akoflow/pkg/client/services/flag_validator_service"
)

func TestValidateFile(t *testing.T) {
	validator := flag_validator_service.New()

	if validator.ValidateFile("") {
		t.Error("Expected false for empty file")
	}

	if validator.ValidateFile("nonexistent.yaml") {
		t.Error("Expected false for nonexistent file")
	}

	tempFile, err := os.CreateTemp("", "testfile.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	if !validator.ValidateFile(tempFile.Name()) {
		t.Error("Expected true for existing file")
	}
}

func TestValidateHost(t *testing.T) {
	validator := flag_validator_service.New()

	if validator.ValidateHost("") {
		t.Error("Expected false for empty host")
	}

	if !validator.ValidateHost("localhost") {
		t.Error("Expected true for valid host")
	}
}

func TestValidatePort(t *testing.T) {
	validator := flag_validator_service.New()

	if validator.ValidatePort("") {
		t.Error("Expected false for empty port")
	}

	if validator.ValidatePort("invalid") {
		t.Error("Expected false for invalid port")
	}

	if validator.ValidatePort("70000") {
		t.Error("Expected false for out of range port")
	}

	if !validator.ValidatePort("8080") {
		t.Error("Expected true for valid port")
	}
}
