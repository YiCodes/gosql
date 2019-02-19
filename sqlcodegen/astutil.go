package sqlcodegen

import (
	"strings"

	"go/ast"
)

func newASTImportSpec(path string, name string) *ast.ImportSpec {
	im := &ast.ImportSpec{}
	im.Name = &ast.Ident{Name: name}
	im.Path = &ast.BasicLit{Value: `"` + path + `"`}

	return im
}

func newASTField(t ast.Expr, names ...string) *ast.Field {
	field := &ast.Field{}
	field.Type = t

	for _, n := range names {
		if n == "" {
			continue
		}

		field.Names = append(field.Names, &ast.Ident{Name: n})
	}

	return field
}

func getBasicLitValue(lit *ast.BasicLit) string {
	if strings.HasPrefix(lit.Value, "`") ||
		strings.HasPrefix(lit.Value, `"`) {
		return lit.Value[1 : len(lit.Value)-1]
	}

	return lit.Value
}

func newASTRefExpr(code string) ast.Expr {
	list := strings.Split(code, ".")

	var expr ast.Expr

	for _, x := range list {
		node := &ast.Ident{Name: x}

		if expr == nil {
			expr = node
		} else {
			selector := &ast.SelectorExpr{}
			selector.Sel = node
			selector.X = expr

			expr = selector
		}
	}

	return expr
}
