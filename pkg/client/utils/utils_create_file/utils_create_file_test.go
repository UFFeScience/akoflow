package utils_create_file

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateFile(t *testing.T) {
	utils := New()
	filePath := "testfile.txt"
	content := "Hello, World!"

	utils.CreateFile(filePath, content)

	_, err := os.Stat(filePath)
	assert.NoError(t, err, "expected file %s to exist, but it does not", filePath)

	data, err := os.ReadFile(filePath)
	assert.NoError(t, err, "failed to read file %s", filePath)

	assert.Equal(t, content, string(data), "expected file content to be %s, but got %s", content, string(data))

	os.Remove(filePath)
}

func TestCreateTempFile(t *testing.T) {
	utils := New()
	content := "Hello, World!"

	filePath := utils.CreateTempFile(content)

	_, err := os.Stat(filePath)
	assert.NoError(t, err, "expected file %s to exist, but it does not", filePath)

	data, err := os.ReadFile(filePath)
	assert.NoError(t, err, "failed to read file %s", filePath)

	assert.Equal(t, content, string(data), "expected file content to be %s, but got %s", content, string(data))

	os.Remove(filePath)
}
