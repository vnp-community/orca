# TDD-FE-08: Editor & File System

**Document:** TDD-FE-08  
**Domain:** Editor, File Explorer, Source Control  
**Source files:** `src/renderer/src/store/slices/editor.ts`, `src/renderer/src/components/editor/`

---

## 1. Tổng quan

Orca có **tích hợp code editor** cho phép mở, xem, và edit files trực tiếp trong app:

```
Editor Tab (trong Tab Bar)
  │
  ├─ Monaco Editor (hoặc custom diff viewer)
  ├─ File Explorer (tree view trong left sidebar)
  └─ Source Control Panel (right sidebar)
```

---

## 2. Editor Slice (`store/slices/editor.ts`) — ~176KB

File slice lớn nhất — quản lý toàn bộ editor state.

```typescript
type EditorSlice = {
  // Open files
  openFiles: OpenFileEntry[]
  activeFileByWorktree: Record<string, string | null>  // worktreeId → fileId
  editorScrollState: Record<string, EditorScrollState>

  // Diff views
  pendingDiffViews: DiffViewEntry[]
  activeDiffByWorktree: Record<string, string | null>

  // Generation state (AI-generated content)
  generationRecords: Record<string, GenerationRecord>

  // Actions
  openFile: (path: string, worktreeId: string, opts?: OpenFileOptions) => void
  closeFile: (fileId: string) => void
  closeAllFiles: (worktreeId: string) => void
  markFileDirty: (fileId: string, isDirty: boolean) => void
  openDiffView: (entry: DiffViewEntry) => void
  closeDiffView: (diffId: string) => void
}

type OpenFileEntry = {
  id: string           // unique per open instance
  path: string         // absolute path
  worktreeId: string
  isDirty: boolean     // unsaved changes
  language?: string    // detected language (TypeScript, Python, ...)
  displayName?: string // basename
  isReadOnly?: boolean
  scrollPosition?: EditorScrollState
}
```

---

## 3. Editor Component (`components/editor/`)

```typescript
// src/renderer/src/components/editor/
// Các components:

// MarkdownTemplatePicker.tsx — chọn template khi tạo file mới
// (Markdown templates: README, PR description, etc.)

// Thực tế: Orca không embed Monaco trực tiếp trong main app
// Thay vào đó: mở files trong terminal text editor (vim/nano/helix)
// HOẶC: forward tới external editor (VSCode, cursor, etc.)

// Editor integration:
// - OSC 7 tracking → biết file nào đang edit
// - File change watch → sync state khi editor saves
// - Git diff → show staged/unstaged changes
```

---

## 4. New Markdown Editor (`store/slices/new-markdown.ts`)

```typescript
// Cho editing markdown files (issue bodies, PR descriptions, etc.)
// Inline markdown editor (không phải terminal)

type NewMarkdownSlice = {
  newMarkdownDrafts: Record<string, NewMarkdownDraft>
  // Drafts survive navigation, auto-save to sessionStorage
}

// Components sử dụng:
// - LinearIssueMarkdownDescriptionEditor.tsx
// - LinearIssueMarkdownToolbar.tsx
// - JiraIssueWorkspace.tsx
// - GitHubItemDialog (PR description)
```

---

## 5. File Explorer

```typescript
// Không có dedicated file explorer component trong Orca
// Files được truy cập qua:
// 1. QuickOpen (Cmd+P) — fuzzy search files
// 2. WorktreeJumpPalette — jump + open files
// 3. Terminal: ls/cd/open commands
// 4. OSC link click trong terminal output

// Filesystem operations qua runtime-file-client.ts:
runtimeListDir(target, path)
runtimeReadFile(target, path)
runtimeWriteFile(target, path, data)
runtimeSearchFiles(target, args)
```

---

## 6. Source Control Panel

```typescript
// src/renderer/src/components/source-control/
// Right sidebar content:

// Git status:
// - Working tree changes (unstaged)
// - Staged changes
// - Stash list

// Actions:
// - Stage file: git add <file>
// - Unstage file: git restore --staged <file>
// - Commit: git commit -m "message"
//   (với AI commit message generation)
// - Push: git push
// - Pull: git fetch + merge

// useGitStatusPolling():
// Poll git status mỗi 5s khi right sidebar visible
// Pause khi tab hidden (Page Visibility API)
```

---

## 7. Diff Comments (`store/slices/diffComments.ts`) — ~19KB

