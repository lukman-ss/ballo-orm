package orm

import (
	"fmt"
	"testing"
)

func TestDebugFindAll(t *testing.T) {
	db := testDB(t)
	setupSchema(t, db)
	o := New(db)

	for i := 0; i < 5; i++ {
		u := User{
			Name:  fmt.Sprintf("User %d", i),
			Email: fmt.Sprintf("user%d@example.com", i),
			Age:   intPtr(20 + i),
		}
		err := o.Insert(&u)
		if err != nil {
			t.Fatalf("Insert %d error: %v", i, err)
		}
		fmt.Printf("inserted %d: ID=%d Name=%s\n", i, u.ID, u.Name)
	}

	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		t.Fatalf("count error: %v", err)
	}
	fmt.Printf("count in db: %d\n", count)

	var users []User
	q := Table("users").OrderBy("id")
	err = o.FindAll(q, &users)
	if err != nil {
		t.Fatalf("FindAll error: %v", err)
	}
	fmt.Printf("users count: %d\n", len(users))
	for i, u := range users {
		fmt.Printf("%d: %+v\n", i, u)
	}
}
