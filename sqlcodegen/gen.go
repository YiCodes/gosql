package sqlcodegen

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func newArgError(context *parseContext, node ast.Node) error {
	return fmt.Errorf(
		"error: function argument error(%v)",
		context.fset.Position(node.Pos()))
}

func newTypeDefError(context *parseContext, typeName string, node ast.Node) error {
	return fmt.Errorf(
		"error: type %s definition error(%v)",
		typeName, context.fset.Position(node.Pos()))
}

func newUnsupportedError(context *parseContext, node ast.Node) error {
	return fmt.Errorf(
		"warn: unsupported(%v)",
		context.fset.Position(node.Pos()))
}

type table struct {
	name      string
	tableName string
	source    *ast.GenDecl

	columns []*column
}

func (t *table) getColumn(name string) (*column, bool) {
	for _, col := range t.columns {
		if col.name == name {
			return col, true
		}
	}

	return nil, false
}

type column struct {
	name       string
	columnName string
	tag        string
	sysType    string
	isNull     bool
	isIdentity bool
}

type parseContext struct {
	fset       *token.FileSet
	entity     map[string]*table
	tables     []*table
	generator  *codeGenerator
	sqlBuilder SQLBuilder
}

func (context *parseContext) getEntityWithExpr(expr ast.Expr) (*table, bool) {
	var entityName string

	switch instExpr := expr.(type) {
	case *ast.Ident:
		entityName = instExpr.Name
	case *ast.SelectorExpr:
		entityExpr, ok := instExpr.X.(*ast.Ident)

		if ok {
			entityName = entityExpr.Name
		}
	}

	table, ok := context.entity[entityName]

	return table, ok
}

type Options struct {
	SQLBuilder SQLBuilder
}

func Compile(srcFileName string, outFileName string, opts Options) error {
	context := parseContext{}
	context.entity = make(map[string]*table)
	context.fset = token.NewFileSet()
	context.generator = newGenerator()

	if opts.SQLBuilder == nil {
		context.sqlBuilder = newDefaultSQLBuilder()
	} else {
		context.sqlBuilder = opts.SQLBuilder
	}

	file, err := parser.ParseFile(context.fset, srcFileName, nil, parser.ParseComments)

	if err != nil {
		return err
	}

	for _, decl := range file.Decls {
		inst, ok := decl.(*ast.GenDecl)
		if ok {
			if inst.Tok == token.TYPE {
				err := addTable(&context, inst)

				if err != nil {
					return err
				}
			}
		}
	}

	outWriter, err := os.Create(outFileName)

	if err != nil {
		return err
	}

	defer outWriter.Close()
	context.generator.writer = outWriter

	packName := file.Name.Name

	var needSqlPackage bool

	for _, decl := range file.Decls {
		inst, ok := decl.(*ast.FuncDecl)

		if ok {
			if inst.Name.Name == "init" {
				for callExpr := range getCallExprList(inst) {
					fun := callExpr.Fun.(*ast.SelectorExpr)

					switch fun.Sel.Name {
					case "SetPackageName":
						lit, ok := callExpr.Args[0].(*ast.BasicLit)

						if !ok ||
							!(strings.HasPrefix(lit.Value, "\"") && strings.HasSuffix(lit.Value, "\"")) {
							return newArgError(&context, callExpr)
						}

						packName = lit.Value[1 : len(lit.Value)-1]
					default:
						fmt.Println(newUnsupportedError(&context, callExpr))
					}
				}
			} else {
				for callExpr := range getCallExprList(inst) {
					fun := callExpr.Fun.(*ast.SelectorExpr)

					methodName := fun.Sel.Name

					if strings.HasPrefix(methodName, "Insert") ||
						strings.HasPrefix(methodName, "Delete") ||
						strings.HasPrefix(methodName, "Update") {
						needSqlPackage = true
						break
					}
				}
			}
		}
	}

	generator := context.generator

	generator.writePackage(packName)

	imports := []*ast.ImportSpec{
		newASTImportSpec("context", ""),
		newASTImportSpec("github.com/YiCodes/gosql/sqlutil", ""),
	}

	if needSqlPackage {
		imports = append(imports, newASTImportSpec("database/sql", ""))
	}

	for _, p := range file.Imports {
		switch getBasicLitValue(p.Path) {
		case "context", "github.com/YiCodes/gosql/sqlcodegen":
			continue
		}

		imports = append(imports, p)
	}

	generator.writeImportList(imports...)

	for _, decl := range file.Decls {
		inst, ok := decl.(*ast.GenDecl)
		if ok {
			if inst.Tok == token.VAR {
				err := addEntity(&context, inst)

				if err != nil {
					return err
				}
			}
		}
	}

	for _, t := range context.tables {
		generator.writeDoc(t.source.Doc)
		generator.write("type ")
		generator.write(t.name)
		generator.write(" struct")
		generator.beginBlock()

		var maxFieldSize = 0

		for _, c := range t.columns {
			fieldSize := len(c.name)

			if fieldSize > maxFieldSize {
				maxFieldSize = fieldSize
			}
		}

		for _, c := range t.columns {
			generator.write(c.name)
			generator.write(strings.Repeat(" ", maxFieldSize-len(c.name)))

			generator.write(" ")
			generator.write(c.sysType)

			if len(c.tag) > 0 {
				generator.write(" ")
				generator.write(c.tag)
			}

			generator.writeLine()
		}

		generator.endBlock()
	}

	generator.writeLine()

	for _, decl := range file.Decls {
		inst, ok := decl.(*ast.FuncDecl)

		if ok {
		CheckSqlMethodLoop:
			for callExpr := range getCallExprList(inst) {
				fun := callExpr.Fun.(*ast.SelectorExpr)

				methodName := fun.Sel.Name
				var err error

				if strings.HasPrefix(methodName, "Insert") {
					err = genInsertFunction(&context, inst)
				} else if strings.HasPrefix(methodName, "Select") {
					err = genSelectFunction(&context, inst)
				} else if strings.HasPrefix(methodName, "Delete") {
					err = genDeleteFunction(&context, inst)
				} else if strings.HasPrefix(methodName, "Update") {
					err = genUpdateFunction(&context, inst)
				} else {
					continue
				}

				if err != nil {
					return err
				}
				break CheckSqlMethodLoop
			}
		}
	}

	return nil
}

