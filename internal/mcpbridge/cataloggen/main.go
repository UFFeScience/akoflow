// Command cataloggen derives the MCP catalog from registered daemon routes.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type definition struct {
	expr ast.Expr
	file *ast.File
	pkg  string
}
type operation struct {
	Name        string         `json:"name"`
	Method      string         `json:"method"`
	Path        string         `json:"path"`
	Handler     string         `json:"handler"`
	Description string         `json:"description"`
	Body        map[string]any `json:"body,omitempty"`
	Query       []string       `json:"query,omitempty"`
}

var definitions = map[string]definition{}
var handlers = map[string]definition{}
var funcs = map[string]*ast.FuncDecl{}
var fset = token.NewFileSet()
var routers []*ast.File

const module = "github.com/UFFeScience/akoflow/"

func main() {
	must(filepath.WalkDir("..", indexFile))
	var operations []operation
	used := map[string]bool{}
	for _, router := range routers {
		ast.Inspect(router, func(node ast.Node) bool {
			op, ok := routeOperation(node)
			if !ok {
				return true
			}
			if used[op.Name] {
				op.Name += "_compat"
			}
			if used[op.Name] {
				panic("duplicate tool: " + op.Name)
			}
			used[op.Name] = true
			op.Body, op.Query = operationContract(op.Handler)
			operations = append(operations, op)
			return true
		})
	}
	sort.Slice(operations, func(i, j int) bool { return operations[i].Name < operations[j].Name })
	data, err := json.MarshalIndent(operations, "", "  ")
	must(err)
	must(os.WriteFile("catalog.json", append(data, '\n'), 0644))
	fmt.Printf("Generated %d MCP operations\n", len(operations))
}

func indexFile(path string, entry fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
		return nil
	}
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return err
	}
	if filepath.Base(filepath.Dir(path)) == "httpserver" {
		routers = append(routers, file)
	}
	base, _ := filepath.Abs("../..")
	dir, _ := filepath.Abs(filepath.Dir(path))
	rel, _ := filepath.Rel(base, dir)
	pkg := module + filepath.ToSlash(rel)
	for _, decl := range file.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range gen.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok {
					definitions[pkg+"."+typ.Name.Name] = definition{typ.Type, file, pkg}
				}
			}
		}
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Body != nil && (strings.Contains(path, "workflow_engine_api_handler") || strings.HasSuffix(path, "httpserver.go")) {
			handlers[fn.Name.Name] = definition{nil, file, pkg}
			funcs[fn.Name.Name] = fn
		}
	}
	return nil
}

func routeOperation(node ast.Node) (operation, bool) {
	call, ok := node.(*ast.CallExpr)
	if !ok || len(call.Args) != 2 {
		return operation{}, false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "HandleFunc" {
		return operation{}, false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return operation{}, false
	}
	pattern, _ := strconv.Unquote(literal.Value)
	method, path, ok := strings.Cut(pattern, " ")
	if !ok {
		return operation{}, false
	}
	name := ""
	ast.Inspect(call.Args[1], func(n ast.Node) bool {
		if selector, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := selector.X.(*ast.Ident); ok && id.Name == "workflowEngine" {
				name = selector.Sel.Name
			}
		}
		if id, ok := n.(*ast.Ident); ok && (id.Name == "HealthCheck" || id.Name == "Preflight") {
			name = id.Name
		}
		return true
	})
	if name == "" {
		panic("unrecognized route: " + pattern)
	}
	// ServeMux end-of-path markers are routing syntax, not API arguments.
	path = strings.TrimSuffix(path, "{$}")
	tool := snake(name)
	description := strings.ReplaceAll(tool, "_", " ") + ". " + method + " " + path + ". Uses the daemon API and its domain validation."
	return operation{Name: tool, Method: method, Path: path, Handler: name, Description: description}, true
}

func operationContract(name string) (map[string]any, []string) {
	fn := funcs[name]
	if fn == nil {
		return nil, nil
	}
	variables := localVariables(fn.Body)
	var body map[string]any
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		decode := false
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			decode = fun.Name == "decode"
		case *ast.SelectorExpr:
			decode = fun.Sel.Name == "Decode"
		}
		if !decode {
			return true
		}
		for _, arg := range call.Args {
			ptr, ok := arg.(*ast.UnaryExpr)
			if !ok {
				continue
			}
			id, ok := ptr.X.(*ast.Ident)
			if !ok {
				continue
			}
			if typ := variables[id.Name]; typ != nil {
				body = schema(typ, handlers[name], map[string]bool{})
			}
		}
		return true
	})
	var buf bytes.Buffer
	must(format.Node(&buf, fset, fn.Body))
	pattern := regexp.MustCompile(`(?:Query\(\)|query|values)\.(?:Get|Has)\("([^"]+)"\)`)
	query := []string{}
	seen := map[string]bool{}
	for _, match := range pattern.FindAllStringSubmatch(buf.String(), -1) {
		if !seen[match[1]] {
			query = append(query, match[1])
			seen[match[1]] = true
		}
	}
	sort.Strings(query)
	return body, query
}

