package orm

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func setupSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			email TEXT UNIQUE NOT NULL,
			age INTEGER
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
}

type User struct {
	ID    int    `db:"id,primary_key"`
	Name  string `db:"name"`
	Email string `db:"email"`
	Age   *int   `db:"age"`
}

func TestQueryBuilder(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *QueryBuilder
		expected string
	}{
		{
			name: "simple select",
			build: func() *QueryBuilder {
				return Table("users").Select("id", "name")
			},
			expected: "SELECT `id`, `name` FROM `users`",
		},
		{
			name: "select all",
			build: func() *QueryBuilder {
				return Table("users")
			},
			expected: "SELECT * FROM `users`",
		},
		{
			name: "with where",
			build: func() *QueryBuilder {
				return Table("users").Where("id = ?", 1)
			},
			expected: "SELECT * FROM `users` WHERE `id` = ?",
		},
		{
			name: "with where args",
			build: func() *QueryBuilder {
				q := Table("users").Where("name = ? AND age > ?", "John", 18)
				_, args, _ := q.ToSQL()
				if len(args) != 2 || args[0] != "John" || args[1] != 18 {
					t.Errorf("wrong args: %v", args)
				}
				return q
			},
			expected: "SELECT * FROM `users` WHERE `name` = ? AND `age` > ?",
		},
		{
			name: "with order by",
			build: func() *QueryBuilder {
				return Table("users").OrderBy("name", "DESC")
			},
			expected: "SELECT * FROM `users` ORDER BY `name` DESC",
		},
		{
			name: "with limit",
			build: func() *QueryBuilder {
				return Table("users").Limit(10)
			},
			expected: "SELECT * FROM `users` LIMIT 10",
		},
		{
			name: "with offset",
			build: func() *QueryBuilder {
				return Table("users").Offset(20)
			},
			expected: "SELECT * FROM `users` OFFSET 20",
		},
		{
			name: "full query",
			build: func() *QueryBuilder {
				return Table("users").Select("id", "name").Where("id > ?", 1).OrderBy("id").Limit(5).Offset(10)
			},
			expected: "SELECT `id`, `name` FROM `users` WHERE `id` > ? ORDER BY `id` ASC LIMIT 5 OFFSET 10",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := tt.build()
			query, _, err := q.ToSQL()
			if err != nil {
				t.Fatalf("ToSQL() error: %v", err)
			}
			if query != tt.expected {
				t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", tt.expected, query)
			}
		})
	}
}

func TestORMInsert(t *testing.T) {
	db := testDB(t)
	setupSchema(t, db)
	o := New(db)

	user := User{Name: "John Doe", Email: "john@example.com", Age: intPtr(25)}
	err := o.Insert(context.Background(), &user)
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}
	// Query back to verify insert succeeded
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users WHERE id = ?", user.ID).Scan(&count)
	if err != nil || count != 1 {
		t.Errorf("user not inserted correctly, ID=%d, count=%d", user.ID, count)
	}
}

func TestORMFindByID(t *testing.T) {
	db := testDB(t)
	setupSchema(t, db)
	o := New(db)

	user := User{Name: "Jane Doe", Email: "jane@example.com", Age: intPtr(30)}
	err := o.Insert(context.Background(), &user)
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	var found User
	err = o.FindByID(context.Background(), user.ID, &found)
	if err != nil {
		t.Fatalf("FindByID() error: %v", err)
	}
	if found.ID != user.ID || found.Name != user.Name {
		t.Errorf("user mismatch: got %+v, want %+v", found, user)
	}
}

func TestORMFindAll(t *testing.T) {
	db := testDB(t)
	setupSchema(t, db)
	o := New(db)

	for i := 0; i < 5; i++ {
		o.Insert(context.Background(), &User{
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   intPtr(20 + i),
		})
	}

	var users []User
	q := Table("users").OrderBy("id")
	err := o.FindAll(context.Background(), q, &users)
	if err != nil {
		t.Fatalf("FindAll() error: %v", err)
	}
	if len(users) != 5 {
		t.Errorf("expected 5 users, got %d", len(users))
	}
}