func getCallExprList(funcDecl *ast.FuncDecl) <-chan *ast.CallExpr {
	channel := make(chan *ast.CallExpr)

	go func() {
		defer close(channel)

		for _, stmt := range funcDecl.Body.List {
			exprStmt, ok := stmt.(*ast.ExprStmt)

			if ok {
				callExpr, ok := exprStmt.X.(*ast.CallExpr)

				if ok {
					_, ok := callExpr.Fun.(*ast.SelectorExpr)

					if ok {
						channel <- callExpr
					}
				}
			}
		}
	}()

	return channel
}

func findSpecCall(funcDecl *ast.FuncDecl, callMethod string) *ast.CallExpr {
	for callExpr := range getCallExprList(funcDecl) {
		fun := callExpr.Fun.(*ast.SelectorExpr)

		if fun.Sel.Name == callMethod {
			return callExpr
		}
	}

	return nil
}

func getFuncParamNames(funcDecl *ast.FuncDecl) map[string]int {
	paramNames := make(map[string]int)

	for i, arg := range funcDecl.Type.Params.List {
		paramNames[arg.Names[0].Name] = i
	}

	return paramNames
}

func genSelectFunction(context *parseContext, funcDecl *ast.FuncDecl) error {
	var selectExpr *ast.CallExpr
	var isSelectAll bool
	var fromExpr *ast.CallExpr
	var orderByList []*SQLOrderExpression
	var whereExpr *ast.CallExpr
	var returnTypeFlag ReturnType
	var chanBufferSize int

	for callExpr := range getCallExprList(funcDecl) {
		fun := callExpr.Fun.(*ast.SelectorExpr)

		switch fun.Sel.Name {
		case "Select":
			selectExpr = callExpr
		case "SelectAll":
			selectExpr = callExpr
			isSelectAll = true
		case "From":
			fromExpr = callExpr
		case "Where":
			whereExpr = callExpr
		case "OrderBy", "OrderByDescending":
			entity, ok := context.getEntityWithExpr(callExpr.Args[0])

			if !ok {
				return newArgError(context, callExpr)
			}

			colExpr, ok := (callExpr.Args[0].(*ast.SelectorExpr))

			if !ok {
				return newArgError(context, callExpr)
			}

			col, ok := entity.getColumn(colExpr.Sel.Name)

			if !ok {
				return newArgError(context, callExpr)
			}

			sqlColExpr := &SQLColumnExpression{}
			sqlColExpr.columnName = col.columnName
			sqlColExpr.source = col
			sqlColExpr.tableName = entity.tableName

			isDesc := fun.Sel.Name == "OrderByDescending"

			sqlOrderExpr := &SQLOrderExpression{column: sqlColExpr, isDescending: isDesc}

			orderByList = append(orderByList, sqlOrderExpr)

		case "SetReturnType":
			if len(callExpr.Args) != 1 {
				return newArgError(context, callExpr)
			}

			c := (callExpr.Args[0].(*ast.SelectorExpr)).Sel.Name

			switch c {
			case "ReturnDefault":
				returnTypeFlag = ReturnDefault
			case "ReturnExecResult":
				returnTypeFlag = ReturnExecResult
			case "ReturnScalar":
				returnTypeFlag = ReturnScalar
			case "ReturnScalarSet":
				returnTypeFlag = ReturnScalarSet
			case "ReturnRecord":
				returnTypeFlag = ReturnRecord
			case "ReturnRecordSet":
				returnTypeFlag = ReturnRecordSet
			case "ReturnRecordChannel":
				returnTypeFlag = ReturnRecordChannel
			default:
				return newArgError(context, callExpr)
			}
		case "SetChannelBufferSize":
			if len(callExpr.Args) != 1 {
				return newArgError(context, callExpr)
			}

			lit, ok := callExpr.Args[0].(*ast.BasicLit)

			if !ok {
				return newArgError(context, callExpr)
			}

			chanBufferSize, _ = strconv.Atoi(lit.Value)
		}
	}

	selectStmt := &SQLSelectStatement{}
	selectStmt.orderByList = orderByList

	if isSelectAll {
		entity, ok := context.getEntityWithExpr(selectExpr.Args[0])

		if !ok {
			return newArgError(context, selectExpr)
		}

		selectStmt = tableToSelectStatement(context.sqlBuilder, selectStmt, entity)
	} else {
		for _, expr := range selectExpr.Args {
			entity, ok := context.getEntityWithExpr(expr)

			if !ok {
				return newArgError(context, selectExpr)
			}

			colExpr, ok := expr.(*ast.SelectorExpr)

			if !ok {
				return newArgError(context, selectExpr)
			}

			sqlColExpr := &SQLColumnExpression{}
			sqlColExpr.tableName = entity.tableName

			col, ok := entity.getColumn(colExpr.Sel.Name)

			if !ok {
				return newArgError(context, selectExpr)
			}

			sqlColExpr.source = col
			sqlColExpr.columnName = col.columnName

			selectStmt.selectList = append(selectStmt.selectList, sqlColExpr)
		}
	}

	if fromExpr != nil {
		tableName, err := getTableNameWithExpr(context, fromExpr)

		if err != nil {
			return err
		}

		selectStmt.table = tableName
	}

	if sqlWhereExpr, err := astToSQLWhereExpression(context, funcDecl, whereExpr); err == nil {
		selectStmt.where = sqlWhereExpr
	} else {
		return err
	}

	if returnTypeFlag == ReturnDefault {
		returnTypeFlag = ReturnRecordSet
	}

	entity, ok := context.getEntityWithExpr(selectExpr.Args[0])

	if !ok {
		return newArgError(context, selectExpr)
	}

	var funcReturnList []*ast.Field
	var returnElementType string

	switch returnTypeFlag {
	case ReturnRecordSet:
		funcReturnList = append(funcReturnList,
			newASTField(newASTRefExpr("[]*"+entity.name), ""))
		returnElementType = entity.name
	case ReturnRecordChannel:
		funcReturnList = append(funcReturnList,
			newASTField(newASTRefExpr("<-chan *"+entity.name), ""))
		returnElementType = entity.name
	case ReturnRecord:
		funcReturnList = append(funcReturnList,
			newASTField(newASTRefExpr("*"+entity.name), ""))
		returnElementType = entity.name
	case ReturnScalar:
		sqlColExpr, ok := selectStmt.getFirstColumnExpression()

		if !ok {
			return newArgError(context, selectExpr)
		}

		funcReturnList = append(funcReturnList,
			newASTField(newASTRefExpr(sqlColExpr.source.sysType), ""))
		returnElementType = sqlColExpr.source.sysType
	case ReturnScalarSet:
		sqlColExpr, ok := selectStmt.getFirstColumnExpression()

		if !ok {
			return newArgError(context, selectExpr)
		}

		funcReturnList = append(funcReturnList,
			newASTField(newASTRefExpr("[]*"+sqlColExpr.source.sysType), ""))
		returnElementType = sqlColExpr.source.sysType
	}

	if returnTypeFlag == ReturnRecordChannel {
		funcReturnList = append(funcReturnList,
			newASTField(newASTRefExpr("context.CancelFunc"), ""))
	}

	context.sqlBuilder.Reset()
	context.sqlBuilder.WriteSelectStatement(selectStmt)
	sqlText := context.sqlBuilder.String()

	genMethodBegin(context, funcDecl.Name.Name, funcDecl.Type.Params.List, funcReturnList, funcDecl.Doc)
	generator := context.generator

	generator.writeConstDeclaration("query", sqlText)
	generator.write("rows, err := db.QueryContext(context.Background(), query")

	for _, p := range getSelectStmtSqlParamList(selectStmt) {
		generator.write(", ")
		generator.write(p.name)
	}

	generator.writeLine(")")

	generator.write("if err != nil")
	generator.beginBlock()

	if returnTypeFlag == ReturnRecordChannel {
		generator.writeLine("return nil, nil, err")
	} else {
		generator.writeLine("return nil, err")
	}

	generator.endBlock()

	if returnTypeFlag != ReturnRecordChannel {
		generator.writeLine("defer rows.Close()")
	}

	switch returnTypeFlag {
	case ReturnRecordSet:
		generator.writeVarDeclaration("result", funcReturnList[0].Type, false)

		generator.write("for rows.Next()")
		generator.beginBlock()

		generator.writeVarDeclaration("o", newASTRefExpr(returnElementType), true)

		generator.write("rows.Scan(")

		for i, selectExpr := range selectStmt.selectList {
			if i > 0 {
				generator.write(", ")
			}

			generator.write("&o.")
			generator.write((selectExpr.(*SQLColumnExpression)).source.name)
		}

		generator.writeLine(")")
		generator.writeLine("result = append(result, o)")

		generator.endBlock()

		generator.writeLine("return result, nil")

	case ReturnRecord:
		generator.write("if rows.Next()")
		generator.beginBlock()

		generator.writeVarDeclaration("o", newASTRefExpr(returnElementType), true)

		generator.write("rows.Scan(")

		for i, selectExpr := range selectStmt.selectList {
			if i > 0 {
				generator.write(", ")
			}

			generator.write("&o.")
			generator.write((selectExpr.(*SQLColumnExpression)).source.name)
		}

		generator.writeLine(")")
		generator.writeLine("return o, nil")

		generator.endBlock()

		generator.writeLine("return nil, nil")
	case ReturnScalar:
		generator.write("if rows.Next()")
		generator.beginBlock()
		generator.writeVarDeclaration("o", funcReturnList[0].Type, true)

		generator.writeLine("rows.Scan(o)")
		generator.writeLine("return o, nil")

		generator.endBlock()
		generator.writeLine("return nil, nil")

	case ReturnScalarSet:
		generator.writeVarDeclaration("result", funcReturnList[0].Type, false)

		generator.write("for rows.Next()")
		generator.beginBlock()

		generator.writeVarDeclaration("o", newASTRefExpr(returnElementType), true)

		generator.writeLine("rows.Scan(o)")
		generator.writeLine("result = append(result, o)")

		generator.endBlock()
		generator.writeLine("return result, nil")

	case ReturnRecordChannel:
		if chanBufferSize > 0 {
			generator.writeLine("channel := make(chan *", returnElementType, ", ", strconv.Itoa(chanBufferSize), ")")
		} else {
			generator.writeLine("channel := make(chan *", returnElementType, ")")
		}

		generator.writeLine("ctx, cancel := context.WithCancel(context.Background())")
		generator.write("go func()")
		generator.beginBlock()
		generator.writeLine("defer close(channel)")
		generator.writeLine("defer rows.Close()")
		generator.write("for rows.Next()")
		generator.beginBlock()

		generator.writeVarDeclaration("o", newASTRefExpr(returnElementType), true)

		generator.write("rows.Scan(")

		for i, selectExpr := range selectStmt.selectList {
			if i > 0 {
				generator.write(", ")
			}

			generator.write("&o.")
			generator.write((selectExpr.(*SQLColumnExpression)).source.name)
		}

		generator.writeLine(")")

		generator.writeLine("select {")
		generator.writeLine("case <-ctx.Done(): ")
		generator.indentLevel++
		generator.writeLine("return")
		generator.indentLevel--
		generator.writeLine("case channel <- o:")
		generator.writeLine("}")

		generator.endBlock()
		generator.endBlock("()")

		generator.writeLine("return channel, cancel, nil")
	}

	genMethodEnd(context)

	return nil
}

