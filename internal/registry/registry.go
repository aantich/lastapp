// Package registry is the one MariaDB registry adapter: the Queryer, typed
// records scanned from hand-written SQL, shared policy, quota and lock queries,
// and the APIError every layer returns.
package registry

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

// Record is a registry struct that can be scanned from its column list.
type Record[T any] interface {
	*T
	ScanFields() []any
}

// ScanAll runs query and scans every row into a T.
func ScanAll[T any, P Record[T]](ctx context.Context, q Queryer, query string, args ...any) ([]T, error) {
	r, e := q.QueryContext(ctx, query, args...)
	if e != nil {
		return nil, e
	}
	defer r.Close()
	out := []T{}
	for r.Next() {
		var v T
		if e = r.Scan(P(&v).ScanFields()...); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, r.Err()
}

// ScanOne runs query and scans its first row; no row is missing() (404).
func ScanOne[T any, P Record[T]](ctx context.Context, q Queryer, query string, args ...any) (T, error) {
	var v T
	e := q.QueryRowContext(ctx, query, args...).Scan(P(&v).ScanFields()...)
	if errors.Is(e, sql.ErrNoRows) {
		return v, Missing()
	}
	return v, e
}

// Bit is a TINYINT 0/1 column: a bool in Go, a number in JSON.
type Bit bool

func (f *Bit) Scan(src any) error {
	var n sql.NullInt64
	if e := n.Scan(src); e != nil {
		return e
	}
	*f = n.Int64 != 0
	return nil
}

func (f Bit) MarshalJSON() ([]byte, error) {
	if f {
		return []byte("1"), nil
	}
	return []byte("0"), nil
}

// ScanStrings runs a one-column query and returns its values.
func ScanStrings(ctx context.Context, q Queryer, query string, args ...any) ([]string, error) {
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

// JSONValue is a *_json column. It marshals as the untyped reads returned it:
// the decoded document, the raw text when it does not parse, or null.
type JSONValue struct{ v any }

func (j *JSONValue) Scan(src any) error {
	*j = JSONValue{}
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

func (j JSONValue) MarshalJSON() ([]byte, error) { return json.Marshal(j.v) }

// Value is the decoded document, as the untyped reads returned it.
func (j JSONValue) Value() any { return j.v }

// Lookup scans one row of query into dest; no row is missing() (404).
func Lookup(ctx context.Context, q Queryer, query string, args []any, dest ...any) error {
	e := q.QueryRowContext(ctx, query, args...).Scan(dest...)
	if errors.Is(e, sql.ErrNoRows) {
		return Missing()
	}
	return e
}

// Prefixed qualifies each column of a comma-separated list with alias.
func Prefixed(alias, columns string) string {
	return alias + "." + strings.ReplaceAll(columns, ",", ","+alias+".")
}