func TestORMUpdate(t *testing.T) {
	db := testDB(t)
	setupSchema(t, db)
	o := New(db)

	user := User{Name: "Update Test", Email: "update@example.com", Age: intPtr(25)}
	err := o.Insert(context.Background(), &user)
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	user.Name = "Updated Name"
	err = o.Update(context.Background(), &user)
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	var updated User
	err = o.FindByID(context.Background(), user.ID, &updated)
	if err != nil {
		t.Fatalf("FindByID() error: %v", err)
	}
	if updated.Name != "Updated Name" {
		t.Errorf("name not updated: got %s", updated.Name)
	}
}

func TestORMDelete(t *testing.T) {
	db := testDB(t)
	setupSchema(t, db)
	o := New(db)

	user := User{Name: "Delete Test", Email: "delete@example.com", Age: intPtr(25)}
	err := o.Insert(context.Background(), &user)
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	err = o.Delete(context.Background(), &user)
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow() error: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 users, got %d", count)
	}
}

func TestORMTransaction(t *testing.T) {
	db := testDB(t)
	setupSchema(t, db)

	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Begin() error: %v", err)
	}
	defer tx.Rollback()

	tORM := New(tx)

	user := User{Name: "Tx Test", Email: "tx@example.com", Age: intPtr(25)}
	err = tORM.Insert(context.Background(), &user)
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("QueryRow() error: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 user, got %d", count)
	}
}

func TestModelPrimaryKey(t *testing.T) {
	user := User{Name: "Test", Email: "test@example.com"}
	pk, found := GetPrimaryKey(user)
	if !found {
		t.Error("primary key should be found")
	}
	if pk != "id" {
		t.Errorf("expected pk='id', got '%s'", pk)
	}
}

func TestModelColumns(t *testing.T) {
	user := User{Name: "Test", Email: "test@example.com"}
	cols := GetColumns(user)
	expected := []string{"id", "name", "email", "age"}
	if len(cols) != len(expected) {
		t.Errorf("expected %d cols, got %d: %v", len(expected), len(cols), cols)
	}
}

func intPtr(i int) *int { return &i }

// TestTableConstructorValidation tests that Table() validates identifiers
func TestTableConstructorValidation(t *testing.T) {
	tests := []struct {
		name    string
		table   string
		wantErr bool
	}{
		{"valid simple name", "users", false},
		{"valid underscore", "user_list", false},
		{"valid digit", "user1", false},
		{"invalid dash", "user-list", true},
		{"invalid space", "user list", true},
		{"invalid special char", "user@list", true},
		{"invalid start digit", "123users", true},
		{"empty string", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Table(tt.table)
			_, _, err := q.ToSQL()
			if tt.wantErr {
				if err == nil {
					t.Errorf("Table(%q) expected error, got nil", tt.table)
				}
			} else {
				if err != nil {
					t.Errorf("Table(%q) unexpected error: %v", tt.table, err)
				}
			}
		})
	}
}

// TestTableMethodValidation tests that QueryBuilder.Table() validates identifiers
func TestTableMethodValidation(t *testing.T) {
	q := &QueryBuilder{}

	// Valid
	q = q.Table("users")
	if q.err != nil {
		t.Errorf("Table('users') should not error: %v", q.err)
	}

	// Invalid
	q = q.Table("bad-table")
	if q.err == nil {
		t.Error("Table('bad-table') expected error, got nil")
	}
}

// TestSelectValidation tests Select() identifier validation
func TestSelectValidation(t *testing.T) {
	q := Table("users")

	// Valid
	q = q.Select("id", "name")
	if q.err != nil {
		t.Errorf("Select valid cols expected no error: %v", q.err)
	}

	// Invalid
	q = Table("users").Select("bad col")
	if q.err == nil {
		t.Error("Select with invalid col expected error, got nil")
	}
}

// TestOrderByValidation tests OrderBy() identifier validation
func TestOrderByValidation(t *testing.T) {
	q := Table("users")

	// Valid
	q = q.OrderBy("name")
	if q.err != nil {
		t.Errorf("OrderBy valid col expected no error: %v", q.err)
	}

	// Invalid
	q = Table("users").OrderBy("bad-col")
	if q.err == nil {
		t.Error("OrderBy with invalid col expected error, got nil")
	}
}

