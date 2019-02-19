package sqlcodegen

type TableName string

type ReturnType uint

const (
	ReturnDefault ReturnType = iota
	ReturnExecResult
	ReturnScalar
	ReturnScalarSet
	ReturnRecord
	ReturnRecordSet
	ReturnRecordChannel
)

func From(table interface{}) {}

func Select(columns ...interface{}) {}

func SelectAll(table interface{}) {}

func Where(condition bool) {}

func InsertAll(table interface{}) {}

func Update(column interface{}, value interface{}) {}

func Delete(table interface{}) {}

func OrderBy(column interface{}) {}

func OrderByDescending(column interface{}) {}

func SetReturnType(t ReturnType) {}

func ExecProcedure(procName string, args ...interface{}) {}

func SetPackageName(packageName string) {}

func SetChannelBufferSize(size int) {}
