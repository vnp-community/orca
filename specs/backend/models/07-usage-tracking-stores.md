# Usage Tracking Stores — `orca-claude-usage.json` / `orca-codex-usage.json`

2 kho JSON **độc lập** (không phải bảng SQL, không nằm trong `orca-data.json`), mỗi kho theo dõi usage
token/cost của 1 CLI agent, cùng pattern lưu trữ:

- File: `join(app.getPath('userData'), 'orca-claude-usage.json')` /
  `'orca-codex-usage.json'` (`claude-usage/store.ts`, `codex-usage/store.ts`).
- Ghi atomic: tmp file + rename (`writeFileSync` tmp → `renameSync`).
- Có `schemaVersion` + `normalizePersistedState()` để migrate khi tăng version (khác cách "vestigial" của
  `PersistedState.schemaVersion` ở [03](./03-electron-desktop-store.md) §4 — ở đây version **có** dùng để gate
  logic normalize).

## `ClaudeUsagePersistedState` (`claude-usage/types.ts`)

```ts
type ClaudeUsagePersistedState = {
  schemaVersion: number
  worktreeFingerprint: string | null
  processedFiles: ClaudeUsagePersistedFile[]   // cache incremental-scan theo file transcript đã xử lý
  sessions: ClaudeUsageSession[]
  dailyAggregates: ClaudeUsageDailyAggregate[]
  scanState: { enabled, lastScanStartedAt, lastScanCompletedAt, lastScanError }
}
```

- **`ClaudeUsageSession`** — `{sessionId, firstTimestamp, lastTimestamp, model, lastCwd, lastGitBranch,
  primaryWorktreeId, primaryRepoId, turnCount, totalInputTokens/OutputTokens/CacheReadTokens/CacheWriteTokens,
  locationBreakdown: ClaudeUsageLocationBreakdown[]}` — 1 phiên chat, gộp usage theo từng vị trí
  (project/repo/worktree) phiên đó chạy qua.
- **`ClaudeUsageDailyAggregate`** — `{day, model, projectKey, projectLabel, repoId, worktreeId, turnCount,
  zeroCacheReadTurnCount, inputTokens/outputTokens/cacheReadTokens/cacheWriteTokens}` — rollup theo ngày,
  phục vụ dashboard usage.
- **`ClaudeUsagePersistedFile`** (`= ClaudeUsageProcessedFile & {sessions, dailyAggregates, ownedDedupeKeys,
  hasDeferredClaims}`) — cache incremental scan: mỗi file transcript đã parse có `ownedDedupeKeys` (khoá
  `message.id:requestId`) để đảm bảo 1 turn chỉ được đếm đúng 1 lần dù session bị fork/resume copy lại turn cũ
  vào file mới; `hasDeferredClaims=true` đánh dấu file này có turn tạm nhường quyền sở hữu cho file khác — khi
  file "chủ" biến mất, chỉ cần re-parse các file deferred thay vì toàn bộ transcript corpus.

`CodexUsagePersistedState` (`codex-usage/types.ts:99`) — cùng gia đình type, cùng pattern JSON-file +
atomic-write + schemaVersion, khác domain (Codex CLI thay vì Claude Code CLI).

## Luồng nghiệp vụ

Quét transcript CLI (`claude-usage/scanner.ts`/`codex-usage/scanner.ts`) → parse turn → gán (attribute) vào
project/repo/worktree hiện hành → gộp theo session và theo ngày → phục vụ dashboard usage/cost trong app và
usage attribution cho `AutomationService` (mỗi `AutomationRun.usage` tham chiếu ngược lại 2 store này qua
`provider: claude|codex`).