// TestToSQLNoQueryOnError verifies ToSQL returns error when builder has invalid state
func TestToSQLNoQueryOnError(t *testing.T) {
	// Test invalid table via constructor
	q1 := Table("invalid!table")
	query, args, err := q1.ToSQL()
	if err == nil {
		t.Error("ToSQL with invalid table should error")
	}
	if query != "" {
		t.Errorf("ToSQL should return empty query on error, got: %s", query)
	}
	if args != nil {
		t.Errorf("ToSQL should return nil args on error, got: %v", args)
	}

	// Test invalid table via method
	q2 := &QueryBuilder{}
	q2.Table("invalid!table")
	query, args, err = q2.ToSQL()
	if err == nil {
		t.Error("ToSQL with invalid table should error")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

// TestWherePlaceholderPreservation verifies that Where arguments are preserved
// and returned separately from the SQL query (placeholders remain "?").
func TestWherePlaceholderPreservation(t *testing.T) {
	q := Table("users").Where("name = ? AND age > ?", "John", 18)
	query, args, err := q.ToSQL()
	if err != nil {
		t.Fatalf("ToSQL() error: %v", err)
	}
	// Placeholders must remain in SQL
	if query != "SELECT * FROM `users` WHERE `name` = ? AND `age` > ?" {
		t.Errorf("unexpected query: %s", query)
	}
	// Arguments must be returned separately
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "John" {
		t.Errorf("expected arg[0]=John, got %v", args[0])
	}
	if args[1] != 18 {
		t.Errorf("expected arg[1]=18, got %v", args[1])
	}
}

// TestWhereStringLiteralPreservation verifies that string literals inside
// WHERE conditions are not modified by requote.
func TestWhereStringLiteralPreservation(t *testing.T) {
	tests := []struct {
		name     string
		cond     string
		expected string
	}{
		{
			name:     "single quoted literal",
			cond:     "name = 'John'",
			expected: "SELECT * FROM `users` WHERE `name` = 'John'",
		},
		{
			name:     "single quoted with escaped quote",
			cond:     "name = 'O''Brien'",
			expected: "SELECT * FROM `users` WHERE `name` = 'O''Brien'",
		},
		{
			name:     "double quoted literal",
			cond:     "name = \"John\"",
			expected: "SELECT * FROM `users` WHERE `name` = \"John\"",
		},
		{
			name:     "multiple literals",
			cond:     "name = 'Alice' AND status = 'active'",
			expected: "SELECT * FROM `users` WHERE `name` = 'Alice' AND `status` = 'active'",
		},
		{
			name:     "literal with backslash escape",
			cond:     "path = 'C:\\Users\\test'",
			expected: "SELECT * FROM `users` WHERE `path` = 'C:\\Users\\test'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Table("users").Where(tt.cond)
			query, _, err := q.ToSQL()
			if err != nil {
				t.Fatalf("ToSQL() error: %v", err)
			}
			want := tt.expected
			if query != want {
				t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", want, query)
			}
		})
	}
}

// TestWhereMultipleOperators verifies conditions with various SQL operators.
func TestWhereMultipleOperators(t *testing.T) {
	tests := []struct {
		name     string
		cond     string
		args     []any
		expected string
	}{
		{
			name:     "comparison operators",
			cond:     "id > ? AND id < ? AND age >= ? AND age <= ?",
			args:     []any{1, 100, 0, 150},
			expected: "SELECT * FROM `users` WHERE `id` > ? AND `id` < ? AND `age` >= ? AND `age` <= ?",
		},
		{
			name:     "null check",
			cond:     "email IS NULL",
			args:     nil,
			expected: "SELECT * FROM `users` WHERE `email` IS NULL",
		},
		{
			name:     "not null check",
			cond:     "email IS NOT NULL",
			args:     nil,
			expected: "SELECT * FROM `users` WHERE `email` IS NOT NULL",
		},
		{
			name:     "in operator",
			cond:     "id IN (?, ?, ?)",
			args:     []any{1, 2, 3},
			expected: "SELECT * FROM `users` WHERE `id` IN (?, ?, ?)",
		},
		{
			name:     "between operator",
			cond:     "age BETWEEN ? AND ?",
			args:     []any{18, 65},
			expected: "SELECT * FROM `users` WHERE `age` BETWEEN ? AND ?",
		},
		{
			name:     "like operator",
			cond:     "name LIKE ?",
			args:     []any{"%john%"},
			expected: "SELECT * FROM `users` WHERE `name` LIKE ?",
		},
		{
			name:     "or condition",
			cond:     "role = ? OR status = ?",
			args:     []any{"admin", "active"},
			expected: "SELECT * FROM `users` WHERE `role` = ? OR `status` = ?",
		},
		{
			name:     "mixed conditions",
			cond:     "id = ? AND (status = ? OR role = ?)",
			args:     []any{1, "active", "admin"},
			expected: "SELECT * FROM `users` WHERE `id` = ? AND (`status` = ? OR `role` = ?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Table("users").Where(tt.cond, tt.args...)
			query, args, err := q.ToSQL()
			if err != nil {
				t.Fatalf("ToSQL() error: %v", err)
			}
			if query != tt.expected {
				t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", tt.expected, query)
			}
			if len(args) != len(tt.args) {
				t.Errorf("expected %d args, got %d", len(tt.args), len(args))
			}
		})
	}
}

