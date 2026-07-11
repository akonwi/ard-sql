package ffi

import (
	"testing"
	"time"
)

// normalize sits between database/sql's driver-specific Go values and the
// Ard side. These tests lock in the conversion contract so that:
//
//   - Ard receives its native `Int` / `Str` / `Float64` for numeric and
//     text columns regardless of driver.
//   - SQL NULLs survive as nil so `decode::nullable` can detect them.
//   - Postgres / MySQL TIMESTAMP columns arrive as RFC3339Nano strings
//     (SQLite already stores dates as TEXT, so this makes all three
//     dialects behave symmetrically from Ard's point of view).
func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want any
	}{
		{"bytes to string", []byte("hello"), "hello"},
		{"int64 to int", int64(42), int(42)},
		{"int32 to int", int32(7), int(7)},
		{"float32 to float64", float32(1.5), float64(1.5)},
		{"string passthrough", "already a string", "already a string"},
		{"int passthrough", int(3), int(3)},
		{"float64 passthrough", float64(2.5), float64(2.5)},
		{"bool passthrough", true, true},
		{"nil preserved for null detection", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalize(tt.in)
			if got != tt.want {
				t.Fatalf("normalize(%v) = %v (%T), want %v (%T)",
					tt.in, got, got, tt.want, tt.want)
			}
		})
	}
}

// time.Time is the Gap 2 case from the FFI normalize follow-up: without
// this conversion, Postgres/MySQL TIMESTAMP columns arrive as an opaque
// time.Time struct that the Ard-side `decode` module can't unwrap. Format
// at the boundary so downstream code just sees a string.
func TestNormalize_TimeIsFormattedRFC3339Nano(t *testing.T) {
	when := time.Date(2025, 3, 14, 15, 9, 26, 535_000_000, time.UTC)

	got := normalize(when)

	want := "2025-03-14T15:09:26.535Z"
	if got != want {
		t.Fatalf("normalize(time.Time) = %q, want %q", got, want)
	}

	// And with sub-millisecond precision, so we know we're using Nano
	// rather than the second-precision RFC3339 constant.
	precise := time.Date(2025, 3, 14, 15, 9, 26, 123_456_789, time.UTC)
	if got := normalize(precise); got != "2025-03-14T15:09:26.123456789Z" {
		t.Fatalf("nanosecond precision lost: got %q", got)
	}
}

// Mirror of the Ard runtime's Maybe[T] shape (unexported pointer field +
// IsNone/Value methods). The real type lives inside each generated
// binary, so bindArg detects it structurally.
type Maybe[T any] struct {
	value *T
}

func (m Maybe[T]) IsNone() bool { return m.value == nil }
func (m Maybe[T]) Value() T {
	if m.value == nil {
		var zero T
		return zero
	}
	return *m.value
}

// bindArg unwraps Ard Maybe values into value-or-nil so nullable columns
// can be written through named params (found syncing nullable scores in
// maestro's fixtures table).
func TestBindArg_UnwrapsMaybe(t *testing.T) {
	seven := 7
	some := Maybe[int]{value: &seven}
	none := Maybe[int]{}

	if got := bindArg(some); got != 7 {
		t.Fatalf("bindArg(some(7)) = %v, want 7", got)
	}
	if got := bindArg(none); got != nil {
		t.Fatalf("bindArg(none) = %v, want nil", got)
	}
	// Non-Maybe values pass through untouched.
	if got := bindArg(42); got != 42 {
		t.Fatalf("bindArg(42) = %v, want 42", got)
	}
	if got := bindArg("hi"); got != "hi" {
		t.Fatalf("bindArg(string) = %v, want unchanged", got)
	}
	if got := bindArg(nil); got != nil {
		t.Fatalf("bindArg(nil) = %v, want nil", got)
	}
}

// End-to-end through a real SQLite connection: write a none and a some
// into a nullable column via ExecDB, read them back.
func TestBindArgs_NullableRoundTrip(t *testing.T) {
	db, err := Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer Close(db)

	if err := ExecDB(db, "CREATE TABLE scores (id INTEGER PRIMARY KEY, points INTEGER)", nil); err != nil {
		t.Fatal(err)
	}
	three := 3
	if err := ExecDB(db, "INSERT INTO scores (id, points) VALUES (?, ?)", []any{1, Maybe[int]{value: &three}}); err != nil {
		t.Fatalf("insert some: %v", err)
	}
	if err := ExecDB(db, "INSERT INTO scores (id, points) VALUES (?, ?)", []any{2, Maybe[int]{}}); err != nil {
		t.Fatalf("insert none: %v", err)
	}

	rows, err := QueryDB(db, "SELECT points FROM scores ORDER BY id", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	first := rows[0].(map[string]any)
	if first["points"] != 3 {
		t.Fatalf("some round-trip: got %v", first["points"])
	}
	second := rows[1].(map[string]any)
	if second["points"] != nil {
		t.Fatalf("none round-trip: got %v, want nil", second["points"])
	}
}
