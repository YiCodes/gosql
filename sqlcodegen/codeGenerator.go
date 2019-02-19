package sqlcodegen

import (
	"fmt"
	"io"

	"go/ast"
)

type codeGenerator struct {
	writer      io.Writer
	indentLevel int
	settings    generatorSettings
	newLine     bool
}

type generatorSettings struct {
	indent string
	line   string
}

func newGenerator() *codeGenerator {
	g := &codeGenerator{}
	g.settings.indent = "\t"
	g.settings.line = "\r\n"
	g.newLine = true

	return g
}

func (g *codeGenerator) writePackage(packageName string) {
	g.write("package ")
	g.write(packageName)
	g.writeLine()
	g.writeLine()
}

func (g *codeGenerator) writeImportList(importList ...*ast.ImportSpec) {
	g.write("import (")
	g.writeLine()
	g.indentLevel++

	for _, im := range importList {
		if im.Name != nil && im.Name.Name != "" {
			g.writeExpr(im.Name)
			g.write(" ")
		}
		g.write(im.Path.Value)
		g.writeLine()
	}

	g.indentLevel--
	g.write(")")
	g.writeLine()
	g.writeLine()
}

func (g *codeGenerator) beginBlock() {
	g.write(" {")
	g.indentLevel++
	g.writeLine()
}

func (g *codeGenerator) endBlock(codes ...string) {
	g.indentLevel--
	g.write("}")
	g.writeLine(codes...)
}

func (g *codeGenerator) write(code string) {
	if g.newLine {
		g.writeIndent()
		g.newLine = false
	}

	g.writer.Write([]byte(code))
}

func (g *codeGenerator) writeExpr(expr ast.Expr) {
	switch inst := expr.(type) {
	case *ast.Ident:
		g.write(inst.Name)
	case *ast.SelectorExpr:
		g.writeExpr(inst.X)
		g.write(".")
		g.write(inst.Sel.Name)
	}
}

func (g *codeGenerator) beginFunc(funcName string, paramList []*ast.Field, returnList []*ast.Field) {
	g.write("func ")
	g.write(funcName)
	g.writeFuncType(paramList, true)

	if len(returnList) > 0 {
		g.write(" ")
		g.writeFuncType(returnList, len(returnList) > 1)
	}

	g.beginBlock()
}

func (g *codeGenerator) endFunc() {
	g.endBlock()
}

func (g *codeGenerator) writeFuncType(list []*ast.Field, hasParenthesis bool) {
	if hasParenthesis {
		g.write("(")
	}

	for i, field := range list {
		if i > 0 {
			g.write(", ")
		}

		if len(field.Names) > 0 {
			g.write(field.Names[0].Name)
			g.write(" ")
		}
		g.writeExpr(field.Type)
	}

	if hasParenthesis {
		g.write(")")
	}
}

func (g *codeGenerator) writeLine(codes ...string) {
	for _, code := range codes {
		g.write(code)
	}

	g.writer.Write([]byte(g.settings.line))
	g.newLine = true
}

func (g *codeGenerator) writeIndent() {
	for index := 0; index < g.indentLevel; index++ {
		g.writer.Write([]byte(g.settings.indent))
	}
}

func (g *codeGenerator) writeStringValue(value string) {
	g.write("\"")
	g.write(value)
	g.write("\"")
}

func (g *codeGenerator) writeDoc(doc *ast.CommentGroup) {
	if doc != nil {
		for _, c := range doc.List {
			g.writeLine(c.Text)
		}
	}
}

func (g *codeGenerator) writeConstDeclaration(varName string, initValue interface{}) {
	g.write("const ")
	g.write(varName)
	g.write(" = ")

	switch inst := initValue.(type) {
	case string:
		g.writeStringValue(inst)
	default:
		g.write(fmt.Sprint(inst))
	}
	g.writeLine()
}

func (g *codeGenerator) writeVarDeclaration(varName string, varType ast.Expr, newObj bool) {
	g.write("var ")
	g.write(varName)

	if newObj {
		g.write(" = ")
		g.write("new(")
		g.writeExpr(varType)
		g.write(")")
	} else {
		g.write(" ")
		g.writeExpr(varType)
	}

	g.writeLine()
}