// TestWhereBacktickPreservation verifies that backticks inside WHERE
// conditions are preserved (not stripped) by requote.
func TestWhereBacktickPreservation(t *testing.T) {
	q := Table("users").Where("`name` = 'test'")
	query, _, err := q.ToSQL()
	if err != nil {
		t.Fatalf("ToSQL() error: %v", err)
	}
	// backtick-quoted identifiers pass through as-is
	want := "SELECT * FROM `users` WHERE `name` = 'test'"
	if query != want {
		t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", want, query)
	}
}

// TestWhereInvalidIdentifierQuoting verifies that invalid identifiers in WHERE
// are left as raw SQL (not silently mangled into broken backtick-quoted output).
func TestWhereInvalidIdentifierQuoting(t *testing.T) {
	q := Table("users").Where("bad-col = ?", 1)
	query, _, err := q.ToSQL()
	if err != nil {
		t.Fatalf("ToSQL() error: %v", err)
	}
	// Invalid identifier must pass through unchanged — Where() accepts raw SQL.
	want := "SELECT * FROM `users` WHERE bad-col = ?"
	if query != want {
		t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", want, query)
	}
}

// TestLimitNegative verifies that negative LIMIT returns an error.
func TestLimitNegative(t *testing.T) {
	q := Table("users").Limit(-1)
	_, _, err := q.ToSQL()
	if err == nil {
		t.Error("LIMIT with negative value should error")
	}
	wantErr := "LIMIT must be non-negative, got -1"
	if err.Error() != wantErr {
		t.Errorf("expected error %q, got %q", wantErr, err.Error())
	}
}

// TestLimitZero verifies that zero LIMIT is allowed (no LIMIT clause added).
func TestLimitZero(t *testing.T) {
	q := Table("users").Limit(0)
	query, _, err := q.ToSQL()
	if err != nil {
		t.Fatalf("LIMIT(0) should be allowed: %v", err)
	}
	if strings.Contains(query, "LIMIT") {
		t.Errorf("LIMIT(0) should not add LIMIT clause, got: %s", query)
	}
}

// TestOffsetNegative verifies that negative OFFSET returns an error.
func TestOffsetNegative(t *testing.T) {
	q := Table("users").Offset(-5)
	_, _, err := q.ToSQL()
	if err == nil {
		t.Error("OFFSET with negative value should error")
	}
	wantErr := "OFFSET must be non-negative, got -5"
	if err.Error() != wantErr {
		t.Errorf("expected error %q, got %q", wantErr, err.Error())
	}
}

// TestOffsetZero verifies that zero OFFSET is allowed (no OFFSET clause added).
func TestOffsetZero(t *testing.T) {
	q := Table("users").Offset(0)
	query, _, err := q.ToSQL()
	if err != nil {
		t.Fatalf("OFFSET(0) should be allowed: %v", err)
	}
	if strings.Contains(query, "OFFSET") {
		t.Errorf("OFFSET(0) should not add OFFSET clause, got: %s", query)
	}
}

