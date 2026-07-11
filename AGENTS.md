# AGENTS.md

## Project

`sql` is an Ard wrapper around Go's `database/sql`, with pure-Go drivers for PostgreSQL, SQLite, and MySQL.

## Layout

- `sql.ard`: public Ard API
- `sql_test.ard`: Ard integration tests using SQLite
- `ffi/db.go`: Go bridge to `database/sql`
- `ffi/db_test.go`: Go-side value normalization tests
- `ard.toml`: Ard package metadata
- `go.mod`: Go driver dependencies

## Development

Run before committing:

```sh
ard format .
ard test
go test ./...
```

Keep driver-specific behavior behind the package API. Preserve SQL NULL values and normalize scanned values into Ard-compatible scalar types. Add Ard and Go tests for behavior changes at the FFI boundary.
