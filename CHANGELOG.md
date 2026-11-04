# Changelog

All notable changes to this project will be documented in this file.

## Unreleased
*Added*
- Implemented core ORM package with `ORM`, `QueryBuilder`, and model tag parsing.
- Provided CRUD methods: `Insert`, `FindByID`, `FindAll`, `Update`, `Delete`.
- Added fluent `QueryBuilder` with `Select`, `Where`, `OrderBy`, `Limit`, `Offset` and a `ToSQL` method.
- Introduced `DBExecutor` interface for database‑agnostic operation.
- Added identifier validation for table/column names.
- Included comprehensive test suite covering query building, CRUD operations, transaction usage, and model metadata.

*Changed*
- Verified implementation across multiple commits; package is ready for usage.

*Fixed*
- Placeholder count validation in `Where` ensures no argument mismatch.
- Primary‑key zero‑value omission during insert to allow auto-generation.

---
*Note: No formal release version has been tagged; entries reflect the current state of the repository.*