func genMethodEnd(context *parseContext) {
	context.generator.endBlock()
}

func genMethodBegin(context *parseContext, funcName string, paramList []*ast.Field, returnList []*ast.Field, doc *ast.CommentGroup) {
	generator := context.generator

	paramListCopy := make([]*ast.Field, len(paramList)+1)

	paramListCopy[0] = newASTField(newASTRefExpr("sqlutil.DbObject"), "db")
	copy(paramListCopy[1:], paramList)

	returnListCopy := make([]*ast.Field, len(returnList), len(returnList)+1)
	copy(returnListCopy, returnList)
	returnListCopy = append(returnListCopy, newASTField(newASTRefExpr("error"), ""))

	generator.writeDoc(doc)
	generator.beginFunc(funcName, paramListCopy, returnListCopy)
}

func astToSQLExpression(expr ast.Expr, context *parseContext, paramNames map[string]int) (SQLExpression, error) {
	switch inst := expr.(type) {
	case *ast.Ident:
		_, ok := paramNames[inst.Name]
		if ok {
			return &SQLParameterExpression{name: inst.Name}, nil
		}
		return nil, errors.New("")

	case *ast.BasicLit:
		return &SQLLiteralExpression{value: inst.Value}, nil

	case *ast.SelectorExpr:
		entity, ok := context.getEntityWithExpr(inst)
		if ok {
			sqlColExpr := &SQLColumnExpression{}

			col, ok := entity.getColumn(inst.Sel.Name)

			if !ok {
				return nil, errors.New("")
			}

			sqlColExpr.source = col
			sqlColExpr.columnName = col.columnName
			sqlColExpr.tableName = entity.tableName

			return sqlColExpr, nil
		}
		return nil, errors.New("")

	case *ast.ParenExpr:
		sqlExpr, err := astToSQLExpression(inst.X, context, paramNames)

		if err != nil {
			return nil, err
		}

		return &SQLParenthesisExpression{target: sqlExpr}, nil

	case *ast.BinaryExpr:
		sqlBinExpr := &SQLBinaryExpression{}
		sqlExpr, err := astToSQLExpression(inst.X, context, paramNames)

		if err != nil {
			return nil, err
		}

		sqlBinExpr.left = sqlExpr
		sqlBinExpr.op = inst.Op.String()

		sqlExpr, err = astToSQLExpression(inst.Y, context, paramNames)

		if err != nil {
			return nil, err
		}

		sqlBinExpr.right = sqlExpr
		return sqlBinExpr, nil
	}

	return nil, errors.New("")
}

