package domain

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The domain is the stable center of AkoFlow. Bounded contexts may reference
// one another, but never storage, providers, plugins or frameworks.
func TestDomainHasNoProjectOrThirdPartyDependencies(t *testing.T) {
	files, err := filepath.Glob("**/*.go")
	require.NoError(t, err)
	rootFiles, err := filepath.Glob("*.go")
	require.NoError(t, err)
	files = append(files, rootFiles...)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imported := range parsed.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			require.NoError(t, err)
			isStandardLibrary := !strings.Contains(path, ".")
			isDomainContext := strings.HasPrefix(path, "github.com/UFFeScience/akoflow/internal/domain")
			require.Truef(t, isStandardLibrary || isDomainContext, "%s imports non-domain package %s", file, path)
		}
	}
}

var _ ast.Node // keep the architecture test explicit about using Go's AST.
