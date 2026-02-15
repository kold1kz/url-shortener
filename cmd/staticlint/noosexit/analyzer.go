package noosexit

import (
	"go/ast"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

var Analyzer = &analysis.Analyzer{
	Name:     "noosexit",
	Doc:      "forbids direct os.Exit calls inside func main of package main",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	if pass.Pkg == nil || pass.Pkg.Name() != "main" {
		return nil, nil
	}

	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil)}, func(n ast.Node) {
		fd := n.(*ast.FuncDecl)

		if fd.Recv != nil || fd.Name == nil || fd.Name.Name != "main" || fd.Body == nil {
			return
		}

		ast.Inspect(fd.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}

			pkgIdent, ok := sel.X.(*ast.Ident)
			if !ok || pkgIdent.Name != "os" {
				return true
			}
			if sel.Sel == nil || sel.Sel.Name != "Exit" {
				return true
			}

			pos := pass.Fset.Position(sel.Sel.Pos())

			// Игнорируем go build cache
			if strings.Contains(pos.Filename, string(filepath.Separator)+"go-build"+string(filepath.Separator)) ||
				strings.Contains(pos.Filename, "Library/Caches/go-build") {
				return true
			}

			pass.Reportf(sel.Sel.Pos(),
				"direct os.Exit call in main is forbidden; return error or handle termination without os.Exit")

			return true
		})
	})

	return nil, nil
}
