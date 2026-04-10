# SQL Migrations

Database schema management for Colink. Dual DB: MySQL 8.0 (production) + SQLite (development).

## Structure

```
sql-change/
├── init.sql               # Current full schema (new environments)
├── README.md              # Team workflow documentation (Chinese)
└── history/               # Archived migrations (pre-1.0.0, 31 files)
    └── 202604060001_add_im_sessions.sql
```

Versioned migrations (post-1.0.0) go in `migrations/v{version}/`:
```
sql-change/migrations/v1.0.1/
    └── 202604100001_add_xxx.sql
```

## Conventions

- **File naming**: `YYYYMMDDNN_description.sql` (date + sequence + description)
- **One file per change**, sequential execution order
- **Include rollback SQL** as comments
- **NEVER modify `init.sql`** — use migrations instead
- **NEVER use `DROP COLUMN IF EXISTS`** — MySQL 5.7 incompatible
- **Test first** in personal `dev_<name>` database before product

## Workflow

1. Create migration file in `sql-change/migrations/v{version}/`
2. Update corresponding model in `internal/model/` and repo in `internal/repo/`
3. Test in dev database
4. For new environments: run `init.sql`, then apply versioned migrations

## Dialect Abstraction

`internal/repo/db.go` defines `Dialect` interface: `Placeholder()`, `QuoteIdentifier()`, `AutoIncrement()`. Write SQL with `?` placeholders — the dialect layer handles MySQL vs SQLite differences.
