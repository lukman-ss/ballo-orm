# Changelog

All notable changes to this project will be documented in this file.

## Unreleased
*Changed*
- `countPlaceholders` now handles escape sequences: `''` and `\` in single-quoted strings, `""` and `\` in double-quoted strings, `` `` `` and `\` in backtick identifiers.
- Placeholder counting correctly ignores `?` inside quoted literals and comments regardless of escape style.
- `TestWhereDangerousIdentifierInjection` updated assertion to clarify requote behavior: identifiers are quoted; raw SQL fragments pass through unchanged.
- `TestWhereStringLiteralPreservation` expectations updated to include full SELECT prefix.
- New `TestWhereEscapedQuotes` tests verify escaped quotes and backslash handling in all quote styles.
- `README` dialect and placeholder-counting notes expanded.

---
*Note: No formal release version has been tagged; entries reflect the current state of the repository.*