// TestWhereDangerousIdentifierInjection verifies dangerous identifiers are not auto-sanitized
// and raw WHERE expressions are NOT automatically safe from SQL injection.
func TestWhereDangerousIdentifierInjection(t *testing.T) {
	q := Table("users").Where("id = ?; DROP TABLE users; --", 1)
	query, args, err := q.ToSQL()
	if err != nil {
		t.Fatalf("ToSQL() error: %v", err)
	}
	// identifiers in raw WHERE are quoted by requote, raw expression parts are preserved
	// The ? placeholder is preserved; the dangerous SQL fragment is passed through
	want := "SELECT * FROM `users` WHERE `id` = ?; `DROP` `TABLE` `users`; --"
	if query != want {
		t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", want, query)
	}
	if len(args) != 1 || args[0] != 1 {
		t.Errorf("args should be preserved separately: got %v", args)
	}
}

// TestWhereRawExpressionNotAutoSafe documents and tests that raw WHERE is NOT auto-safe
func TestWhereRawExpressionNotAutoSafe(t *testing.T) {
	// Even with validation, raw SQL expressions must use placeholders
	q := Table("users").Where("role = '" + "admin" + "'")
	query, _, err := q.ToSQL()
	if err != nil {
		t.Fatalf("ToSQL() error: %v", err)
	}
	// String literal remains as-is in raw condition - not automatically escaped
	// Caller is responsible for injection prevention
	if !strings.Contains(query, "'admin'") {
		t.Errorf("raw string literal in condition not preserved as-is")
	}
}

// TestWherePlaceholderCountMismatch tests argument/placeholder validation
func TestWherePlaceholderCountMismatch(t *testing.T) {
	tests := []struct {
		name string
		cond string
		args []any
	}{
		{name: "too few args", cond: "id = ? AND name = ?", args: []any{1}},
		{name: "too many args", cond: "id = ?", args: []any{1, 2}},
		{name: "zero placeholders with args", cond: "id = 1", args: []any{1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Table("users").Where(tt.cond, tt.args...)
			_, _, err := q.ToSQL()
			if err == nil {
				t.Errorf("expected placeholder count mismatch error")
			}
		})
	}
}

// TestWhereStringLiteralWithSemicolonAndQuotes tests complex literals
func TestWhereStringLiteralWithSemicolonAndQuotes(t *testing.T) {
	q := Table("users").Where("name = 'O''Neil; DROP TABLE'")
	query, _, err := q.ToSQL()
	if err != nil {
		t.Fatalf("ToSQL() error: %v", err)
	}
	// Literal must not be modified
	if !strings.Contains(query, "O''Neil; DROP TABLE") {
		t.Errorf("literal content corrupted: %s", query)
	}
}

// TestWherePlaceholderInjectionPrevention verifies args stay separate
func TestWherePlaceholderInjectionPrevention(t *testing.T) {
	malicious := "' OR '1'='1"
	q := Table("users").Where("name = ?", malicious)
	query, args, err := q.ToSQL()
	if err != nil {
		t.Fatalf("ToSQL() error: %v", err)
	}
	if query != "SELECT * FROM `users` WHERE `name` = ?" {
		t.Errorf("unexpected query: %s", query)
	}
	if len(args) != 1 || args[0] != malicious {
		t.Errorf("malicious arg not preserved separately: %v", args)
	}
}

// TestWherePlaceholderCounting ignores question marks inside literals/quotes/comments.
func TestWherePlaceholderCounting(t *testing.T) {
	tests := []struct {
		name      string
		cond      string
		args      []any
		wantErr   bool
		wantQuery string
	}{
		{
			name:      "placeholder outside quotes",
			cond:      "name = ?",
			args:      []any{"John"},
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = ?",
		},
		{
			name:      "question mark inside single-quoted literal",
			cond:      "name = '?'",
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = '?'",
		},
		{
			name:      "question mark inside double-quoted literal",
			cond:      `name = "?"`,
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = \"?\"",
		},
		{
			name:      "question mark outside backtick identifier",
			cond:      "`col` = ?",
			args:      []any{1},
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `col` = ?",
		},
		{
			name:      "placeholder with escaped quote literal",
			cond:      "name = 'O''?'",
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = 'O''?'",
		},
		{
			name:      "question mark in SQL comment",
			cond:      "id = 1 -- ?",
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `id` = 1 -- ?",
		},
		{
			name:      "question mark in block comment",
			cond:      "id = 1 /* ? */",
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `id` = 1 /* ? */",
		},
		{
			name:      "mixed placeholders and question marks in literals",
			cond:      "name = ? AND note = '?'",
			args:      []any{"John"},
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = ? AND `note` = '?'",
		},
		{
			name:      "mismatch still detected when real placeholders differ",
			cond:      "id = ? AND name = ?",
			args:      []any{1},
			wantErr:   true,
			wantQuery: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Table("users").Where(tt.cond, tt.args...)
			query, args, err := q.ToSQL()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got query: %s", query)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := tt.wantQuery
			if want == "" {
				want = "SELECT * FROM `users` WHERE " + tt.cond
			}
			if query != want {
				t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", want, query)
			}
			if len(args) != len(tt.args) {
				t.Errorf("args length mismatch: got %d, want %d", len(args), len(tt.args))
			}
			for i, got := range args {
				if got != tt.args[i] {
					t.Errorf("arg[%d] mismatch: got %v, want %v", i, got, tt.args[i])
				}
			}
		})
	}
}