func getTableNameWithExpr(context *parseContext, fromExpr *ast.CallExpr) (string, error) {
	if len(fromExpr.Args) == 1 {
		entityName, ok := fromExpr.Args[0].(*ast.Ident)

		if ok {
			entity, ok := context.entity[entityName.Name]

			if ok {
				return entity.tableName, nil
			}
		}
	}

	return "", newArgError(context, fromExpr)
}

func astToSQLWhereExpression(context *parseContext, funcDecl *ast.FuncDecl, whereExpr *ast.CallExpr) (SQLExpression, error) {
	if whereExpr == nil {
		return nil, nil
	}

	if len(whereExpr.Args) != 1 {
		return nil, newArgError(context, whereExpr)
	}

	funcParamNames := getFuncParamNames(funcDecl)
	sqlWhereExpr, err := astToSQLExpression(whereExpr.Args[0], context, funcParamNames)

	if err != nil {
		return nil, newArgError(context, whereExpr)
	}

	return sqlWhereExpr, nil
}

func genDeleteFunction(context *parseContext, funcDecl *ast.FuncDecl) error {
	var whereExpr *ast.CallExpr
	var deleteExpr *ast.CallExpr

	for callExpr := range getCallExprList(funcDecl) {
		fun := callExpr.Fun.(*ast.SelectorExpr)

		switch fun.Sel.Name {
		case "Delete":
			deleteExpr = callExpr
		case "Where":
			whereExpr = callExpr
		}
	}

	deleteStmt := &SQLDeleteStatement{}

	tableName, err := getTableNameWithExpr(context, deleteExpr)

	if err != nil {
		return err
	}

	deleteStmt.table = tableName

	sqlWhereExpr, err := astToSQLWhereExpression(context, funcDecl, whereExpr)

	if err != nil {
		return err
	}

	deleteStmt.where = sqlWhereExpr

	funcResultList := []*ast.Field{newASTField(newASTRefExpr("sql.Result"), "")}

	genMethodBegin(context, funcDecl.Name.Name, funcDecl.Type.Params.List, funcResultList, funcDecl.Doc)

	context.sqlBuilder.Reset()
	context.sqlBuilder.WriteDeleteStatement(deleteStmt)
	sqlText := context.sqlBuilder.String()

	generator := context.generator
	generator.writeConstDeclaration("query", sqlText)
	generator.write("return db.ExecContext(context.Background(), query")

	sqlParamList := getDeleteStmtSqlParamList(deleteStmt)

	for _, p := range sqlParamList {
		generator.write(", ")
		generator.write(p.name)
	}

	generator.writeLine(")")

	genMethodEnd(context)

	return nil
}

