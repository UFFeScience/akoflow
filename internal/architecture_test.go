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

// These baselines make the architecture checks incremental. They document
// debt that predates the v1.0 release while ensuring new code cannot make an
// existing exception worse or introduce another exception silently.
var legacyTypeNameExceptions = map[string]map[string]bool{
	"infrastructure/instancearchive/service.go": {"Service": true},
}

var legacyLineLengthLimits = map[string]int{
	"api/handlers/workflow_engine_api_handler/catalog_operations_test.go":     786,
	"api/handlers/workflow_engine_api_handler/operations_test.go":             599,
	"api/handlers/workflow_engine_api_handler/workflow_engine_api_handler.go": 365,
	"application/cloudprovision/service.go":                                   269,
	"application/console/service_test.go":                                     256,
	"application/environment/discovery_service_test.go":                       497,
	"application/execution/controller_test.go":                                267,
	"application/transfer/service.go":                                         344,
	"application/transfer/service_test.go":                                    336,
	"controlplane/eventloop/cloud_operation_handler.go":                       330,
	"controlplane/execution/cloud_allocator.go":                               389,
	"infrastructure/database/cloud/repository.go":                             268,
	"infrastructure/database/console/repository_test.go":                      456,
	"infrastructure/database/data/repository.go":                              258,
	"infrastructure/database/data/repository_test.go":                         302,
	"infrastructure/database/environment/repository.go":                       378,
	"infrastructure/database/environment/repository_test.go":                  263,
	"infrastructure/database/network/repository_test.go":                      279,
	"infrastructure/database/planning/repository_test.go":                     414,
	"infrastructure/database/storage/repository_test.go":                      346,
	"infrastructure/database/workflow/repository_test.go":                     298,
	"planning/algorithms/cloud_lifecycle.go":                                  332,
	"planning/algorithms/cloud_lifecycle_test.go":                             259,
	"provider/kubernetes/discovery_test.go":                                   257,
	"provider/kubernetes/terminal.go":                                         485,
	"provider/local/adapter_test.go":                                          428,
	"provider/slurm/adapter.go":                                               340,
	"provider/slurm/discovery.go":                                             558,
	"provider/storage/s3/driver_test.go":                                      278,
}

var legacyFunctionLineLimits = map[string]int{
	"api/handlers/workflow_engine_api_handler/workflow_engine_api_handler.go:Search":                   108,
	"api/handlers/workflow_engine_api_handler/workflow_engine_api_handler.go:resolveBuildPreparations": 90,
	"api/httpserver/httpserver.go:NewMux":                                                              130,
	"application/cloudprovision/service.go:Provision":                                                  87,
	"application/transfer/coordinator.go:Prepare":                                                      90,
	"application/transfer/service.go:Materialize":                                                      146,
	"controlplane/eventloop/cloud_operation_handler.go:Handle":                                         89,
	"controlplane/execution/supervisor.go:startReadyActivities":                                        92,
	"infrastructure/database/environment/repository_test.go:TestEnvironmentDefinitionCreate":           95,
	"infrastructure/database/workflow/repository.go:FindVersion":                                       81,
	"infrastructure/instancearchive/service.go:Import":                                                 82,
	"infrastructure/transfer/endpoint_resolver.go:ResolveTransferEndpoint":                             93,
	"planning/algorithms/prism_evaluator.go:evaluateCompleteCompactPRISMState":                         195,
	"provider/cloud/ansible/runner.go:Configure":                                                       81,
}

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
					if legacyTypeNameExceptions[path][name] {
						continue
					}
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
			limit := maximumGoLineLength
			if legacyLimit := legacyLineLengthLimits[path]; legacyLimit > limit {
				limit = legacyLimit
			}
			if len(scanner.Bytes()) > limit {
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
			limit := maximumFunctionLines
			key := path + ":" + function.Name.Name
			if legacyLimit := legacyFunctionLineLimits[key]; legacyLimit > limit {
				limit = legacyLimit
			}
			if lines := end - start + 1; lines > limit {
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
