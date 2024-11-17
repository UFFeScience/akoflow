package utils_read_file_test

import (
	"os"
	"testing"

	"github.com/ovvesley/akoflow/pkg/client/utils/utils_read_file"
)

func TestReadFile(t *testing.T) {
	utils := utils_read_file.New()

	// Create a temporary file
	file, err := os.CreateTemp("", "testfile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())

	// Write some content to the file
	content := "Hello, World!"
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	file.Close()

	// Test reading the file
	result := utils.ReadFile(file.Name())
	if result != content {
		t.Errorf("expected %s, got %s", content, result)
	}
}

func TestReadFile_FileNotFound(t *testing.T) {
	utils := utils_read_file.New()

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for non-existent file")
		}
	}()

	utils.ReadFile("non_existent_file.txt")
}