func localVariables(body *ast.BlockStmt) map[string]ast.Expr {
	variables := map[string]ast.Expr{}
	ast.Inspect(body, func(n ast.Node) bool {
		if value, ok := n.(*ast.ValueSpec); ok && value.Type != nil {
			for _, id := range value.Names {
				variables[id.Name] = value.Type
			}
		}
		if assign, ok := n.(*ast.AssignStmt); ok {
			for i, right := range assign.Rhs {
				if i >= len(assign.Lhs) {
					break
				}
				literal, ok := right.(*ast.CompositeLit)
				if !ok {
					continue
				}
				if id, ok := assign.Lhs[i].(*ast.Ident); ok {
					variables[id.Name] = literal.Type
				}
			}
		}
		return true
	})
	return variables
}

func schema(expr ast.Expr, context definition, seen map[string]bool) map[string]any {
	switch typ := expr.(type) {
	case *ast.StarExpr:
		return map[string]any{"anyOf": []any{schema(typ.X, context, seen), map[string]any{"type": "null"}}}
	case *ast.ArrayType:
		if id, ok := typ.Elt.(*ast.Ident); ok && (id.Name == "byte" || id.Name == "uint8") && typ.Len == nil {
			return map[string]any{"type": "string", "contentEncoding": "base64"}
		}
		return map[string]any{"type": "array", "items": schema(typ.Elt, context, seen)}
	case *ast.MapType:
		return map[string]any{"type": "object", "additionalProperties": schema(typ.Value, context, seen)}
	case *ast.StructType:
		return structSchema(typ, context, seen)
	case *ast.Ident:
		switch typ.Name {
		case "string":
			return map[string]any{"type": "string"}
		case "bool":
			return map[string]any{"type": "boolean"}
		case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
			return map[string]any{"type": "integer"}
		case "float32", "float64":
			return map[string]any{"type": "number"}
		case "any":
			return map[string]any{}
		}
		return named(context.pkg+"."+typ.Name, seen)
	case *ast.SelectorExpr:
		return selectorSchema(typ, context, seen)
	}
	return map[string]any{}
}

func structSchema(typ *ast.StructType, context definition, seen map[string]bool) map[string]any {
	properties := map[string]any{}
	for _, field := range typ.Fields.List {
		if len(field.Names) == 0 {
			if nested, ok := schema(field.Type, context, seen)["properties"].(map[string]any); ok {
				for key, value := range nested {
					properties[key] = value
				}
			}
			continue
		}
		name := field.Names[0].Name
		if name == "" || name[0] < 'A' || name[0] > 'Z' {
			continue
		}
		// JSON decoding accepts case-insensitive Go fields. Prefer conventional
		// lower-camel names when a request type does not declare JSON tags.
		name = strings.ToLower(name[:1]) + name[1:]
		if field.Tag != nil {
			tag, _ := strconv.Unquote(field.Tag.Value)
			if value, ok := reflect.StructTag(tag).Lookup("json"); ok {
				if tagName := strings.Split(value, ",")[0]; tagName != "" {
					name = tagName
				}
			}
		}
		if name == "-" {
			continue
		}
		value := schema(field.Type, context, seen)
		if field.Doc != nil {
			value["description"] = strings.TrimSpace(field.Doc.Text())
		}
		properties[name] = value
	}
	return map[string]any{"type": "object", "properties": properties}
}

func selectorSchema(typ *ast.SelectorExpr, context definition, seen map[string]bool) map[string]any {
	id, ok := typ.X.(*ast.Ident)
	if !ok {
		return map[string]any{}
	}
	for _, imp := range context.file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		alias := filepath.Base(path)
		if imp.Name != nil {
			alias = imp.Name.Name
		}
		if alias != id.Name {
			continue
		}
		if path == "encoding/json" && typ.Sel.Name == "RawMessage" {
			return map[string]any{}
		}
		if path == "time" && typ.Sel.Name == "Time" {
			return map[string]any{"type": "string", "format": "date-time"}
		}
		return named(path+"."+typ.Sel.Name, seen)
	}
	return map[string]any{}
}

func named(key string, seen map[string]bool) map[string]any {
	if seen[key] {
		return map[string]any{}
	}
	def, ok := definitions[key]
	if !ok {
		return map[string]any{}
	}
	copySeen := map[string]bool{}
	for key, value := range seen {
		copySeen[key] = value
	}
	copySeen[key] = true
	return schema(def.expr, def, copySeen)
}

func snake(value string) string {
	first := regexp.MustCompile(`([A-Z]+)([A-Z][a-z])`).ReplaceAllString(value, "${1}_${2}")
	return strings.ToLower(regexp.MustCompile(`([a-z0-9])([A-Z])`).ReplaceAllString(first, "${1}_${2}"))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
