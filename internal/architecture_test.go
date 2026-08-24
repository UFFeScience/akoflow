package internal_test

import (
	"bufio"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const maximumGoLineLength = 240
const maximumFunctionLines = 80

func TestRequiredArchitectureDirectoriesExist(t *testing.T) {
	required := []string{
		"domain/workflow", "domain/environment", "domain/resource",
		"domain/planning", "domain/execution", "application/execution",
		"application/ports", "infrastructure/database",
		"infrastructure/plugins", "provider/local", "provider/kubernetes",
		"provider/slurm", "provider/registry", "provider/simgrid",
		"controlplane/eventloop", "controlplane/execution",
		"api/handlers",
	}
	for _, path := range required {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			t.Errorf("required architecture directory %q is missing", path)
		}
	}
}

func TestPackagesUseCapabilityNamesInsteadOfServiceOrRepositorySuffixes(t *testing.T) {
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, declaration := range file.Decls {
			generic, ok := declaration.(*ast.GenDecl)
			if !ok || generic.Tok != token.TYPE {
				continue
			}
			for _, specification := range generic.Specs {
				name := specification.(*ast.TypeSpec).Name.Name
				if strings.HasSuffix(name, "Service") ||
					(strings.HasSuffix(name, "Repository") && name != "Repository") {
					t.Errorf("%s declares architecture-specific implementation name %q", path, name)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGoSourceDoesNotContainUnreadableInlineStructures(t *testing.T) {
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, openErr := os.Open(path)
		if openErr != nil {
			return openErr
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			if len(scanner.Bytes()) > maximumGoLineLength {
				t.Errorf(
					"%s:%d has %d characters; split the inline structure into readable fields",
					path, lineNumber, len(scanner.Bytes()),
				)
			}
		}
		return scanner.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFunctionsRemainFocused(t *testing.T) {
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		files := token.NewFileSet()
		file, parseErr := parser.ParseFile(files, path, nil, 0)
		if parseErr != nil {
			return parseErr
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				continue
			}
			start := files.Position(function.Pos()).Line
			end := files.Position(function.End()).Line
			if lines := end - start + 1; lines > maximumFunctionLines {
				t.Errorf(
					"%s:%d function %s has %d lines; extract semantic responsibilities",
					path, start, function.Name.Name, lines,
				)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInternalCodeDoesNotImportRemovedServerTree(t *testing.T) {
	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		for _, imported := range file.Imports {
			value, unquoteErr := strconv.Unquote(imported.Path.Value)
			if unquoteErr != nil {
				return unquoteErr
			}
			if strings.Contains(value, "/pkg/server/") {
				t.Errorf("%s still imports removed server tree: %s", path, value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