```typescript
// Quản lý inline PR review comments

type DiffCommentsSlice = {
  diffCommentsByPr: Record<string, DiffComment[]>    // prUrl → comments
  pendingDiffComments: Record<string, DraftComment>  // draft edits
  expandedThreads: Set<string>                        // threadId set
}

type DiffComment = {
  id: string
  path: string          // file path
  line: number          // line number
  side: 'LEFT' | 'RIGHT'
  body: string          // markdown content
  author: GitHubUser
  createdAt: string
  reactions?: Reaction[]
  replyTo?: string      // parent comment id
}

// Components:
// - diff-comments/ dir → inline comment threads
// - DiffComments.tsx → render thread trong PullRequestPage diff
```

---

## 8. AI Commit Message Generation

```typescript
// src/renderer/src/store/slices/commit-message-generation.ts

type CommitMessageGenerationSlice = {
  generationByWorktree: Record<string, CommitMsgGeneration>
}

// Flow:
// 1. User opens commit panel
// 2. "Generate commit message" button
// 3. → callRuntimeRpc('git.generateCommitMessage', { worktreeId })
//    Backend: git diff → AI prompt → stream response
// 4. Streaming response updates store
// 5. User edits → commit

// AI context:
// - git diff --staged output
// - Convention detection từ COMMIT_CONVENTION file
// - Conventional Commits format
```

---

## 9. Pull Request Generation

```typescript
// src/renderer/src/store/slices/pull-request-generation.ts

type PullRequestGenerationSlice = {
  generationByWorktree: Record<string, PRGeneration>
  // Phase: idle | generating | done | error
}

// Flow:
// 1. User clicks "Create PR"
// 2. "Generate PR description" via AI
// 3. callRuntimeRpc('git.generatePullRequest', {
//      worktreeId,
//      baseBranch
//    })
// 4. Backend: git log + diff → AI → stream PR description
// 5. User reviews + submits PR via GitHub API
```

---

## 10. Sparse Checkout (`store/slices/sparse-presets.ts`)

```typescript
// src/renderer/src/store/slices/sparse-presets.ts (~8K)
// Git sparse checkout: chỉ checkout một phần của monorepo

type SparsePresetsSlice = {
  sparsePresets: Record<string, SparsePreset[]>   // repoId → presets
  activeSparsePreset: Record<string, string | null>
}

type SparsePreset = {
  id: string
  name: string         // "Backend only", "Frontend only"
  patterns: string[]   // ["src/backend/**", "!src/frontend/**"]
}

// Actions:
// applySparsePreset(repoId, presetId)
// → git sparse-checkout set <patterns>
// → refresh file explorer
```

---

## 11. Workspace Cleanup (`store/slices/workspace-cleanup.ts`) — ~32KB

```typescript
// Garbage collection cho old worktrees

type WorkspaceCleanupSlice = {
  scanState: 'idle' | 'scanning' | 'done'
  candidates: WorkspaceCleanupCandidate[]  // worktrees có thể xóa
  scanProgress: ScanProgress
  removalPreflight: RemovalPreflight | null
}

type WorkspaceCleanupCandidate = {
  worktreeId: string
  repoId: string
  path: string
  branch: string
  lastActivity: number          // last terminal activity
  gitStatus: 'clean' | 'dirty' | 'merged'  // merged branch → safe to delete
  diskUsage: number             // bytes
  isPrClosed: boolean           // GitHub PR closed → safe to delete
}

// UI: Settings → Workspace Cleanup
// → Scan → Show candidates → Confirm → Delete
```

---

## 12. Detected Agents (`store/slices/detected-agents.ts`)

```typescript
// src/renderer/src/store/slices/detected-agents.ts (~13K)
// Track which AI agents are running in terminals

type DetectedAgentsSlice = {
  detectedAgents: Record<string, DetectedAgent>  // ptyId → agent
}

type DetectedAgent = {
  ptyId: string
  worktreeId: string
  type: AgentType   // 'claude' | 'codex' | 'cursor' | 'droid' | 'gemini-cli' | ...
  detectedAt: number
  title: string     // terminal title qua which agent was detected
  status: AgentStatus
}

// Detection methods (từ terminal output):
// 1. Terminal title pattern matching
// 2. Custom ANSI/OSC sequences
// 3. Process name detection (via agent-hooks server)
// 4. stdin/stdout pattern scanning
```

---

## 13. Dictation (`store/slices/dictation.ts`)

```typescript
// Voice-to-text input
type DictationSlice = {
  isListening: boolean
  transcription: string
  targetPtyId: string | null   // destination terminal
}

// Flow:
// 1. User activates microphone (keyboard shortcut)
// 2. Web Audio API capture
// 3. → window.api.speech.transcribe(audioData)
// 4. Whisper model (sherpa-onnx, bundled)
// 5. Transcription → paste vào active terminal
```
