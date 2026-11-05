package orm

import (
	"fmt"
	"regexp"
	"strings"
)

// identRe matches valid SQL identifiers.
var identRe = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// validateIdent checks if s is a valid SQL identifier.
func validateIdent(s string) error {
	if !identRe.MatchString(s) {
		return fmt.Errorf("invalid identifier %q", s)
	}
	return nil
}

// QueryBuilder provides a fluent interface for building SELECT queries.
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
	dialect    Dialect
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
// Placeholders ("?") are used for values and passed as separate arguments.
// The condition string is parsed by requote: identifiers are quoted with backticks,
// while string literals (single/double quoted), backtick-quoted identifiers, and
// SQL comments are preserved as-is during placeholder counting.
//
// Escapes supported:
//   - Single-quoted strings: ” for escaped quote, \ for backslash escapes
//   - Double-quoted strings: "" for escaped quote, \ for backslash escapes
//   - Backtick-quoted identifiers: “ for escaped backtick, \ for backslash escapes
//   - SQL comments: -- line comments, /* block comments */
//
// Security note: Where accepts raw SQL expressions from the caller. The condition
// expression itself is not validated for safety — values MUST use placeholders
// to prevent SQL injection. Raw conditions are NOT automatically safe. Passing
// user-controlled SQL fragments is unsafe even if arguments use placeholders.
func countPlaceholders(cond string) int {
	count := 0
	state := "normal"
	for i := 0; i < len(cond); i++ {
		ch := cond[i]
		switch state {
		case "normal":
			if ch == '\'' {
				state = "single"
			} else if ch == '"' {
				state = "double"
			} else if ch == '`' {
				state = "backtick"
			} else if ch == '-' && i+1 < len(cond) && cond[i+1] == '-' {
				state = "line"
				i++ // skip second '-'
			} else if ch == '/' && i+1 < len(cond) && cond[i+1] == '*' {
				state = "block"
				i++ // skip '*'
			} else if ch == '?' {
				count++
			}
		case "single":
			if ch == '\'' {
				// escaped single quote ''
				if i+1 < len(cond) && cond[i+1] == '\'' {
					i++
					continue
				}
				state = "normal"
			} else if ch == '\\' && i+1 < len(cond) {
				i++ // skip escaped character
			}
		case "double":
			if ch == '"' {
				// escaped double quote ""
				if i+1 < len(cond) && cond[i+1] == '"' {
					i++
					continue
				}
				state = "normal"
			} else if ch == '\\' && i+1 < len(cond) {
				i++ // skip escaped character
			}
		case "backtick":
			if ch == '`' {
				// escaped backtick ``
				if i+1 < len(cond) && cond[i+1] == '`' {
					i++
					continue
				}
				state = "normal"
			} else if ch == '\\' && i+1 < len(cond) {
				i++ // skip escaped character
			}
		case "line":
			if ch == '\n' {
				state = "normal"
			}
		case "block":
			if ch == '*' && i+1 < len(cond) && cond[i+1] == '/' {
				state = "normal"
				i++
			}
		}
	}
	return count
}

func (q *QueryBuilder) Where(condition string, args ...any) *QueryBuilder {
	if condition != "" {
		if placeholders := countPlaceholders(condition); placeholders != len(args) {
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

// Dialect sets the SQL dialect for the query builder.
func (q *QueryBuilder) Dialect(d Dialect) *QueryBuilder {
	if d != nil {
		q.dialect = d
	}
	return q
}

// ToSQL builds the final SQL query and arguments.
func (q *QueryBuilder) ToSQL() (string, []any, error) {
	if q.err != nil {
		return "", nil, q.err
	}
	if q.dialect == nil {
		q.dialect = defaultDialect
	}
	// Quote identifiers in WHERE clause using the dialect.
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
					if cond[i] == '\'' && i+1 < len(cond) && cond[i+1] == '\'' {
						result.WriteByte('\'')
						result.WriteByte('\'')
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
					if cond[i] == '"' && i+1 < len(cond) && cond[i+1] == '"' {
						result.WriteByte('"')
						result.WriteByte('"')
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
					if cond[i] == '`' && i+1 < len(cond) && cond[i+1] == '`' {
						result.WriteByte('`')
						result.WriteByte('`')
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
					result.WriteString(q.dialect.QuoteIdent(m))
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
			query.WriteString(q.dialect.QuoteIdent(col))
		}
	}

	query.WriteString(" FROM ")
	query.WriteString(q.dialect.QuoteIdent(q.tableName))

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
		query.WriteString(" ORDER BY ")
		query.WriteString(q.dialect.QuoteIdent(q.orderByCol))
		query.WriteString(" ")
		query.WriteString(q.orderByDir)
	}

	// LIMIT/OFFSET via dialect
	if q.limitN > 0 || q.offsetN > 0 {
		query.WriteString(q.dialect.LimitOffset(q.limitN, q.offsetN))
	}

	return query.String(), q.whereArgs, nil
}
