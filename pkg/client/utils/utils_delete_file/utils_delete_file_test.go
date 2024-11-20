package utils_delete_file

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteFile(t *testing.T) {
	utils := New()

	// Create a temporary file
	file, err := os.CreateTemp("", "testfile")
	require.NoError(t, err, "Failed to create temp file")
	filePath := file.Name()
	file.Close()

	// Ensure the file exists
	_, err = os.Stat(filePath)
	require.NoError(t, err, "File does not exist")

	// Delete the file
	err = utils.DeleteFile(filePath)
	assert.NoError(t, err, "Failed to delete file")

	// Ensure the file no longer exists
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "File still exists")
}

func TestDeleteNonExistentFile(t *testing.T) {
	utils := New()

	// Attempt to delete a non-existent file
	err := utils.DeleteFile("/non/existent/file/path")
	assert.Error(t, err, "Expected an error when deleting a non-existent file")
}
