package algorithms

import (
	"embed"
	"strings"
)

//go:embed *.go
var nativeSource embed.FS

// Sources exports the implementation shipped with this engine, including shared helpers.
func Sources() map[string]string {
	entries, _ := nativeSource.ReadDir(".")
	files := map[string]string{}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), "_test.go") || entry.Name() == "source.go" {
			continue
		}
		data, _ := nativeSource.ReadFile(entry.Name())
		files[entry.Name()] = string(data)
	}
	return files
}
