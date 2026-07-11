# sql

A small Ard wrapper around Go's [`database/sql`](https://pkg.go.dev/database/sql) package.

It provides a consistent API for PostgreSQL, SQLite, and MySQL using pure-Go drivers, including named parameters, transactions, and query results represented as opaque Ard `Any` values.

## Requirements

- Ard 0.26.0 or newer
- Go 1.26 or newer

## Installation

```sh
ard add <repository-url>@<tag-or-commit> as sql
```

Then import the module:

```ard
use sql/sql
```

For local development:

```toml
[dependencies]
sql = { path = "../sql" }
```

## Usage

```ard
use sql/sql

fn users(database_url: Str) [Any]!Str {
  let db = try sql::open(database_url)
  let rows = try db
    .query("SELECT id, name FROM users WHERE active = @active")
    .all(["active": true])
  try db.close()
  Result::ok(rows)
}
```

Queries use `@name` parameters. They are rewritten to the placeholder syntax required by the selected database driver.

Transactions expose the same `exec` and `query` operations:

```ard
let tx = try db.begin()
try tx.query("DELETE FROM users WHERE id = @id").run(["id": 42])
try tx.commit()
```

## Database URLs

The driver is inferred from the connection string:

- PostgreSQL: `postgres://...` or `postgresql://...`
- MySQL: DSNs containing `@tcp(` or `@unix(`
- SQLite: all other values, such as `:memory:` or `app.db`

## API

- `open` opens a database connection
- `Database.close` closes it
- `Database.exec` executes SQL without parameters
- `Database.query` creates a parameterized query
- `Database.begin` starts a transaction
- `Query.run` executes a statement
- `Query.all` returns every matching row
- `Query.first` returns the first row, if present
- `Transaction.commit` and `Transaction.rollback` complete a transaction

## Development

```sh
ard format .
ard test
go test ./...
```
