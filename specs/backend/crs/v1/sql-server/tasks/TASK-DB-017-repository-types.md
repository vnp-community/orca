# TASK-DB-017: Tạo `src/main/repositories/types.ts` — IStateRepository ✅ DONE

**Source:** SOL-DB-005 §4.1  
**Phase:** 3 | **Effort:** XS (< 20 min)   | **Status:** ✅ COMPLETED 2026-07-23  
**Depends on:** —

---

## Objective

Tạo `src/main/repositories/types.ts` — định nghĩa `IStateRepository` và các sub-interfaces (`IProjectRepository`, `IRepoRepository`, `ISshTargetRepository`, `IGlobalSettingsRepository`).

---

## Context cần đọc

- `src/shared/types.ts` (hoặc tương đương) để biết `Project`, `Repo`, `SshTarget`, `GlobalSettings` types
- SOL-DB-005 §4.1

---

## Files to create

### `src/main/repositories/types.ts`

```typescript
/**
 * State Repository Interfaces
 *
 * Defines the contract for state persistence regardless of backend
 * (JSON file, SQLite, MySQL, PostgreSQL).
 *
 * Implementations:
 * - JsonFileStateRepository — wraps JSON file store (default Electron mode)
 * - SqlStateRepository — SQL backend for server mode
 *
 * @module repositories/types
 */

// NOTE: Import paths may need adjustment based on actual shared types location
// Common locations: '../../shared/types', '../types/models', etc.
// Read existing imports in src/main/ to find the correct path.
import type { Project, Repo, SshTarget } from '../../shared/types'

/** Global application settings */
export interface GlobalSettings {
  theme?: string
  language?: string
  [key: string]: unknown
}

/**
 * Generic CRUD repository interface.
 * T = entity type, CreateInput = input for create (typically Omit<T, 'id'>),
 * UpdateInput = partial update.
 */
export interface IRepository<T, CreateInput = Omit<T, 'id'>, UpdateInput = Partial<T>> {
  findById(id: string): Promise<T | null>
  findAll(): Promise<T[]>
  create(input: CreateInput): Promise<T>
  update(id: string, input: UpdateInput): Promise<T>
  delete(id: string): Promise<void>
}

export interface IProjectRepository extends IRepository<Project> {
  findByGroup?(groupId: string): Promise<Project[]>
}

export interface IRepoRepository extends IRepository<Repo> {
  findByProject?(projectId: string): Promise<Repo[]>
}

export interface ISshTargetRepository extends IRepository<SshTarget> {}

export interface IGlobalSettingsRepository {
  get(): Promise<GlobalSettings>
  update(patch: Partial<GlobalSettings>): Promise<GlobalSettings>
}

/** The unified state repository contract */
export interface IStateRepository {
  readonly projects: IProjectRepository
  readonly repos: IRepoRepository
  readonly sshTargets: ISshTargetRepository
  readonly settings: IGlobalSettingsRepository
  /** Check connectivity — returns true if healthy */
  ping(): Promise<boolean>
  /** Release resources (flush writes, close connections) */
  close(): Promise<void>
}
```

---

## QUAN TRỌNG

Trước khi tạo file, hãy đọc file types trong `src/shared/` để tìm đúng path cho `Project`, `Repo`, `SshTarget`:

```bash
find src/shared src/main -name "*.ts" | xargs grep -l "export.*interface Project" 2>/dev/null | head -5
```

Nếu types không tồn tại theo paths trên, điều chỉnh import path cho phù hợp với codebase thực tế.

---

## Verification

```bash
npx tsc --noEmit 2>&1 | grep "repositories/types" | head -5
```

---

## Done criteria

- [x] `src/main/repositories/types.ts` tồn tại
- [x] Export `IStateRepository`, `IProjectRepository`, `IRepoRepository`, `ISshTargetRepository`, `IGlobalSettingsRepository`
- [x] Export `GlobalSettings` (hoặc re-export từ shared types nếu có)
- [x] `IRepository<T>` generic với đúng constraints
- [x] Không có `any`
- [x] TypeScript compile OK
