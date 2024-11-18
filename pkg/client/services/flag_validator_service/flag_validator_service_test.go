package flag_validator_service

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateFile(t *testing.T) {
	validator := New()

	assert.False(t, validator.ValidateFile(""), "Expected false for empty file")
	assert.False(t, validator.ValidateFile("nonexistent.yaml"), "Expected false for nonexistent file")

	tempFile, err := os.CreateTemp("", "testfile.yaml")
	assert.NoError(t, err, "Failed to create temp file")
	defer os.Remove(tempFile.Name())

	assert.True(t, validator.ValidateFile(tempFile.Name()), "Expected true for existing file")
}

func TestValidateHost(t *testing.T) {
	validator := New()

	assert.False(t, validator.ValidateHost(""), "Expected false for empty host")
	assert.True(t, validator.ValidateHost("localhost"), "Expected true for valid host")
}

func TestValidatePort(t *testing.T) {
	validator := New()

	assert.False(t, validator.ValidatePort(""), "Expected false for empty port")
	assert.False(t, validator.ValidatePort("invalid"), "Expected false for invalid port")
	assert.False(t, validator.ValidatePort("70000"), "Expected false for out of range port")
	assert.True(t, validator.ValidatePort("8080"), "Expected true for valid port")
}
