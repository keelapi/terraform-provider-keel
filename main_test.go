package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"testing"
)

// Release builds set the provider version with -ldflags "-X main.<name>=...".
// The linker silently ignores -X for a variable that does not exist, so check
// that every such flag names a package-level variable of main.go, and that
// version is what main passes to provider.New.
func TestLdflagsTargetMainVersion(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}

	vars := map[string]bool{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}
		for _, spec := range gen.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				vars[name.Name] = true
			}
		}
	}

	ldflag := regexp.MustCompile(`-X\s+main\.(\w+)=`)
	for _, name := range []string{".goreleaser.yml", "GNUmakefile"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		matches := ldflag.FindAllStringSubmatch(string(data), -1)
		if len(matches) == 0 {
			t.Errorf("%s sets no -X main.<variable> ldflag", name)
		}
		for _, m := range matches {
			if !vars[m[1]] {
				t.Errorf("%s sets -X main.%s, but main.go declares no variable %q", name, m[1], m[1])
			}
		}
	}

	passed := false
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "New" || len(call.Args) != 1 {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "provider" {
			if arg, ok := call.Args[0].(*ast.Ident); ok && arg.Name == "version" {
				passed = true
			}
		}
		return true
	})
	if !passed {
		t.Error("main.go must pass the version variable to provider.New")
	}
}
