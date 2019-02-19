package sqlcodegen

import (
	"bytes"
	"strconv"
	"strings"
)

type SQLExpression interface {
}

type SQLParameterExpression struct {
	name string
}

type SQLBinaryExpression struct {
	left  SQLExpression
	op    string
	right SQLExpression
}

type SQLLiteralExpression struct {
	value string
}

type SQLOrderExpression struct {
	column       *SQLColumnExpression
	isDescending bool
}

type SQLParenthesisExpression struct {
	target SQLExpression
}

type SQLColumnExpression struct {
	tableName  string
	columnName string

	source *column
}

type SQLSelectStatement struct {
	selectList  []SQLExpression
	table       string
	where       SQLExpression
	orderByList []*SQLOrderExpression
	limitRows   int
}

func (stmt *SQLSelectStatement) getFirstColumnExpression() (*SQLColumnExpression, bool) {
	if len(stmt.selectList) > 0 {
		expr, ok := stmt.selectList[0].(*SQLColumnExpression)
		return expr, ok
	}

	return nil, false
}

type SQLInsertStatement struct {
	columns []string
	table   string
}

type SQLUpdateStatement struct {
	updateList []SQLExpression
	table      string
	where      SQLExpression
}

type SQLDeleteStatement struct {
	table string
	where SQLExpression
}

type SQLBuilder interface {
	Reset()
	String() string
	Write(code string)
	WriteLine()
	WriteWhere(where SQLExpression)
	WriteDeleteStatement(stmt *SQLDeleteStatement)
	WriteUpdateStatement(stmt *SQLUpdateStatement)
	WriteInsertStatement(stmt *SQLInsertStatement)
	WriteSelectStatement(stmt *SQLSelectStatement)
	WriteSQLExpression(expr SQLExpression)
	GetInvokeParameterList(paramList []*SQLParameterExpression) []*SQLParameterExpression
}

type defaultSQLBuilder struct {
	buffer bytes.Buffer
}

func newDefaultSQLBuilder() *defaultSQLBuilder {
	b := &defaultSQLBuilder{}
	return b
}

func (builder *defaultSQLBuilder) Reset() {
	builder.buffer.Reset()
}

func (builder *defaultSQLBuilder) String() string {
	return builder.buffer.String()
}

func (builder *defaultSQLBuilder) Write(code string) {
	builder.buffer.WriteString(code)
}

func (builder *defaultSQLBuilder) WriteLine() {
	builder.Write(`\n`)
}

func (builder *defaultSQLBuilder) WriteWhere(where SQLExpression) {
	if where == nil {
		return
	}

	builder.Write("WHERE ")
	builder.WriteSQLExpression(where)
	builder.WriteLine()
}

func (builder *defaultSQLBuilder) WriteDeleteStatement(stmt *SQLDeleteStatement) {
	builder.Write("DELETE ")
	builder.Write(stmt.table)
	builder.WriteLine()
	builder.WriteWhere(stmt.where)
}

func (builder *defaultSQLBuilder) WriteUpdateStatement(stmt *SQLUpdateStatement) {
	builder.Write("UPDATE ")
	builder.Write(stmt.table)
	builder.WriteLine()
	builder.Write("SET ")

	for i, expr := range stmt.updateList {
		if i > 0 {
			builder.Write(",")
		}

		binExpr := expr.(*SQLBinaryExpression)
		builder.WriteSQLExpression(binExpr.left)
		builder.Write(" = ")
		builder.WriteSQLExpression(binExpr.right)
	}
	builder.WriteLine()

	builder.WriteWhere(stmt.where)
}

func (builder *defaultSQLBuilder) WriteInsertStatement(stmt *SQLInsertStatement) {
	builder.Write("INSERT INTO ")
	builder.Write(stmt.table)
	builder.Write("(")

	for i, col := range stmt.columns {
		if i > 0 {
			builder.Write(",")
		}

		builder.Write(col)
	}

	builder.Write(")")
	builder.WriteLine()
	builder.Write("VALUES(")

	for i := range stmt.columns {
		if i > 0 {
			builder.Write(",")
		}

		builder.Write("?")
	}

	builder.Write(")")
}

