package orm

import (
	"database/sql"
	"fmt"
	"os"
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
	err := o.Insert(&user)
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
	err := o.Insert(&user)
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	var found User
	err = o.FindByID(user.ID, &found)
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
		o.Insert(&User{
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   intPtr(20 + i),
		})
	}

	var users []User
	q := Table("users").OrderBy("id")
	err := o.FindAll(q, &users)
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
	err := o.Insert(&user)
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	user.Name = "Updated Name"
	err = o.Update(&user)
	if err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	var updated User
	err = o.FindByID(user.ID, &updated)
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
	err := o.Insert(&user)
	if err != nil {
		t.Fatalf("Insert() error: %v", err)
	}

	err = o.Delete(&user)
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
	err = tORM.Insert(&user)
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

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
