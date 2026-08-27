# Electron/Desktop Persistence Store (`persistence.ts`, `persistence-paths.ts`, `persistence-migration.ts`)

Đây là backend lưu trữ **mặc định** khi chạy Orca như ứng dụng Electron desktop (không phải server mode).
Toàn bộ dữ liệu người dùng — Project/Repo/SSH/Worktree/Tab/Terminal/Browser/Automation/Settings/UI state —
nằm trong **1 file JSON lớn**, không đụng tới `db/**`/SQL nào.

## 1. Vị trí file trên đĩa

`persistence-paths.ts`:
- `userDataDir = process.env.ORCA_DATA_DIR ?? app.getPath('userData')` — chốt 1 lần lúc khởi động (trước khi
  `app.setName('Orca')` có thể đổi path casing).
- `_dataFile = join(userDataDir, 'orca-data.json')` — file chính, được `getDataFile()` expose cho `persistence.ts`.
- `getCanonicalUserDataPath()` — expose thư mục gốc cho các subsystem khác (mobile pairing, credential files...).

**Không phải 1 file duy nhất** — cùng thư mục còn có:

| File | Vai trò |
|---|---|
| `orca-data.json` | State chính (`PersistedState`, xem §2) |
| `orca-data.json.bak.0` … `.bak.4` | 5 bản backup xoay vòng (`BACKUP_COUNT=5`), dùng để phục hồi khi file chính hỏng (`restoreFromBackup()`) |
| `orca-github-cache.json` | Cache PR/issue GitHub (TTL 5 phút) — tách riêng **có chủ đích** để refresh cache không phá hash-skip-write guard của file chính |
| `orca-devices.json` | Device registry cho mobile pairing |
| `orca-e2ee-keypair.json` | Keypair E2EE cho mobile companion |
| `orca-claude-usage.json`, `orca-codex-usage.json` | Usage tracking riêng — xem [07](./07-usage-tracking-stores.md) |
| `~/.orca/openai-speech-token.enc` | API key OpenAI cho speech-to-text, mã hoá `safeStorage` |

Ghi file: atomic (tmp file + rename), có content-hash để **skip ghi nếu không đổi** — vì vậy cache GitHub phải
tách file riêng (nếu không, refresh cache 5 phút/lần sẽ liên tục "đánh thức" ghi lại toàn bộ state lớn).

## 2. Shape đầy đủ của `PersistedState` (`shared/types.ts:3634`)

```ts
type PersistedState = {
  schemaVersion: number                      // SCHEMA_VERSION=1, KHÔNG tăng bao giờ — vestigial, không dùng để gate logic
  repos: Repo[]
  projects: Project[]
  projectHostSetups: ProjectHostSetup[]
  projectGroups: ProjectGroup[]
  folderWorkspaces: FolderWorkspace[]
  sparsePresetsByRepo: Record<string, SparsePreset[]>
  worktreeMeta: Record<string, WorktreeMeta>
  worktreeLineageById: Record<string, WorktreeLineage>
  workspaceLineageByChildKey: Record<WorkspaceKey, WorkspaceLineage>
  settings: GlobalSettings                    // xem 06, ~230 field
  ui: PersistedUIState                        // xem 06, ~90 field (bao gồm windowBounds/windowMaximized)
  githubCache: { pr: Record<...>, issue: Record<...> }   // KHÔNG ghi vào orca-data.json — ghi ra sidecar riêng
  workspaceSession: WorkspaceSessionState      // session state host 'local' (legacy single-blob)
  workspaceSessionsByHostId?: Partial<Record<ExecutionHostId, WorkspaceSessionState>>
  sshTargets: SshTarget[]
  deletedSshConfigAliases: string[]
  removedSshTargetTombstones?: RemovedSshTargetTombstone[]
  sshRemotePtyLeases: SshRemotePtyLease[]
  claudeLivePtySessionIds?: string[]
  migrationUnsupportedPtyEntries: MigrationUnsupportedPtyEntry[]
  legacyPaneKeyAliasEntries: LegacyPaneKeyAliasEntry[]
  automations: Automation[]                   // "WorkflowTemplate" tương đương ở desktop mode
  automationRuns: AutomationRun[]              // "WorkflowExecution" tương đương ở desktop mode
  onboarding: OnboardingState
  featureInteractionTelemetryBuckets?: FeatureInteractionTelemetryBucketState
  devServers?: PersistedDevServer[]
  vapidKeys?: { publicKey: string; privateKey: string } | null   // KHÔNG mã hoá privateKey — xem §5
  webPushSubscriptions?: WebPushSubscription[]
}
```

Xem [06-shared-domain-types.md](./06-shared-domain-types.md) cho chi tiết field-level từng entity con
(`Project`, `Repo`, `Worktree`, `Tab`, `Automation`, `GlobalSettings`...).

