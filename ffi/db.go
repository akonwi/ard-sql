// Package ffi bridges Ard lists and row values to database/sql while Ard
// retains native *sql.DB and *sql.Tx handles. Pure-Go drivers are registered
// here so compiled binaries can remain CGO-free.
package ffi

import (
	"database/sql"
	"reflect"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// ExecDBHandle and QueryDBHandle bridge Ard lists to database/sql's
// variadic argument API while allowing Ard to retain the native *sql.DB handle.
func ExecDBHandle(db *sql.DB, query string, args []any) error {
	_, err := db.Exec(query, bindArgs(args)...)
	return err
}

func QueryDBHandle(db *sql.DB, query string, args []any) ([]any, error) {
	return scanRows(db.Query(query, bindArgs(args)...))
}

// ExecTxHandle and QueryTxHandle are the transaction equivalents of the
// native database handle bridges above.
func ExecTxHandle(tx *sql.Tx, query string, args []any) error {
	_, err := tx.Exec(query, bindArgs(args)...)
	return err
}

func QueryTxHandle(tx *sql.Tx, query string, args []any) ([]any, error) {
	return scanRows(tx.Query(query, bindArgs(args)...))
}

// bindArgs maps Ard values into driver-friendly bind parameters.
// Notably it unwraps Ard's Maybe[T] optionals: a none becomes SQL NULL,
// a some becomes its inner value. Detection is by method shape
// (IsNone() bool + Value() T on a struct named Maybe[...]) since the
// runtime type lives in each generated binary, not in a shared package.
func bindArgs(args []any) []any {
	out := make([]any, len(args))
	for i, a := range args {
		out[i] = bindArg(a)
	}
	return out
}

func bindArg(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Struct || !strings.HasPrefix(rv.Type().Name(), "Maybe[") {
		return v
	}
	isNone := rv.MethodByName("IsNone")
	value := rv.MethodByName("Value")
	if !isNone.IsValid() || !value.IsValid() {
		return v
	}
	if isNone.Call(nil)[0].Bool() {
		return nil
	}
	return value.Call(nil)[0].Interface()
}

// scanRows collects every row as an `any` holding a column->value map.
// The outer slice type is []any (not []map[string]any) so it surfaces on
// the Ard side as `[Any]` — each row is an opaque Any that the caller
// can decode into typed values. []byte columns are converted
// to strings so they arrive as Ard Str.
func scanRows(rows *sql.Rows, err error) ([]any, error) {
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []any
	for rows.Next() {
		values := make([]any, len(cols))
		pointers := make([]any, len(cols))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalize(values[i])
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// normalize converts values scanned from database/sql into shapes the Ard
// side sees as its native scalar types. Without this, integer columns come
// through as int64 (Ard: Int64, a sized type) rather than the default Int,
// forcing every decoder to unsafe::cast<Int64> instead of the natural Int.
//
// time.Time is formatted as RFC3339Nano so that TIMESTAMP / TIMESTAMPTZ /
// DATETIME columns (Postgres, MySQL) arrive as Ard Str the same way SQLite
// dates already do. Callers write times back as RFC3339 strings, which
// database/sql converts on bind, so writes are symmetric without any FFI
// help. nil is preserved so that decode::nullable can detect SQL NULLs.
func normalize(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	case int64:
		return int(x)
	case int32:
		return int(x)
	case float32:
		return float64(x)
	case time.Time:
		return x.Format(time.RFC3339Nano)
	default:
		return v
	}
}
