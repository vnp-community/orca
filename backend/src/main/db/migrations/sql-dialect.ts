/**
 * Migration SQL Dialect Helpers
 *
 * Migrations 0001-0010 were written as SQLite-only SQL (`datetime('now')`,
 * `strftime(...)`, `INTEGER PRIMARY KEY AUTOINCREMENT`) — none of that is valid
 * Postgres/MySQL syntax. Confirmed live on b15.openledger.vn: switching `pool`
 * to Postgres crashed migrations with `function datetime(unknown) does not
 * exist`, leaving `orca_users` uncreated (see
 * docs/guides/postgres-shared-database-design.md, BUG-BE-RPC-003 follow-up).
 *
 * These helpers return the equivalent SQL fragment per `db.capabilities.dialect`
 * so a migration's `CREATE TABLE` stays a single template literal instead of a
 * fully duplicated per-dialect statement.
 *
 * @module db/migrations/sql-dialect
 */

import type { IDatabaseCapabilities } from '../types'

type Dialect = IDatabaseCapabilities['dialect']

/**
 * DEFAULT expression for a TEXT column storing "now" as
 * `YYYY-MM-DD HH:MM:SS` (matches SQLite's `datetime('now')` format exactly,
 * so existing `new Date(text)` call sites keep parsing the same way).
 */
export function nowTextDefaultSql(dialect: Dialect): string {
  switch (dialect) {
    case 'postgresql':
      return "(to_char(CURRENT_TIMESTAMP AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS'))"
    case 'mysql':
    case 'tidb':
    case 'mariadb':
      return '(UTC_TIMESTAMP())'
    case 'sqlite':
    default:
      return "(datetime('now'))"
  }
}

/**
 * DEFAULT expression for an INTEGER column storing "now" as Unix epoch
 * milliseconds (matches SQLite's `strftime('%s', 'now') * 1000` pattern
 * already used elsewhere in these migrations, e.g. 0010_tasks.ts).
 */
export function nowEpochMsDefaultSql(dialect: Dialect): string {
  switch (dialect) {
    case 'postgresql':
      return '(FLOOR(EXTRACT(EPOCH FROM CURRENT_TIMESTAMP) * 1000))'
    case 'mysql':
    case 'tidb':
    case 'mariadb':
      return '(UNIX_TIMESTAMP() * 1000)'
    case 'sqlite':
    default:
      return "(strftime('%s', 'now') * 1000)"
  }
}

/**
 * Auto-incrementing integer PRIMARY KEY column definition — SQLite's
 * `AUTOINCREMENT` keyword doesn't exist in any other dialect. Postgres 10+
 * supports the SQL-standard `GENERATED ALWAYS AS IDENTITY`; MySQL/TiDB/
 * MariaDB use `AUTO_INCREMENT`.
 */
export function autoIncrementPrimaryKeySql(dialect: Dialect): string {
  switch (dialect) {
    case 'postgresql':
      return 'INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY'
    case 'mysql':
    case 'tidb':
    case 'mariadb':
      return 'INTEGER PRIMARY KEY AUTO_INCREMENT'
    case 'sqlite':
    default:
      return 'INTEGER PRIMARY KEY AUTOINCREMENT'
  }
}
