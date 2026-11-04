package orm

import (
	"fmt"
	"regexp"
	"strings"
)

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateIdent returns error when identifier contains unsafe characters.
func validateIdent(s string) error {
	if !identRe.MatchString(s) {
		return fmt.Errorf("invalid identifier %q", s)
	}
	return nil
}

// QueryBuilder provides fluent interface for building SQL queries.
type QueryBuilder struct {
	tableName  string
	selectCols []string
	whereParts []string
	whereArgs  []any
	orderByCol string
	orderByDir string
	limitN     int
	offsetN    int
	err        error
}

// Table sets the table name.
func (q *QueryBuilder) Table(name string) *QueryBuilder {
	if err := validateIdent(name); err != nil {
		q.tableName = ""
		q.err = err
		return q
	}
	q.tableName = name
	return q
}

// Select sets the columns to select.
func (q *QueryBuilder) Select(cols ...string) *QueryBuilder {
	for _, c := range cols {
		if err := validateIdent(c); err != nil {
			q.err = err
			return q
		}
	}
	q.selectCols = cols
	return q
}

// Where adds a WHERE clause condition.
func (q *QueryBuilder) Where(condition string, args ...any) *QueryBuilder {
	if condition != "" {
		q.whereParts = append(q.whereParts, condition)
		q.whereArgs = append(q.whereArgs, args...)
	}
	return q
}

// OrderBy sets the ORDER BY clause.
func (q *QueryBuilder) OrderBy(col string, dir ...string) *QueryBuilder {
	if err := validateIdent(col); err != nil {
		q.err = err
		return q
	}
	q.orderByCol = col
	if len(dir) > 0 {
		q.orderByDir = strings.ToUpper(dir[0])
		if q.orderByDir != "ASC" && q.orderByDir != "DESC" {
			q.orderByDir = "ASC"
		}
	} else {
		q.orderByDir = "ASC"
	}
	return q
}

// Limit sets the LIMIT clause.
func (q *QueryBuilder) Limit(n int) *QueryBuilder {
	q.limitN = n
	return q
}

// Offset sets the OFFSET clause.
func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	q.offsetN = n
	return q
}

// ToSQL builds the final SQL query and arguments.
func (q *QueryBuilder) ToSQL() (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}
	// Quote identifiers in WHERE clause
	requote := func(cond string) string {
		re := regexp.MustCompile(`\b([a-zA-Z_][a-zA-Z0-9_]*)\b`)
		return re.ReplaceAllStringFunc(cond, func(m string) string {
			if m == "?" || strings.EqualFold(m, "AND") || strings.EqualFold(m, "OR") || strings.EqualFold(m, "NOT") {
				return m
			}
			return "`" + m + "`"
		})
	}

	if q.tableName == "" {
		return "", nil, fmt.Errorf("table name is required")
	}

	var query strings.Builder
	query.WriteString("SELECT ")

	if len(q.selectCols) == 0 {
		query.WriteString("*")
	} else {
		for i, col := range q.selectCols {
			if i > 0 {
				query.WriteString(", ")
			}
			query.WriteString("`")
			query.WriteString(col)
			query.WriteString("`")
		}
	}

	query.WriteString(" FROM `")
	query.WriteString(q.tableName)
	query.WriteString("`")

	// WHERE
	if len(q.whereParts) > 0 {
		query.WriteString(" WHERE ")
		parts := make([]string, len(q.whereParts))
		for i, part := range q.whereParts {
			parts[i] = requote(part)
		}
		query.WriteString(strings.Join(parts, " AND "))
	}

	// ORDER BY
	if q.orderByCol != "" {
		query.WriteString(" ORDER BY `")
		query.WriteString(q.orderByCol)
		query.WriteString("` ")
		query.WriteString(q.orderByDir)
	}

	// LIMIT
	if q.limitN > 0 {
		query.WriteString(" LIMIT ")
		query.WriteString(fmt.Sprintf("%d", q.limitN))
	}

	// OFFSET
	if q.offsetN > 0 {
		query.WriteString(" OFFSET ")
		query.WriteString(fmt.Sprintf("%d", q.offsetN))
	}

	return query.String(), q.whereArgs, nil
}
