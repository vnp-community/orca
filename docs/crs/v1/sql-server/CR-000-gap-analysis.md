# CR-000 — Tổng quan: Multi-Database Support Gap Analysis

**Ngày:** 2026-07-23  
**Tác giả:** VNP-BLC Platform Team  
**Status:** ✅ Implemented (2026-07-24)  
**Priority:** High  

---

## 1. Bối cảnh

Orca hiện lưu trữ toàn bộ state ứng dụng trong **JSON file** (`orca-data.json`) và sử dụng **SQLite** (thông qua `node:sqlite` built-in của Node.js) cho một số module analytics riêng biệt:

- `src/main/sqlite/sync-database.ts` — Wrapper bao bọc `node:sqlite` `DatabaseSync`
- `src/main/opencode-usage/` — Đọc SQLite DB của OpenCode CLI để lấy usage analytics
- `src/main/ai-vault/` — Đọc SQLite DB của các AI agents (Claude, Gemini, OpenCode)
- `src/main/automations/hermes-cron-output.ts` — Đọc SQLite session transcripts

Khi triển khai Orca ở môi trường **enterprise/production** (server mode với `server-bootstrap.ts`), việc chỉ dùng SQLite file-based có một số hạn chế quan trọng:

1. **Không scale ngang** — SQLite không hỗ trợ multi-writer concurrent access
2. **Không phù hợp với HA/DR** — Không thể replicate SQLite theo chuẩn enterprise
3. **Không tích hợp với infra hiện có** — Nhiều tổ chức đã có MySQL/PostgreSQL/TiDB cluster
4. **Backup & restore phức tạp** — File-based backup không đảm bảo consistency

---

## 2. Khảo sát hiện trạng

### 2.1 Các điểm lưu trữ dữ liệu trong Orca

| Module | Storage Hiện Tại | File/Location | Mô tả |
|--------|-----------------|---------------|--------|
| Main App State | JSON file | `userData/orca-data.json` | Projects, repos, settings, SSH targets |
| GitHub Cache | JSON file | `userData/orca-github-cache.json` | PR/issue badges cache |
| Stats | JSON file | `userData/orca-stats.json` | Telemetry & usage stats |
| OpenCode Usage | SQLite (read-only) | External OpenCode DBs | Analytics từ OpenCode CLI |
| AI Vault Sessions | SQLite (read-only) | External agent DBs | Session transcripts từ AI agents |
| Terminal Scrollback | Binary files | `userData/scrollback/` | Terminal buffer snapshots |
| Orca Profiles | JSON files | `userData/profiles/` | Profile configs |

### 2.2 Những gì Orca ĐÃ có

| Tính năng | File / Module | Mức độ |
|-----------|---------------|--------|
| JSON persistence (main state) | `persistence.ts` → `orca-data.json` | ✅ Đầy đủ |
| SQLite read adapter | `sqlite/sync-database.ts` | ✅ Cơ bản |
| SQLite-based analytics read | `opencode-usage/scanner.ts` | ✅ Read-only |
| Server mode bootstrap | `server-bootstrap.ts` | ✅ Partial |
| Data path abstraction | `initDataPath()`, `getDataFile()` | ✅ Cơ bản |

### 2.3 Những gì Orca CHƯA có

| Tính năng | Tác động | CR |
|-----------|---------|-----|
| Database provider abstraction layer | Không thể swap backend DB | CR-001 |
| Connection pool & lifecycle management | Không thể dùng MySQL/PgSQL | CR-002 |
| Schema migration framework (multi-DB) | Không thể evolve schema cross-DB | CR-003 |
| Configuration & DSN management | Không có cơ chế config DB connection | CR-004 |
| Repository pattern refactor (main state) | `Store` class coupled to JSON file | CR-005 |
| Health check & connection monitoring | Không biết DB connection có sống không | CR-006 |

---

## 3. Phân tích kiến trúc

### 3.1 Persistence Architecture Hiện Tại

```
src/main/
├── persistence.ts          ← Monolithic Store class (JSON file)
├── sqlite/
│   └── sync-database.ts    ← SQLite adapter (node:sqlite built-in)
├── opencode-usage/
│   ├── scanner.ts          ← Reads external OpenCode SQLite DBs
│   └── schema-helpers.ts   ← SQLite schema introspection
└── ai-vault/
    ├── session-scanner-opencode-sqlite.ts
    └── session-scanner-opencode-sqlite-discovery.ts
```

### 3.2 Persistence Architecture Mục tiêu