// TestWhereEscapedQuotes tests that escaped quotes inside quoted text
// do not end the literal prematurely and placeholders inside are ignored.
func TestWhereEscapedQuotes(t *testing.T) {
	tests := []struct {
		name      string
		cond      string
		args      []any
		wantErr   bool
		wantQuery string
	}{
		{
			name:      "escaped single quote in single-quoted literal",
			cond:      "name = 'O''Brien'",
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = 'O''Brien'",
		},
		{
			name:      "escaped double quote in double-quoted literal",
			cond:      `name = "O""Brien"`,
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = \"O\"\"Brien\"",
		},
		{
			name:      "escaped backtick in backtick-quoted identifier",
			cond:      "`col``x` = ?",
			args:      []any{1},
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `col``x` = ?",
		},
		{
			name:      "question mark inside escaped single-quoted literal",
			cond:      "name = 'O''?'",
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = 'O''?'",
		},
		{
			name:      "question mark inside escaped double-quoted literal",
			cond:      `name = "O""?"`,
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = \"O\"\"?\"",
		},
		{
			name:      "backslash escape inside single-quoted literal",
			cond:      "path = 'C:\\\\Users\\\\test'",
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `path` = 'C:\\\\Users\\\\test'",
		},
		{
			name:      "backslash escape inside double-quoted literal",
			cond:      "path = \"C:\\\\Users\\\\test\"",
			args:      nil,
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `path` = \"C:\\\\Users\\\\test\"",
		},
		{
			name:      "backslash escape inside backtick-quoted identifier",
			cond:      "`col\\`x` = ?",
			args:      []any{1},
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `col\\`x` = ?",
		},
		{
			name:      "placeholder valid outside escaped literal",
			cond:      "name = ? AND note = 'O''Brien'",
			args:      []any{"John"},
			wantErr:   false,
			wantQuery: "SELECT * FROM `users` WHERE `name` = ? AND `note` = 'O''Brien'",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Table("users").Where(tt.cond, tt.args...)
			query, args, err := q.ToSQL()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got query: %s", query)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := tt.wantQuery
			if want == "" {
				want = "SELECT * FROM `users` WHERE " + tt.cond
			}
			if query != want {
				t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", want, query)
			}
			if len(args) != len(tt.args) {
				t.Errorf("args length mismatch: got %d, want %d", len(args), len(tt.args))
			}
			for i, got := range args {
				if got != tt.args[i] {
					t.Errorf("arg[%d] mismatch: got %v, want %v", i, got, tt.args[i])
				}
			}
		})
	}
}

// TestWhereMultipleCalls verifies multiple Where calls accumulate parts and args.
func TestWhereMultipleCalls(t *testing.T) {
	q := Table("users").Where("id = ?", 1).Where("name = ?", "John")
	query, args, err := q.ToSQL()
	if err != nil {
		t.Fatalf("ToSQL() error: %v", err)
	}
	want := "SELECT * FROM `users` WHERE `id` = ? AND `name` = ?"
	if query != want {
		t.Errorf("query mismatch:\nexpected: %s\ngot:      %s", want, query)
	}
	if len(args) != 2 || args[0] != 1 || args[1] != "John" {
		t.Errorf("args mismatch: %v", args)
	}
}
