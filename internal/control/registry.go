package control

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

// Typed registry access: hand-written SQL scanned explicitly into structs.
// Each record type names its columns once (a const next to the type) and
// returns the matching scan destinations from fields(). Business records
// (entity data) stay dynamic.
//
// The structs reproduce the JSON that the untyped Row reads produced: fields
// are declared in JSON key order (a map marshals its keys sorted), a NULL
// column is a pointer (null) and a TINYINT flag keeps its numeric 0/1.

// record is a registry struct that can be scanned from its column list.
type record[T any] interface {
	*T
	fields() []any
}

// scanAll runs query and scans every row into a T.
func scanAll[T any, P record[T]](ctx context.Context, q Queryer, query string, args ...any) ([]T, error) {
	r, e := q.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	out := []T{}
	for r.Next() {
		var v T
		if e = r.Scan(P(&v).fields()...); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, r.Err()
}

// scanOne runs query and scans its first row; no row is missing() (404).
func scanOne[T any, P record[T]](ctx context.Context, q Queryer, query string, args ...any) (T, error) {
	var v T
	e := q.QueryRowContext(ctx, query, args...).Scan(P(&v).fields()...)
	if errors.Is(e, sql.ErrNoRows) {
		return v, missing()
	}
	return v, e
}

// bit is a TINYINT 0/1 column: a bool in Go, a number in JSON.
type bit bool

func (f *bit) Scan(src any) error {
	var n sql.NullInt64
	if e := n.Scan(src); e != nil {
		return e
	}
	*f = n.Int64 != 0
	return nil
}

func (f bit) MarshalJSON() ([]byte, error) {
	if f {
		return []byte("1"), nil
	}
	return []byte("0"), nil
}

// scanStrings runs a one-column query and returns its values.
func scanStrings(ctx context.Context, q Queryer, query string, args ...any) ([]string, error) {
	r, e := q.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	out := []string{}
	for r.Next() {
		var v string
		if e = r.Scan(&v); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, r.Err()
}

// jsonValue is a *_json column. It marshals as the untyped reads returned it:
// the decoded document, the raw text when it does not parse, or null.
type jsonValue struct{ v any }

func (j *jsonValue) Scan(src any) error {
	*j = jsonValue{}
	var raw []byte
	switch t := src.(type) {
	case nil:
		return nil
	case []byte:
		raw = t
	case string:
		raw = []byte(t)
	default:
		return errors.New("unsupported JSON column type")
	}
	if json.Unmarshal(raw, &j.v) != nil {
		j.v = string(raw)
	}
	return nil
}

func (j jsonValue) MarshalJSON() ([]byte, error) { return json.Marshal(j.v) }

// value is the decoded document, as the untyped reads returned it.
func (j jsonValue) value() any { return j.v }

// lookup scans one row of query into dest; no row is missing() (404).
func lookup(ctx context.Context, q Queryer, query string, args []any, dest ...any) error {
	e := q.QueryRowContext(ctx, query, args...).Scan(dest...)
	if errors.Is(e, sql.ErrNoRows) {
		return missing()
	}
	return e
}

// prefixed qualifies each column of a comma-separated list with alias.
func prefixed(alias, columns string) string {
	return alias + "." + strings.ReplaceAll(columns, ",", ","+alias+".")
}
