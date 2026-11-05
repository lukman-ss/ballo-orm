package orm

import (
	"fmt"
)

// Dialect abstracts SQL dialect differences.
type Dialect interface {
	// QuoteIdent quotes an identifier (table or column name).
	QuoteIdent(ident string) string
	// Placeholder returns the placeholder string for the given index (starting at 1).
	// Most dialects use "?" regardless of index.
	Placeholder(idx int) string
	// LimitOffset returns the LIMIT/OFFSET clause for given values.
	// Return empty string if not applicable.
	LimitOffset(limit, offset int) string
}

// sqliteDialect implements Dialect for SQLite/MySQL style backticks and "?" placeholders.
type sqliteDialect struct{}

func (d sqliteDialect) QuoteIdent(ident string) string { return "`" + ident + "`" }
func (d sqliteDialect) Placeholder(idx int) string     { return "?" }
func (d sqliteDialect) LimitOffset(limit, offset int) string {
	if limit > 0 && offset > 0 {
		return " LIMIT " + fmt.Sprintf("%d", limit) + " OFFSET " + fmt.Sprintf("%d", offset)
	}
	if limit > 0 {
		return " LIMIT " + fmt.Sprintf("%d", limit)
	}
	if offset > 0 {
		// SQLite does not support OFFSET without LIMIT; use large LIMIT.
		return " LIMIT 9223372036854775807 OFFSET " + fmt.Sprintf("%d", offset)
	}
	return ""
}

// defaultDialect is the dialect used throughout the package.
var defaultDialect Dialect = sqliteDialect{}
