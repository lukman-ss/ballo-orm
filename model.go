package orm

import (
	"reflect"
	"strings"
)

// DBTag represents parsed struct field tag information.
type DBTag struct {
	Name       string
	PrimaryKey bool
	Skip       bool
}

// ParseDBTag parses a struct field tag and returns DBTag info.
func ParseDBTag(tag string) DBTag {
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "-" {
		return DBTag{Skip: true}
	}
	tag = strings.Join(parts[1:], ",")
	return DBTag{
		Name:       name,
		PrimaryKey: strings.Contains(tag, "primary_key"),
	}
}

// GetPrimaryKey returns the primary key column name for a model struct.
func GetPrimaryKey(model any) (string, bool) {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return "", false
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tagStr := field.Tag.Get("db")
		dbTag := ParseDBTag(tagStr)
		if dbTag.PrimaryKey {
			return dbTag.Name, true
		}
		// Default: field named "ID"
		if strings.EqualFold(field.Name, "ID") {
			colName := snakeCase(field.Name)
			if tagStr != "" {
				colName = dbTag.Name
			}
			return colName, true
		}
	}
	return "", false
}

// GetColumns returns column names for a struct.
func GetColumns(model any) []string {
	v := reflect.ValueOf(model)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	var cols []string
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		tagStr := field.Tag.Get("db")
		dbTag := ParseDBTag(tagStr)
		if dbTag.Skip {
			continue
		}
		colName := dbTag.Name
		if colName == "" {
			colName = snakeCase(field.Name)
		}
		cols = append(cols, colName)
	}
	return cols
}

// snakeCase converts PascalCase to snake_case.
func snakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			result.WriteRune(r + 32)
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// ScanRows scans sql.Rows into a slice of structs.
