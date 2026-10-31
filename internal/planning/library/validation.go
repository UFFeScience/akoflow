package library

import (
	"context"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
)

type Diagnostic struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}
type Validation struct {
	Valid           bool         `json:"valid"`
	Compiled        bool         `json:"compiled"`
	FormattedSource string       `json:"formattedSource,omitempty"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
}

func sourceUnit(source string) (string, int) {
	if _, err := parser.ParseFile(token.NewFileSet(), "algorithm.go", source, parser.PackageClauseOnly); err != nil {
		return "package main\n" + source, 1
	}
	return source, 0
}
func Validate(ctx context.Context, source string, compile bool) (Validation, error) {
	result := Validation{Diagnostics: []Diagnostic{}}
	if len(source) == 0 || len(source) > 512*1024 {
		return result, fmt.Errorf("Go source must contain between 1 byte and 512 KiB")
	}
	unit, offset := sourceUnit(source)
	fileset := token.NewFileSet()
	file, err := parser.ParseFile(fileset, "algorithm.go", unit, parser.AllErrors)
	if err != nil {
		if errors, ok := err.(scanner.ErrorList); ok {
			seen := map[string]bool{}
			for _, e := range errors {
				key := fmt.Sprintf("%d:%d:%s", e.Pos.Line, e.Pos.Column, e.Msg)
				if seen[key] {
					continue
				}
				seen[key] = true
				result.Diagnostics = append(result.Diagnostics, Diagnostic{Line: max(1, e.Pos.Line-offset), Column: e.Pos.Column, Message: e.Msg, Severity: "error"})
			}
		} else {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Line: 1, Column: 1, Message: err.Error(), Severity: "error"})
		}
		return result, nil
	}
	add := func(pos token.Pos, message string) {
		p := fileset.Position(pos)
		result.Diagnostics = append(result.Diagnostics, Diagnostic{Line: max(1, p.Line-offset), Column: p.Column, Message: message, Severity: "error"})
	}
	if file.Name.Name != "main" {
		add(file.Name.Pos(), "Use package main for a scheduler plugin.")
	}
	var schedule *ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fn.Name.Name == "main" {
			add(fn.Pos(), "The daemon provides main; implement only Schedule and helpers.")
		}
		if fn.Name.Name == "Schedule" {
			if schedule != nil {
				add(fn.Pos(), "Schedule is declared more than once.")
			}
			schedule = fn
		}
	}
	if schedule == nil {
		add(file.Pos(), "Implement func Schedule(input Input) ([]Plan, error).")
	} else if schedule.Recv != nil || !signature(schedule.Type) {
		add(schedule.Pos(), "Schedule must accept one Input and return ([]Plan, error).")
	}
	if len(result.Diagnostics) > 0 {
		return result, nil
	}
	formatted, err := format.Source([]byte(unit))
	if err != nil {
		return result, err
	}
	result.FormattedSource = string(formatted)
	if offset > 0 {
		result.FormattedSource = strings.TrimPrefix(result.FormattedSource, "package main\n\n")
	}
	result.Valid = true
	if !compile {
		return result, nil
	}
	root, err := os.MkdirTemp("", "akoflow-syntax-check-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(root)
	service := &Service{root: root}
	_, err = service.compile(ctx, Package{ID: "validation", SourceHash: "current", Source: source})
	if err != nil {
		result.Valid = false
		pattern := regexp.MustCompile(`algorithm\.go:(\d+):(\d+): ([^\n]+)`)
		for _, match := range pattern.FindAllStringSubmatch(err.Error(), -1) {
			line, _ := strconv.Atoi(match[1])
			column, _ := strconv.Atoi(match[2])
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Line: max(1, line-offset), Column: column, Message: match[3], Severity: "error"})
		}
		if len(result.Diagnostics) == 0 {
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Line: 1, Column: 1, Message: err.Error(), Severity: "error"})
		}
		return result, nil
	}
	result.Compiled = true
	return result, nil
}
func signature(fn *ast.FuncType) bool {
	if fn.TypeParams != nil || fn.Params == nil || len(fn.Params.List) != 1 || fn.Results == nil || len(fn.Results.List) != 2 {
		return false
	}
	p := fn.Params.List[0]
	if len(p.Names) > 1 {
		return false
	}
	input, ok := p.Type.(*ast.Ident)
	if !ok || input.Name != "Input" {
		return false
	}
	first := fn.Results.List[0]
	if len(first.Names) > 1 {
		return false
	}
	plans, ok := first.Type.(*ast.ArrayType)
	if !ok || plans.Len != nil {
		return false
	}
	plan, ok := plans.Elt.(*ast.Ident)
	if !ok || plan.Name != "Plan" {
		return false
	}
	last := fn.Results.List[1]
	if len(last.Names) > 1 {
		return false
	}
	output, ok := last.Type.(*ast.Ident)
	return ok && output.Name == "error"
}
