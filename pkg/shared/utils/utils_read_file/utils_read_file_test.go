package utils_read_file

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadFile(t *testing.T) {
	utils := New()

	// Create a temporary file
	file, err := os.CreateTemp("", "testfile")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	// Write some content to the file
	content := "Hello, World!"
	_, err = file.Write([]byte(content))
	require.NoError(t, err)
	file.Close()

	// Test reading the file
	result := utils.ReadFile(file.Name())
	assert.Equal(t, content, result)
}

func TestReadFile_FileNotFound(t *testing.T) {
	utils := New()

	assert.Panics(t, func() {
		utils.ReadFile("non_existent_file.txt")
	}, "expected panic for non-existent file")
}
