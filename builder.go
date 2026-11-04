package orm

import (
	"fmt"
	"regexp"
	"strings"
)

var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateIdent returns error when identifier contains unsafe characters.
// Validate identifier before usage.
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
//
// The condition string is taken as-is; identifiers are quoted safely by ToSQL().
// Placeholders ("?") are used for values and passed as separate arguments.
//
// Security note: Where accepts raw SQL expressions from the caller. The condition
// expression itself is not validated for safety — values MUST use placeholders
// to prevent SQL injection. Raw conditions are NOT automatically safe. Passing
// user-controlled SQL fragments is unsafe even if arguments use placeholders.
func (q *QueryBuilder) Where(condition string, args ...any) *QueryBuilder {
	if condition != "" {
		// Count placeholders and verify argument count matches.
		if placeholders := strings.Count(condition, "?"); placeholders != len(args) {
			q.err = fmt.Errorf("placeholder count mismatch: %d args provided, %d placeholders", len(args), placeholders)
			return q
		}
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
	if n < 0 {
		q.err = fmt.Errorf("LIMIT must be non-negative, got %d", n)
		return q
	}
	q.limitN = n
	return q
}

// Offset sets the OFFSET clause.
func (q *QueryBuilder) Offset(n int) *QueryBuilder {
	if n < 0 {
		q.err = fmt.Errorf("OFFSET must be non-negative, got %d", n)
		return q
	}
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
		// Track single/double quotes to avoid re-quoting inside string literals
		var result strings.Builder
		i := 0
		for i < len(cond) {
			ch := cond[i]
			// Single-quoted string literal
			if ch == '\'' {
				// Copy whole literal including escaped quotes
				result.WriteByte(ch)
				i++
				for i < len(cond) {
					if cond[i] == '\\' && i+1 < len(cond) {
						result.WriteByte(cond[i])
						result.WriteByte(cond[i+1])
						i += 2
						continue
					}
					result.WriteByte(cond[i])
					if cond[i] == '\'' {
						i++
						break
					}
					i++
				}
				continue
			}
			// Double-quoted string literal
			if ch == '"' {
				result.WriteByte(ch)
				i++
				for i < len(cond) {
					if cond[i] == '\\' && i+1 < len(cond) {
						result.WriteByte(cond[i])
						result.WriteByte(cond[i+1])
						i += 2
						continue
					}
					result.WriteByte(cond[i])
					if cond[i] == '"' {
						i++
						break
					}
					i++
				}
				continue
			}
			// Backtick-quoted identifier - preserve as-is
			if ch == '`' {
				result.WriteByte(ch)
				i++
				for i < len(cond) {
					if cond[i] == '\\' && i+1 < len(cond) {
						result.WriteByte(cond[i])
						result.WriteByte(cond[i+1])
						i += 2
						continue
					}
					result.WriteByte(cond[i])
					if cond[i] == '`' {
						i++
						break
					}
					i++
				}
				continue
			}
			// Identifier or keyword/placeholder
			if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_' {
				start := i
				i++
				for i < len(cond) {
					c := cond[i]
					if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
						i++
						continue
					}
					break
				}
				m := cond[start:i]
				if m == "?" || strings.EqualFold(m, "AND") || strings.EqualFold(m, "OR") || strings.EqualFold(m, "NOT") ||
					strings.EqualFold(m, "TRUE") || strings.EqualFold(m, "FALSE") || strings.EqualFold(m, "NULL") ||
					strings.EqualFold(m, "ASC") || strings.EqualFold(m, "DESC") || strings.EqualFold(m, "IN") ||
					strings.EqualFold(m, "LIKE") || strings.EqualFold(m, "BETWEEN") || strings.EqualFold(m, "EXISTS") ||
					strings.EqualFold(m, "IS") {
					result.WriteString(m)
					continue
				}
				if identRe.MatchString(m) {
					// If followed by '-' it may be part of dashed identifier - pass through whole problematic token
					if i < len(cond) && cond[i] == '-' {
						// read rest of dashed sequence
						j := i
						for j < len(cond) {
							c := cond[j]
							if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '-' {
								j++
								continue
							}
							break
						}
						result.WriteString(cond[start:j])
						i = j
						continue
					}
					result.WriteString("`" + m + "`")
					continue
				}
				// Invalid identifier in raw WHERE — pass through as-is.
				result.WriteString(m)
				continue
			}
			result.WriteByte(ch)
			i++
		}
		return result.String()
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