func (builder *defaultSQLBuilder) WriteSelectStatement(stmt *SQLSelectStatement) {
	builder.Write("SELECT ")

	for i, expr := range stmt.selectList {
		if i > 0 {
			builder.Write(", ")
		}

		builder.WriteSQLExpression(expr)
	}

	builder.WriteLine()

	if stmt.table != "" {
		builder.Write("FROM ")
		builder.Write(stmt.table)
		builder.WriteLine()
	}

	builder.WriteWhere(stmt.where)

	if stmt.orderByList != nil && len(stmt.orderByList) > 0 {
		builder.Write("ORDER BY ")

		for i, o := range stmt.orderByList {
			if i > 0 {
				builder.Write(",")
			}

			builder.WriteSQLExpression(o)
		}

		builder.WriteLine()
	}

	if stmt.limitRows > 0 {
		builder.Write("LIMIT ")
		builder.Write(strconv.Itoa(stmt.limitRows))
		builder.WriteLine()
	}
}

func (builder *defaultSQLBuilder) WriteSQLExpression(expr SQLExpression) {
	switch inst := expr.(type) {
	case *SQLLiteralExpression:
		v := strings.Replace(inst.value, `"`, `\"`, -1)
		builder.Write(v)

	case *SQLParenthesisExpression:
		builder.Write("(")
		builder.WriteSQLExpression(inst.target)
		builder.Write(")")

	case *SQLColumnExpression:
		/*if inst.tableName != "" {
			builder.Write(inst.tableName)
			builder.Write(".")
		}*/
		builder.Write(inst.columnName)

	case *SQLParameterExpression:
		builder.Write("?")

	case *SQLBinaryExpression:
		builder.WriteSQLExpression(inst.left)
		builder.Write(" ")
		switch inst.op {
		case "!=":
			builder.Write("<>")
		case "&&":
			builder.Write("AND")
		case "||":
			builder.Write("OR")
		case "==":
			builder.Write("=")
		default:
			builder.Write(inst.op)
		}

		builder.Write(" ")
		builder.WriteSQLExpression(inst.right)

	case *SQLOrderExpression:
		builder.WriteSQLExpression(inst.column)

		if inst.isDescending {
			builder.Write(" DESC")
		}
	}
}

func (builder *defaultSQLBuilder) GetInvokeParameterList(paramList []*SQLParameterExpression) []*SQLParameterExpression {
	return paramList
}

func getSqlParamListFromExpression(expr SQLExpression) []*SQLParameterExpression {
	list := make([]*SQLParameterExpression, 0)

	switch inst := expr.(type) {
	case *SQLBinaryExpression:
		list = append(list, getSqlParamListFromExpression(inst.left)...)
		list = append(list, getSqlParamListFromExpression(inst.right)...)

	case *SQLParameterExpression:
		list = append(list, inst)
	}

	return list
}

func getSelectStmtSqlParamList(stmt *SQLSelectStatement) []*SQLParameterExpression {
	list := make([]*SQLParameterExpression, 0)

	list = append(list, getSqlParamListFromExpression(stmt.where)...)

	return list
}

func getUpdateStmtSqlParamList(stmt *SQLUpdateStatement) []*SQLParameterExpression {
	list := make([]*SQLParameterExpression, 0)

	for _, updateExpr := range stmt.updateList {
		inst := updateExpr.(*SQLBinaryExpression)

		list = append(list, getSqlParamListFromExpression(inst.right)...)
	}

	list = append(list, getSqlParamListFromExpression(stmt.where)...)

	return list
}

func getDeleteStmtSqlParamList(stmt *SQLDeleteStatement) []*SQLParameterExpression {
	return getSqlParamListFromExpression(stmt.where)
}

func tableToSelectStatement(builder SQLBuilder, stmt *SQLSelectStatement, table *table) *SQLSelectStatement {
	if stmt == nil {
		stmt = &SQLSelectStatement{}
	}

	stmt.table = table.tableName

	for _, col := range table.columns {
		colExpr := &SQLColumnExpression{}
		colExpr.columnName = col.columnName
		colExpr.source = col
		colExpr.tableName = stmt.table

		stmt.selectList = append(stmt.selectList, colExpr)
	}

	return stmt
}

func tableToInsertStatement(builder SQLBuilder, stmt *SQLInsertStatement, table *table) *SQLInsertStatement {
	if stmt == nil {
		stmt = &SQLInsertStatement{}
	}

	stmt.table = table.tableName

	for _, col := range table.columns {
		if col.isIdentity {
			continue
		}

		stmt.columns = append(stmt.columns, col.columnName)
	}

	return stmt
}
