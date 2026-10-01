package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/UFFeScience/akoflow/internal/validation"
)

// examples-list walks the examples/ directory of either the in-tree
// akoflow fixtures or a standalone akoflow-examples checkout and prints the
// discovered fixture categories together with the files they ship. The CLI is
// the read-only inspection counterpart of the validation test suite: it lets
// docs authors and CI scripts verify which fixtures the harness sees without
// having to write Go code.
func main() {
	var root string
	flag.StringVar(&root, "root", "", "examples/ directory to walk (defaults to the in-tree examples/ relative to the repository root)")
	flag.Parse()

	if root == "" {
		resolved, err := validation.RepositoryRoot()
		if err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
		root = resolved
	}

	fixtures, err := validation.DiscoverFixtures(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	names := fixtures.SortedNames()
	for _, n := range names {
		c := fixtures[n]
		files := make([]string, 0, len(c.Files))
		for f := range c.Files {
			files = append(files, f)
		}
		sort.Strings(files)
		fmt.Printf("[%s] dir=%s files=%v\n", n, c.Directory, files)
	}
	fmt.Printf("total: %d categories\n", len(fixtures))
}