## 3. Quan hệ với `JsonFileStateRepository` (server mode)

**Hoàn toàn tách biệt** — không phải quan hệ đọc chung 1 file:

- `repositories/json-file-repository.ts` tự ghi rõ trong doc comment: *"NOT used in Electron/desktop mode
  (which uses the full persistence.ts store)"*.
- `JsonState` (backend cho `JsonFileStateRepository`) chỉ có 4 field: `{projects, repos, sshTargets,
  globalSettings: Partial<GlobalSettings>}` — tập con rất nhỏ của ~30 field `PersistedState`.
- Ghi vào `dataFile` được truyền qua constructor (thường là `store.json` ở server mode, **không phải**
  `orca-data.json`), debounce 200ms — cơ chế atomic-write/backup/hash-skip của `persistence.ts` **không áp
  dụng** ở đây.

Về mặt khái niệm, `PersistedState` là superset của 4 field đó, nhưng đây là **2 implementation khác nhau chạy
ở 2 run mode loại trừ lẫn nhau** (desktop vs. server-không-có-DB-config), không phải 1 đọc từ cái kia.

## 4. `persistence-migration.ts` — KHÔNG phải hệ migration theo version

Tên gây hiểu lầm — đây **không** phải migration engine kiểu `switch(schemaVersion)` như `db/migrations/**`.
2 helper độc lập:

1. `migrateMobilePairingDataToCanonicalUserDataPath()` — copy 1 lần các file mobile-pairing (device registry +
   E2EE keypair) từ userData path cũ sang path canonical, chỉ khi file nguồn tồn tại và file đích chưa có; sau
   đó `hardenExistingSecureFile()` siết lại quyền file.
2. `sanitizeOnboardingUpdate()` — sanitize/merge partial update cho onboarding wizard state, remap số
   `lastCompletedStep` cũ khi số bước wizard thay đổi qua các version (v2/v3/pre-v2).

**Migration schema thật** cho `PersistedState` nằm **inline trong `Store.load()`** (`persistence.ts`) — hàng
chục hàm normalize ad-hoc kiểm tra field thiếu/sai-shape rồi coerce forward mỗi lần load (vd.
`migrateTerminalScrollbackRows`, `migrateAgentYoloDefaults`, `migrateOnboardingChecklist`...). Không có
`switch(schemaVersion)` — `schemaVersion` gần như vestigial (luôn = 1, không tăng), migration thực chất chạy
theo kiểu "normalize-on-load" dựa trên sự hiện diện/hình dạng field, không theo version number.

## 5. Mã hoá secret trong `orca-data.json`

Hỗn hợp — 1 vài field mã hoá tại chỗ bằng `safeStorage`, phần lớn credential thật nằm **ngoài** file này
(xem [05-credential-secret-stores.md](./05-credential-secret-stores.md) để đầy đủ):

| Field | Mã hoá? |
|---|---|
| `settings.opencodeSessionCookie`, `settings.httpProxyUrl`, `ui.browserKagiSessionLink` | ✅ `safeStorage.encryptString`/`decryptString` tại boundary save/load; nếu decrypt fail (vd. keychain bị reset) → fallback coi là plaintext + log cảnh báo |
| `vapidKeys.privateKey` | ❌ Plaintext — có chủ đích, đây là key ký Web Push (không phải credential người dùng) |
| `SshTarget.identityFile`/`identityAgent` | Chỉ lưu **đường dẫn**, không lưu key material |
| `SshTarget.lastRequiredPassphrase` | Chỉ là boolean flag, không lưu passphrase |

## 6. Các store Electron khác nằm ngoài `orca-data.json`

- **Window bounds** — thực ra nằm **trong** `orca-data.json` (`ui.windowBounds`/`ui.windowMaximized`), không
  phải file riêng.
- **GitHub PR/issue cache**, **backup rotation**, **mobile pairing (device registry + E2EE keypair)**,
  **OpenAI speech key**, **per-integration credential files** (Linear/Jira/Bitbucket/Azure DevOps/Gitea),
  **Claude/Codex CLI OAuth** (OS Keychain) — xem [05](./05-credential-secret-stores.md).
- **AI Vault session cache** (`desktop/src/main/ai-vault/*`) — scan/cache transcript session của các CLI agent
  khác (Claude/Codex/Gemini...) — đọc dữ liệu của tool ngoài, không phải store nghiệp vụ của Orca.
- Không tìm thấy store "recent files" hay "telemetry event log" riêng — trường gần nhất là
  `featureInteractionTelemetryBuckets` bên trong `PersistedState`, chỉ là de-dupe marker do main process quản lý.
