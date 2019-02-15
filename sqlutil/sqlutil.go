package sqlutil

import (
	"context"
	"database/sql"
)

type DbObject interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

type DataReadFunction func(*sql.Rows) interface{}

type DataChannel struct {
	rows         *sql.Rows
	cancel       context.CancelFunc
	ctx          context.Context
	readFunction DataReadFunction
	channel      chan interface{}
}

func newDataChannel(rows *sql.Rows, readFunction DataReadFunction) *DataChannel {
	c := &DataChannel{
		rows:         rows,
		readFunction: readFunction,
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.channel = make(chan interface{})

	go func(c *DataChannel) {
		defer close(c.channel)
		defer c.rows.Close()

		for c.rows.Next() {
			data := c.readFunction(c.rows)

			select {
			case <-c.ctx.Done():
				return
			case c.channel <- data:
			}
		}
	}(c)

	return c
}

func (c *DataChannel) Get() <-chan interface{} {
	return c.channel
}

func (c *DataChannel) Close() {
	c.cancel()
}

func QueryChannel(e DbObject, query string, readFunc DataReadFunction, args ...interface{}) (*DataChannel, error) {
	rows, err := e.QueryContext(context.Background(), query, args...)

	if err != nil {
		return nil, err
	}

	return newDataChannel(rows, readFunc), nil
}

func QueryRecord(e DbObject, query string, readFunc DataReadFunction, args ...interface{}) (interface{}, error) {
	rows, err := e.QueryContext(context.Background(), query, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var result interface{}

	if rows.Next() {
		result = readFunc(rows)
	}

	return result, nil
}

func QueryRecordSet(e DbObject, query string, readFunc DataReadFunction, args ...interface{}) ([]interface{}, error) {
	rows, err := e.QueryContext(context.Background(), query, args...)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	result := make([]interface{}, 0)

	for rows.Next() {
		result = append(result, readFunc(rows))
	}

	return result, nil
}