func genUpdateFunction(context *parseContext, funcDecl *ast.FuncDecl) error {
	var updateExprList []*ast.CallExpr
	var whereExpr *ast.CallExpr
	var fromExpr *ast.CallExpr

	for callExpr := range getCallExprList(funcDecl) {
		fun := callExpr.Fun.(*ast.SelectorExpr)

		switch fun.Sel.Name {
		case "Update":
			updateExprList = append(updateExprList, callExpr)
		case "From":
			fromExpr = callExpr
		case "Where":
			whereExpr = callExpr
		}
	}

	updateStmt := &SQLUpdateStatement{}

	if fromExpr != nil {
		tableName, err := getTableNameWithExpr(context, fromExpr)

		if err != nil {
			return err
		}

		updateStmt.table = tableName
	}

	if sqlWhereExpr, err := astToSQLWhereExpression(context, funcDecl, whereExpr); err == nil {
		updateStmt.where = sqlWhereExpr
	} else {
		return err
	}

	for _, updateExpr := range updateExprList {
		if len(updateExpr.Args) != 2 {
			return newArgError(context, updateExpr)
		}

		funcFuncNames := getFuncParamNames(funcDecl)

		sqlExpr, err := astToSQLExpression(updateExpr.Args[0], context, funcFuncNames)

		if err != nil {
			return newArgError(context, updateExpr)
		}

		if _, ok := sqlExpr.(*SQLColumnExpression); !ok {
			return newArgError(context, updateExpr)
		}

		sqlAssignExpr := &SQLBinaryExpression{}
		sqlAssignExpr.left = sqlExpr
		sqlAssignExpr.op = "="
		sqlExpr, err = astToSQLExpression(updateExpr.Args[1], context, funcFuncNames)

		if err != nil {
			return newArgError(context, updateExpr)
		}

		if _, ok := sqlExpr.(*SQLParameterExpression); ok {

		} else if _, ok := sqlExpr.(*SQLLiteralExpression); ok {

		} else {
			return newArgError(context, updateExpr)
		}

		sqlAssignExpr.right = sqlExpr

		updateStmt.updateList = append(updateStmt.updateList, sqlAssignExpr)
	}

	funcResultList := []*ast.Field{newASTField(newASTRefExpr("sql.Result"), "")}

	genMethodBegin(context, funcDecl.Name.Name, funcDecl.Type.Params.List, funcResultList, funcDecl.Doc)

	context.sqlBuilder.Reset()
	context.sqlBuilder.WriteUpdateStatement(updateStmt)
	sqlText := context.sqlBuilder.String()

	generator := context.generator
	generator.writeConstDeclaration("query", sqlText)
	generator.write("return db.ExecContext(context.Background(), query")

	sqlParamList := getUpdateStmtSqlParamList(updateStmt)

	for _, p := range sqlParamList {
		generator.write(", ")
		generator.write(p.name)
	}

	generator.writeLine(")")

	genMethodEnd(context)

	return nil
}

