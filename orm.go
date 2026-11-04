package orm

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
)

// DBExecutor abstracts database operations compatible with *sql.DB and *sql.Tx.
type DBExecutor interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
	Exec(query string, args ...any) (sql.Result, error)
}

// ORM provides CRUD operations using a DBExecutor.
type ORM struct {
	db DBExecutor
}

// New creates a new ORM instance.
func New(db DBExecutor) *ORM {
	return &ORM{db: db}
}

// Table returns a new QueryBuilder.
func Table(name string) *QueryBuilder {
	return &QueryBuilder{tableName: name}
}

// Insert inserts a model into the database.
func (o *ORM) Insert(model any) error {
	pk, _ := GetPrimaryKey(model)
	tb, cols, values := buildInsertQuery(model)
	// Omit zero primary key so database auto-generates it.
	if pk != "" {
		newCols := make([]string, 0, len(cols))
		newVals := make([]any, 0, len(values))
		for i, col := range cols {
			if col == pk {
				v := getValue(model, col)
				if iv, ok := toInt64(v); ok && iv == 0 {
					continue
				}
			}
			newCols = append(newCols, col)
			newVals = append(newVals, values[i])
		}
		cols, values = newCols, newVals
	}
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		tb,
		formatColumns(cols),
		formatPlaceholders(len(cols)),
	)
	result, err := o.db.Exec(query, values...)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		tagStr := field.Tag.Get("db")
		dbTag := ParseDBTag(tagStr)
		if dbTag.PrimaryKey {
			fv := v.Field(i)
			if fv.CanSet() && (fv.Kind() == reflect.Int || fv.Kind() == reflect.Int64) && fv.Int() == 0 {
				fv.SetInt(id)
			}
		}
	}
	return nil
}

// FindByID finds a record by primary key.
func (o *ORM) FindByID(id any, dest any) error {
	pk, found := GetPrimaryKey(dest)
	if !found {
		return fmt.Errorf("primary key not found in struct")
	}
	table := buildTableName(dest)
	cols := GetColumns(dest)
	colList := strings.Join(cols, ", ")
	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", colList, table, pk)
	row := o.db.QueryRow(query, id)

	// Prepare pointers slice for Scan
	v := reflect.ValueOf(dest)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return fmt.Errorf("dest must be a struct pointer")
	}
	fieldPtrs := make([]any, len(cols))
	for i, col := range cols {
		fieldIdx := -1
		for j := 0; j < v.NumField(); j++ {
			f := v.Type().Field(j)
			c := ParseDBTag(f.Tag.Get("db")).Name
			if c == "" {
				c = snakeCase(f.Name)
			}
			if c == col {
				fieldIdx = j
				break
			}
		}
		if fieldIdx >= 0 {
			fieldPtrs[i] = v.Field(fieldIdx).Addr().Interface()
		} else {
			var placeholder any
			fieldPtrs[i] = &placeholder
		}
	}
	err := row.Scan(fieldPtrs...)
	if err != nil {
		return err
	}
	return nil
}

// FindAll executes a query and scans results into a slice.
func (o *ORM) FindAll(q *QueryBuilder, destSlice any) error {
	query, args, err := q.ToSQL()
	if err != nil {
		return err
	}
	rows, err := o.db.Query(query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	return scanRows(rows, destSlice)
}

// Update updates a model in the database.
func (o *ORM) Update(model any) error {
	pk, _ := GetPrimaryKey(model)
	tb, setClauses, values := buildUpdateQuery(model, pk)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", tb, setClauses, pk)
	values = append(values, getValue(model, pk))
	_, err := o.db.Exec(query, values...)
	return err
}

// Delete deletes a record by primary key.
func (o *ORM) Delete(model any) error {
	pk, found := GetPrimaryKey(model)
	if !found {
		return fmt.Errorf("primary key not found")
	}
	tb := buildTableName(model)
	val := getValue(model, pk)
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", tb, pk)
	_, err := o.db.Exec(query, val)
	return err
}

// buildInsertQuery builds the INSERT query components.
func buildInsertQuery(model any) (string, []string, []any) {
	tb := buildTableName(model)
	cols := GetColumns(model)
	var values []any
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	for _, col := range cols {
		val := getFieldValue(v, col)
		values = append(values, val)
	}
	return tb, cols, values
}

// buildUpdateQuery builds the UPDATE query components.
func buildUpdateQuery(model any, pk string) (string, string, []any) {
	tb := buildTableName(model)
	cols := GetColumns(model)
	var setParts []string
	var values []any
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	for _, col := range cols {
		if col == pk {
			continue
		}
		setParts = append(setParts, fmt.Sprintf("%s = ?", col))
		values = append(values, getFieldValue(v, col))
	}
	return tb, strings.Join(setParts, ", "), values
}

func buildTableName(model any) string {
	t := reflect.TypeOf(model)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return "unknown"
	}
	name := snakeCase(t.Name())
	return name + "s"
}

func getFieldValue(v reflect.Value, colName string) any {
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		tagStr := field.Tag.Get("db")
		dbTag := ParseDBTag(tagStr)
		fieldCol := dbTag.Name
		if fieldCol == "" {
			fieldCol = snakeCase(field.Name)
		}
		if fieldCol == colName {
			return v.Field(i).Interface()
		}
	}
	return nil
}

func getValue(model any, colName string) any {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	return getFieldValue(v, colName)
}

func formatColumns(cols []string) string {
	for i, c := range cols {
		cols[i] = fmt.Sprintf("`%s`", c)
	}
	return strings.Join(cols, ", ")
}

func formatPlaceholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ", ")
}

func toInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int:
		return int64(x), true
	case int64:
		return x, true
	default:
		return 0, false
	}
}

// scanRows scans sql.Rows into a slice of structs.
func scanRows(rows *sql.Rows, dest any) error {
	rv := reflect.ValueOf(dest)
	if rv.Kind() != reflect.Ptr || rv.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("dest must be a pointer to a slice")
	}

	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	colIndex := make(map[string]int)
	for i, c := range cols {
		colIndex[c] = i
	}

	destSlice := reflect.MakeSlice(rv.Elem().Type(), 0, 0)

	for rows.Next() {
		elem := reflect.New(rv.Elem().Type().Elem()).Elem()
		values := make([]any, len(cols))
		fieldPtrs := make([]any, len(cols))
		for i := range values {
			fieldPtrs[i] = &values[i]
		}
		if err := rows.Scan(fieldPtrs...); err != nil {
			return err
		}
		t := elem.Type()
		for j := 0; j < t.NumField(); j++ {
			field := t.Field(j)
			tagStr := field.Tag.Get("db")
			dbTag := ParseDBTag(tagStr)
			colName := dbTag.Name
			if colName == "" {
				colName = snakeCase(field.Name)
			}
			if idx, ok := colIndex[colName]; ok {
				setField(elem, j, values[idx])
			}
		}
		destSlice = reflect.Append(destSlice, elem)
	}
	rv.Elem().Set(destSlice)
	return nil
}

func setField(v reflect.Value, index int, value any) {
	field := v.Field(index)
	if !field.CanSet() {
		return
	}
	switch field.Kind() {
	case reflect.String:
		if b, ok := value.([]byte); ok {
			field.SetString(string(b))
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, ok := value.(int64); ok {
			field.SetInt(n)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if n, ok := value.(int64); ok {
			field.SetUint(uint64(n))
		}
	case reflect.Float32, reflect.Float64:
		if n, ok := value.(float64); ok {
			field.SetFloat(n)
		}
	}
}
