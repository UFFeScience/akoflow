package logger

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLogger(t *testing.T) {
	logger, err := NewLogger("test.log")
	require.NoError(t, err)
	defer os.Remove("test.log")
	defer logger.Close()

	assert.NotNil(t, logger.file, "expected file to be initialized")
	assert.NotNil(t, logger.logger, "expected logger to be initialized")
}

func TestLogger_Info(t *testing.T) {
	logger, err := NewLogger("test.log")
	require.NoError(t, err)
	defer os.Remove("test.log")
	defer logger.Close()

	logger.Info("This is an info message")

	content, err := os.ReadFile("test.log")
	require.NoError(t, err)
	assert.Contains(t, string(content), "This is an info message")
}

func TestLogger_Infof(t *testing.T) {
	logger, err := NewLogger("test.log")
	require.NoError(t, err)
	defer os.Remove("test.log")
	defer logger.Close()

	logger.Infof("This is an info message with %s", "formatting")

	content, err := os.ReadFile("test.log")
	require.NoError(t, err)
	assert.Contains(t, string(content), "This is an info message with formatting")
}

func TestLogger_Warning(t *testing.T) {
	logger, err := NewLogger("test.log")
	require.NoError(t, err)
	defer os.Remove("test.log")
	defer logger.Close()

	logger.Warning("This is a warning message")

	content, err := os.ReadFile("test.log")
	require.NoError(t, err)
	assert.Contains(t, string(content), "This is a warning message")
}

func TestLogger_Error(t *testing.T) {
	logger, err := NewLogger("test.log")
	require.NoError(t, err)
	defer os.Remove("test.log")
	defer logger.Close()

	logger.Error("This is an error message")

	content, err := os.ReadFile("test.log")
	require.NoError(t, err)
	assert.Contains(t, string(content), "This is an error message")
}