func genInsertFunction(context *parseContext, funcDecl *ast.FuncDecl) error {
	insertModelCall := findSpecCall(funcDecl, "InsertAll")

	if insertModelCall == nil {

	} else {
		arg, ok := insertModelCall.Args[0].(*ast.Ident)

		if ok {
			entity, ok := context.entity[arg.Name]

			if !ok {
				return newArgError(context, insertModelCall)
			}

			generator := context.generator

			var paramList []*ast.Field
			paramList = append(paramList, newASTField(newASTRefExpr("sqlutil.DbObject"), "db"))
			paramList = append(paramList, newASTField(newASTRefExpr("*"+entity.name), "o"))

			var returnList []*ast.Field
			returnList = append(returnList, newASTField(newASTRefExpr("sql.Result"), ""))
			returnList = append(returnList, newASTField(newASTRefExpr("error"), ""))

			generator.beginFunc(funcDecl.Name.Name, paramList, returnList)

			context.sqlBuilder.Reset()
			context.sqlBuilder.WriteInsertStatement(
				tableToInsertStatement(context.sqlBuilder, nil, entity))

			sqlText := context.sqlBuilder.String()

			generator.writeConstDeclaration("query", sqlText)

			generator.write("return db.ExecContext(context.Background(), query")

			for _, col := range entity.columns {
				if col.isIdentity {
					continue
				}

				generator.write(",")
				generator.write("o.")
				generator.write(col.name)
			}

			generator.write(")")
			generator.writeLine()
			generator.endBlock()
		} else {
			return newArgError(context, insertModelCall)
		}
	}

	return nil
}