```
src/main/
├── persistence.ts               ← Store class (dùng IStateRepository)
├── db/
│   ├── types.ts                 ← IDatabase, IStatement interfaces
│   ├── provider.ts              ← DatabaseProvider factory
│   ├── config.ts                ← DatabaseConfig schema (Zod)
│   ├── sqlite/
│   │   └── sqlite-database.ts   ← SQLite adapter (existing)
│   ├── mysql/
│   │   └── mysql-database.ts    ← MySQL adapter (new)
│   ├── postgresql/
│   │   └── pg-database.ts       ← PostgreSQL adapter (new)
│   └── tidb/
│       └── tidb-database.ts     ← TiDB adapter (MySQL protocol)
└── repositories/
    ├── types.ts                 ← IStateRepository interface
    ├── json-file-repository.ts  ← Existing JSON file storage
    └── sql-repository.ts        ← SQL-based storage (new)
```

---

## 4. Danh sách Change Requests

| CR | Tiêu đề | Priority | Effort |
|----|---------|---------|--------|
| [CR-001](./CR-001-database-provider-abstraction.md) | Database Provider Abstraction Layer | Critical | L |
| [CR-002](./CR-002-connection-pool-lifecycle.md) | Connection Pool & Lifecycle Management | Critical | M |
| [CR-003](./CR-003-schema-migration-framework.md) | Schema Migration Framework | High | L |
| [CR-004](./CR-004-db-config-dsn-management.md) | Database Configuration & DSN Management | High | M |
| [CR-005](./CR-005-state-repository-refactor.md) | State Repository Pattern Refactor | High | XL |
| [CR-006](./CR-006-db-health-monitoring.md) | Database Health Check & Monitoring | Medium | M |

---

## 5. Mức độ ưu tiên & Phasing

```
Phase 1 — Foundation (Bắt buộc):
  CR-001  Database Provider Abstraction Layer
  CR-004  DB Config & DSN Management

Phase 2 — Core Implementation:
  CR-002  Connection Pool & Lifecycle
  CR-003  Schema Migration Framework

Phase 3 — Refactor & Migration:
  CR-005  State Repository Refactor (JSON → SQL)

Phase 4 — Operations:
  CR-006  Health Check & Monitoring
```

---

## 6. Các Database Được Hỗ Trợ

| Database | Protocol | Driver | Ghi chú |
|----------|----------|--------|---------|
| SQLite | Native | `node:sqlite` (built-in) | Default, Desktop mode |
| MySQL 8.x | MySQL | `mysql2` | Production |
| PostgreSQL 14+ | PgSQL | `pg` | Production |
| TiDB | MySQL protocol | `mysql2` | MySQL-compatible |
| MariaDB | MySQL protocol | `mysql2` | MySQL-compatible |

---

## 7. Backwards Compatibility

> **IMPORTANT**: SQLite + JSON file mode PHẢI được giữ làm default cho Desktop Electron app.  
> Multi-DB chỉ cần thiết cho **Server mode** (`server-bootstrap.ts`).  
> Không được breaking change với existing Electron app users.

---

## 8. Implementation Summary

> **✅ IMPLEMENTED — 2026-07-24**  
> Commit: `c165f349d feat: multi-user auth, admin panel, SSH isolation, fleet management, onboarding`

| CR | Status | Files tạo ra |
|----|--------|--------------|
| CR-001 | ✅ Implemented | `src/main/db/types.ts`, `src/main/db/provider.ts`, `src/main/db/sqlite/sqlite-adapter.ts`, `src/main/db/mysql/mysql-adapter.ts`, `src/main/db/postgresql/pg-adapter.ts` |
| CR-002 | ✅ Implemented | `src/main/db/pool.ts`, `src/main/db/generic-pool.ts`, `src/main/db/sqlite/sqlite-pool.ts` |
| CR-003 | ✅ Implemented | `src/main/db/migrations/runner.ts`, `src/main/db/migrations/types.ts`, `src/main/db/migrations/index.ts`, `0001–0005_*.ts` |
| CR-004 | ✅ Implemented | `src/main/db/config.ts`, `src/main/db/dsn-parser.ts`, `src/main/db/config-loader.ts` |
| CR-005 | ✅ Implemented | `src/main/repositories/types.ts`, `src/main/repositories/json-file-repository.ts`, `src/main/repositories/sql-repository.ts`, `src/main/repositories/factory.ts` |
| CR-006 | ✅ Implemented | `src/main/db/health.ts`, `src/main/db/health-monitor.ts` |

**Tests:** Xem `src/main/db/**/__tests__/` và `src/main/repositories/__tests__/`
