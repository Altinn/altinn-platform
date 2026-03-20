package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	var (
		entryFilesArg = flag.String("entry-files", "manifests/main.go", "comma-separated Go files that reference the generated k8s imports")
		importsDir    = flag.String("imports-dir", "imports/k8s", "directory containing generated cdk8s Go imports")
		dryRun        = flag.Bool("dry-run", false, "print the pruning result without modifying files")
	)
	flag.Parse()

	entryFiles := splitCSV(*entryFilesArg)
	if len(entryFiles) == 0 {
		fatalf("no entry files provided")
	}

	typeToFile, err := discoverGeneratedTypes(*importsDir)
	if err != nil {
		fatalf("discover generated types: %v", err)
	}

	roots, err := findRootTypes(entryFiles, *importsDir, typeToFile)
	if err != nil {
		fatalf("find root types: %v", err)
	}
	if len(roots) == 0 {
		fatalf("no k8s import usages found in %v", entryFiles)
	}

	keepTypes, err := computeClosure(*importsDir, typeToFile, roots)
	if err != nil {
		fatalf("compute closure: %v", err)
	}

	keepFiles := buildKeepFiles(*importsDir, typeToFile, keepTypes)
	removeFiles := findRemoveFiles(*importsDir, keepFiles)

	fmt.Printf("cdk8s prune: keeping %d types, %d files; removing %d files\n", len(keepTypes), len(keepFiles), len(removeFiles))
	if *dryRun {
		for _, file := range removeFiles {
			fmt.Println(file)
		}
		return
	}

	if err := rewriteMainFile(filepath.Join(*importsDir, "main.go"), keepTypes); err != nil {
		fatalf("rewrite main.go: %v", err)
	}

	for _, file := range removeFiles {
		if err := os.Remove(file); err != nil {
			fatalf("remove %s: %v", file, err)
		}
	}
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	var out []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func discoverGeneratedTypes(importsDir string) (map[string]string, error) {
	entries, err := os.ReadDir(importsDir)
	if err != nil {
		return nil, err
	}

	typeToFile := make(map[string]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if name == "main.go" || strings.HasSuffix(name, "__checks.go") || strings.HasSuffix(name, "__no_checks.go") {
			continue
		}
		typeName := strings.TrimSuffix(name, ".go")
		typeToFile[typeName] = filepath.Join(importsDir, name)
	}

	return typeToFile, nil
}

func findRootTypes(entryFiles []string, importsDir string, typeToFile map[string]string) (map[string]struct{}, error) {
	roots := make(map[string]struct{})
	normalizedImportsDir := filepath.ToSlash(importsDir)

	for _, path := range entryFiles {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}

		aliases := make(map[string]struct{})
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("unquote import path in %s: %w", path, err)
			}
			if !strings.HasSuffix(filepath.ToSlash(importPath), normalizedImportsDir) {
				continue
			}

			alias := filepath.Base(importPath)
			if spec.Name != nil {
				alias = spec.Name.Name
			}
			aliases[alias] = struct{}{}
		}

		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			ident, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}

			if _, ok := aliases[ident.Name]; ok {
				if typeName, ok := selectorToType(selector.Sel.Name, typeToFile); ok {
					roots[typeName] = struct{}{}
				}
			}
			return true
		})
	}

	return roots, nil
}

func selectorToType(selector string, typeToFile map[string]string) (string, bool) {
	if _, ok := typeToFile[selector]; ok {
		return selector, true
	}

	if typeName, ok := strings.CutPrefix(selector, "New"); ok {
		if _, ok := typeToFile[typeName]; ok {
			return typeName, true
		}
	}

	if idx := strings.Index(selector, "_"); idx > 0 {
		typeName := selector[:idx]
		if _, ok := typeToFile[typeName]; ok {
			return typeName, true
		}
	}

	return "", false
}

func computeClosure(importsDir string, typeToFile map[string]string, roots map[string]struct{}) (map[string]struct{}, error) {
	queue := make([]string, 0, len(roots))
	for root := range roots {
		if _, ok := typeToFile[root]; ok {
			queue = append(queue, root)
		}
	}

	keep := make(map[string]struct{})
	for len(queue) > 0 {
		name := queue[len(queue)-1]
		queue = queue[:len(queue)-1]
		if _, ok := keep[name]; ok {
			continue
		}

		path, ok := typeToFile[name]
		if !ok {
			continue
		}
		keep[name] = struct{}{}

		deps, err := fileDependencies(path, typeToFile)
		if err != nil {
			return nil, err
		}
		for dep := range deps {
			if _, ok := keep[dep]; !ok {
				queue = append(queue, dep)
			}
		}

		for _, suffix := range []string{"__checks.go", "__no_checks.go"} {
			checkPath := filepath.Join(importsDir, name+suffix)
			if _, err := os.Stat(checkPath); err == nil {
				checkDeps, err := fileDependencies(checkPath, typeToFile)
				if err != nil {
					return nil, err
				}
				for dep := range checkDeps {
					if _, ok := keep[dep]; !ok {
						queue = append(queue, dep)
					}
				}
			}
		}
	}

	return keep, nil
}

func fileDependencies(path string, typeToFile map[string]string) (map[string]struct{}, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	deps := make(map[string]struct{})
	ast.Inspect(file, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := typeToFile[ident.Name]; ok {
			deps[ident.Name] = struct{}{}
		}
		return true
	})

	return deps, nil
}

func buildKeepFiles(importsDir string, typeToFile map[string]string, keepTypes map[string]struct{}) map[string]struct{} {
	keepFiles := map[string]struct{}{
		filepath.Join(importsDir, "main.go"): {},
	}

	for typeName := range keepTypes {
		keepFiles[typeToFile[typeName]] = struct{}{}
		for _, suffix := range []string{"__checks.go", "__no_checks.go"} {
			path := filepath.Join(importsDir, typeName+suffix)
			if _, err := os.Stat(path); err == nil {
				keepFiles[path] = struct{}{}
			}
		}
	}

	return keepFiles
}

func findRemoveFiles(importsDir string, keepFiles map[string]struct{}) []string {
	entries, err := os.ReadDir(importsDir)
	if err != nil {
		fatalf("read %s: %v", importsDir, err)
	}

	var remove []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		path := filepath.Join(importsDir, entry.Name())
		if _, ok := keepFiles[path]; !ok {
			remove = append(remove, path)
		}
	}

	sort.Strings(remove)
	return remove
}

func rewriteMainFile(path string, keepTypes map[string]struct{}) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "init" || fn.Body == nil {
			continue
		}

		var kept []ast.Stmt
		for _, stmt := range fn.Body.List {
			if shouldKeepStmt(stmt, keepTypes) {
				kept = append(kept, stmt)
			}
		}
		fn.Body.List = kept
		break
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, file); err != nil {
		return fmt.Errorf("print %s: %w", path, err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format %s: %w", path, err)
	}

	return os.WriteFile(path, formatted, 0o644)
}

func shouldKeepStmt(stmt ast.Stmt, keepTypes map[string]struct{}) bool {
	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return true
	}

	call, ok := exprStmt.X.(*ast.CallExpr)
	if !ok {
		return true
	}

	if len(call.Args) == 0 {
		return true
	}

	firstArg, ok := call.Args[0].(*ast.BasicLit)
	if !ok || firstArg.Kind != token.STRING {
		return true
	}

	value, err := strconv.Unquote(firstArg.Value)
	if err != nil {
		return true
	}

	typeName, ok := strings.CutPrefix(value, "k8s.")
	if !ok {
		return true
	}

	_, ok = keepTypes[typeName]
	return ok
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