func getTypeName(expr ast.Expr) string {
	switch inst := expr.(type) {
	case *ast.Ident:
		return inst.Name
	case *ast.SelectorExpr:
		return getTypeName(inst.X) + "." + inst.Sel.Name
	}

	return ""
}

func addEntity(context *parseContext, genDecl *ast.GenDecl) error {
	for _, spec := range genDecl.Specs {
		varSpec, _ := spec.(*ast.ValueSpec)

		tableType := getTypeName(varSpec.Type)

		for index := 0; index < len(context.tables); index++ {
			if context.tables[index].name == tableType {
				context.entity[varSpec.Names[0].Name] = context.tables[index]
				return nil
			}
		}
	}

	return nil
}

func getTags(code string) map[string]string {
	result := make(map[string]string)

	tags := strings.Fields(code)

	reg := regexp.MustCompile(`(\w+):"(.+)"`)
	for _, t := range tags {
		kv := reg.FindStringSubmatch(t)

		if len(kv) != 3 {
			continue
		}

		result[kv[1]] = kv[2]
	}

	return result
}

func addTable(context *parseContext, genDecl *ast.GenDecl) error {
	table := &table{}

	for _, spec := range genDecl.Specs {
		typeSpec, _ := spec.(*ast.TypeSpec)

		table.name = typeSpec.Name.Name
		table.source = genDecl

		structType, ok := typeSpec.Type.(*ast.StructType)

		if !ok {
			return newTypeDefError(context, typeSpec.Name.Name, genDecl)
		}

		table.columns = make([]*column, 0, 4)

		for _, field := range structType.Fields.List {
			if strings.HasSuffix(getTypeName(field.Type), "TableName") &&
				field.Tag != nil {
				tags := getTags(field.Tag.Value)
				name, ok := tags["tableName"]

				if ok {
					table.tableName = name
					continue
				}
			}

			column := &column{}
			column.name = field.Names[0].Name

			switch columnType := field.Type.(type) {
			case *ast.Ident:
				column.sysType = columnType.Name
			case *ast.SelectorExpr:
				column.sysType = getTypeName(columnType)
				column.isNull = strings.Index(columnType.Sel.Name, "Null") == 0
			}

			if field.Tag != nil {
				tags := getTags(field.Tag.Value)
				colIdent, ok := tags["identity"]

				if ok && colIdent == "true" {
					column.isIdentity = true
				}

				colName, ok := tags["name"]

				if ok {
					column.columnName = colName
				}

				column.tag = field.Tag.Value
			}

			if column.columnName == "" {
				column.columnName = column.name
			}

			table.columns = append(table.columns, column)
		}

		if table.tableName == "" {
			table.tableName = table.name
		}
	}

	context.tables = append(context.tables, table)

	return nil
}
