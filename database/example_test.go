package database

import (
	"context"
	"database/sql"
	"fmt"
	"io"
)

type DBTestHelp struct {
	db *sql.DB
}

func NewDBTestHelp(db *sql.DB) *DBTestHelp {
	return &DBTestHelp{
		db: db,
	}
}

// QueryMuch 查询多项行数据 通过w打印结果
func (d *DBTestHelp) QueryMuch(ctx context.Context, query string, w io.Writer) error {

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("queryContext fail. error: %v", err)
	}
	defer rows.Close()

	var columns []string
	columns, err = rows.Columns()
	if err != nil {
		return fmt.Errorf("columns fail. error: %v", err)
	}

	data := make([]interface{}, len(columns))
	dataStr := make([]interface{}, len(columns))

	for i := 0; i < len(columns); i++ {
		dataStr[i] = &data[i]
	}

	for rows.Next() {
		if err = rows.Scan(dataStr...); err != nil {
			return fmt.Errorf("scan fail. error: %v", err)
		}
		if _, err = fmt.Fprintln(w, data); err != nil {
			return fmt.Errorf("fprintln fail. error: %v", err)
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("rows has error. error: %v", err)
	}
	return nil
}

// QueryOne 查询一行数据 通过dest返回结果
func (d *DBTestHelp) QueryOne(ctx context.Context, query string, dest ...interface{}) error {
	if err := d.db.QueryRowContext(ctx, query).Scan(dest); err != nil {
		return fmt.Errorf("queryrow scan fail. error: %v", err)
	}
	return nil
}

// Exec 执行一个query语句 args为参数
func (d *DBTestHelp) Exec(ctx context.Context, query string, args ...interface{}) error {
	if _, err := d.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("execContext scan fail. error: %v", err)
	}
	return nil
}
