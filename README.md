# Ballo ORM

## Purpose
Ballo ORM provides a lightweight, reflection‑based Object‑Relational Mapping layer for Go. It works with any `database/sql` driver that implements the standard `*sql.DB` or `*sql.Tx` interfaces. The package focuses on simple CRUD operations and a fluent query builder without heavy abstractions.

## Go version & dependencies
- Go 1.23 (as declared in `go.mod`).
- Requires a database driver; the test suite uses `modernc.org/sqlite` for an in‑memory SQLite database.
- Indirect dependencies are managed automatically by Go modules.

## Installation
```bash
go get github.com/lukman-ss/ballo-orm
```

## Quick start
```go
package main

import (
    "context"
    "database/sql"
    "fmt"

    "github.com/lukman-ss/ballo-orm"
    _ "modernc.org/sqlite"
)

type User struct {
    ID    int    `db:"id,primary_key"`
    Name  string `db:"name"`
    Email string `db:"email"`
    Age   *int   `db:"age"`
}

func main() {
    // Open SQLite in‑memory database
    db, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
    if err != nil { panic(err) }
    defer db.Close()

    // Create table used by the tests (simplified example)
    _, err = db.Exec(`
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            email TEXT UNIQUE NOT NULL,
            age INTEGER
        )`)
    if err != nil { panic(err) }

    // Initialise ORM
    o := orm.New(db)

    // Insert a record
    u := User{Name: "John Doe", Email: "john@example.com", Age: intPtr(30)}
    if err := o.Insert(context.Background(), &u); err != nil { panic(err) }
    fmt.Printf("Inserted ID=%d\n", u.ID)

    // Find by primary key
    var fetched User
    if err := o.FindByID(context.Background(), u.ID, &fetched); err != nil { panic(err) }
    fmt.Printf("Fetched: %+v\n", fetched)

    // Query multiple rows with the fluent builder
    var users []User
    qb := orm.Table("users").OrderBy("id")
    if err := o.FindAll(context.Background(), qb, &users); err != nil { panic(err) }
    fmt.Printf("All users: %+v\n", users)
}

func intPtr(i int) *int { return &i }
```

## API Overview
### Core types
- `type ORM struct{ db DBExecutor }` – wraps a `*sql.DB` or `*sql.Tx`.
- `type QueryBuilder struct` – fluent builder for `SELECT` statements.
- `type DBExecutor interface` – subset of `*sql.DB`/`*sql.Tx` methods used by the ORM.

### Constructor
- `func New(db DBExecutor) *ORM` – creates an ORM instance.
- `func Table(name string) *QueryBuilder` – starts a query for the given table (validates identifier).

### QueryBuilder methods
- `Select(cols ...string) *QueryBuilder`
- `Where(cond string, args ...any) *QueryBuilder`
- `OrderBy(col string, dir ...string) *QueryBuilder`
- `Limit(n int) *QueryBuilder`
- `Offset(n int) *QueryBuilder`
- `ToSQL() (string, []any, error)` – builds the final SQL and arguments.

### ORM CRUD methods
- `Insert(ctx context.Context, model any) error`
- `FindByID(ctx context.Context, id any, dest any) error`
- `FindAll(ctx context.Context, q *QueryBuilder, destSlice any) error`
- `Update(ctx context.Context, model any) error`
- `Delete(ctx context.Context, model any) error`

## Running tests
```bash
go test ./...
```
All tests run against an in‑memory SQLite database and verify the core functionality.

## Limitations & compatibility
- Currently demonstrated with SQLite (`modernc.org/sqlite`). Other drivers work as long as they implement the standard `database/sql` interfaces.
- Primary key handling assumes a single integer primary key named via the `primary_key` tag or a field named `ID`.
- No built‑in support for complex relationships or migrations – users manage schema themselves.
- **Security**: `Where` accepts raw SQL strings. Caller must ensure values use placeholders to prevent injection. Raw identifiers are quoted when safe; invalid identifiers pass through unchanged.
- **Dialect**: QueryBuilder uses backticks for identifier quoting, compatible with MySQL and SQLite. Other dialects may require manual adjustments.
- **Placeholder counting**: Placeholders (`?`) inside string literals (single/double quoted), backtick-quoted identifiers, and SQL comments (`--`, `/* */`) are not counted. Escaped quotes (`''`, `""`, `` ` ``) and backslash escapes (`\`) inside quoted text are respected. Note: `requote` modifies conditions by quoting valid identifiers with backticks — the expression is not passed through unchanged.
- **Placeholder syntax**: Uses `?` positional placeholders (SQLite/MySQL style). PostgreSQL `$1`, `$2` syntax requires manual adjustment.
- **Dialect**: Placeholders use `?` syntax (SQLite/MySQL style). For PostgreSQL `$1`, `$2` syntax, manual adjustment required.

---
*Generated from the actual codebase; features not present in the source are omitted.*
