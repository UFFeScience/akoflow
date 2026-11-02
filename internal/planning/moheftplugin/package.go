package moheftplugin

import (
	"embed"
	"go/format"
	"strings"

	"github.com/UFFeScience/akoflow/internal/planning/library"
)

//go:embed algorithm.go README.md
var files embed.FS

// Package returns editable Go source, without registering or compiling it.
func Package() (library.Package, error) {
	source, err := files.ReadFile("algorithm.go")
	if err != nil {
		return library.Package{}, err
	}
	source = []byte(strings.TrimPrefix(string(source), "//go:build ignore\n\n"))
	source, err = format.Source(source)
	if err != nil {
		return library.Package{}, err
	}
	docs, err := files.ReadFile("README.md")
	if err != nil {
		return library.Package{}, err
	}
	return library.Package{SchemaVersion: "akoflow.scheduler/v1", ID: "moheft", Name: "MOHEFT", Description: "Pareto list scheduling for completion time and financial cost (Durillo et al., 2012).", Objective: "pareto", Multiple: true, Origin: "plugin", Defaults: map[string]any{"candidateCount": 25}, Source: string(source), Documentation: string(docs)}, nil
}
