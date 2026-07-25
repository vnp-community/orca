# TDD-08: Agent Orchestration

**Document:** TDD-08  
**Domain:** Multi-Agent Orchestration — Coordinator, Task DAG, Message Protocol  
**Source files:** `src/main/runtime/orchestration/`, `src/main/ipc/agent-hooks.ts`

---

## 1. Tổng quan

Orca có hệ thống **orchestration** cho phép điều phối nhiều AI agents làm việc song song trên các worktrees khác nhau:

```
Coordinator Agent (terminal A)
  │
  │ orchestration.send(to: "worker-1", type: "dispatch", task: "...")
  │ orchestration.send(to: "worker-2", type: "dispatch", task: "...")
  │
  ├──→ Worker Agent 1 (terminal B, worktree X)
  │       │  orchestration.check(condition: "status")
  │       │  orchestration.send(to: coordinator, type: "worker_done")
  │
  └──→ Worker Agent 2 (terminal C, worktree Y)
          │  orchestration.check(condition: "status")
          │  orchestration.send(to: coordinator, type: "merge_ready")
```

---

## 2. OrchestrationDb Schema (SQLite v6)

```sql
-- Orchestration dùng SQLite riêng (không phải store.json)
-- Tại: ~/.config/orca/orchestration.db

-- Messages: inter-agent messaging
CREATE TABLE messages (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  from_handle TEXT,
  to_handle TEXT NOT NULL,
  subject TEXT NOT NULL,
  body TEXT,
  type TEXT NOT NULL CHECK (type IN (
    'status', 'dispatch', 'worker_done', 'merge_ready',
    'escalation', 'handoff', 'decision_gate', 'heartbeat'
  )),
  priority TEXT NOT NULL DEFAULT 'normal' CHECK (priority IN ('normal', 'high', 'urgent')),
  thread_id TEXT,
  payload TEXT,         -- JSON
  created_at INTEGER NOT NULL,
  read INTEGER NOT NULL DEFAULT 0,
  delivered_at INTEGER, -- v3: khi message được poll
  sender_pane_key TEXT  -- v6: stable pane identity của sender
);

-- Tasks: work items trong orchestration run
CREATE TABLE tasks (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  spec TEXT NOT NULL,           -- task description cho agent
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN (
    'pending', 'ready', 'dispatched', 'completed', 'failed', 'blocked'
  )),
  created_by_handle TEXT,       -- v4: terminal handle tạo task
  task_title TEXT,              -- v5: display title
  display_name TEXT,            -- v5: human-readable name
  priority INTEGER DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

-- Dispatch contexts: track assignment của task → agent
CREATE TABLE dispatch_contexts (
  id TEXT PRIMARY KEY,
  task_id TEXT NOT NULL REFERENCES tasks(id),
  terminal_handle TEXT NOT NULL,
  assignee_pane_key TEXT,       -- v6: stable pane identity
  dispatched_at INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'active'
);

-- Decision gates: blocking points chờ coordinator decision
CREATE TABLE decision_gates (
  id TEXT PRIMARY KEY,
  run_id TEXT NOT NULL,
  question TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  answer TEXT,
  created_at INTEGER NOT NULL
);

-- Coordinator runs: metadata của mỗi orchestration run
CREATE TABLE coordinator_runs (
  id TEXT PRIMARY KEY,
  coordinator_handle TEXT NOT NULL,
  spec TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'running',
  started_at INTEGER NOT NULL,
  last_heartbeat_at INTEGER,   -- v2
  completed_at INTEGER
);
```

---

## 3. Message Types

```typescript
type MessageType =
  | 'status'        // agent báo cáo trạng thái
  | 'dispatch'      // coordinator giao việc cho worker
  | 'worker_done'   // worker báo hoàn thành
  | 'merge_ready'   // worker sẵn sàng merge
  | 'escalation'    // worker cần help từ coordinator
  | 'handoff'       // chuyển task cho agent khác
  | 'decision_gate' // blocking question cho coordinator
  | 'heartbeat'     // liveness signal

type MessagePriority = 'normal' | 'high' | 'urgent'
```

---

## 4. RPC Methods (`orchestration.ts`)

### 4.1 orchestration.send

```typescript
// Agent gửi message tới agent khác
params: {
  to: string           // terminal handle hoặc group address
  subject: string      // tiêu đề message
  from?: string        // sender handle (optional)
  body?: string        // nội dung text
  type?: MessageType
  priority?: 'normal' | 'high' | 'urgent'
  threadId?: string    // grouping messages
  payload?: string     // JSON payload
  senderPaneKey?: string  // stable identity (chống remint)
}
```

### 4.2 orchestration.check

```typescript
// Long-poll: chờ condition được thỏa mãn
params: {
  handle?: string      // terminal handle để filter
  condition?: string   // 'status' | 'any' | 'unread' | ...
  timeoutMs?: number   // max wait time
  worktree?: string    // filter theo worktree
}

// Server giữ request mở đến khi có message hoặc timeout
// Keepalive frames mỗi 10s (xem TDD-04)
```

### 4.3 orchestration.list

```typescript
// Lấy danh sách messages/tasks
params: {
  runId?: string
  handle?: string
  limit?: number
  includeRead?: boolean
}
```

### 4.4 orchestration.create-task

```typescript
// Tạo task trong orchestration DAG
params: {
  spec: string         // task description
  runId?: string
  title?: string
  worktreeSelector?: string
}
```

---

## 5. Coordinator (`coordinator.ts`)

```typescript
class Coordinator {
  // Phases:
  type Phase = 'decomposing' | 'dispatching' | 'monitoring' | 'merging' | 'done'

  async run(options: CoordinatorOptions): Promise<void> {
    // 1. Decompose spec thành tasks
    await this.decomposeSpec()

    // 2. Dispatch tasks tới worker agents
    await this.dispatchTasks()

    // 3. Monitor progress
    await this.monitorWorkers()

    // 4. Handle merge khi workers done
    await this.coordinateMerge()
  }

  private async decomposeSpec(): Promise<void> {
    // Gửi toàn bộ spec vào coordinator terminal
    // Agent (Claude/Codex) phân tích và tạo tasks
    await this.runtime.sendTerminalAgentPrompt(
      this.coordinatorHandle,
      buildDispatchPreamble({ spec: this.spec })
    )

    // Poll database cho đến khi tasks được tạo
    await this.pollUntil(() => this.db.hasPendingTasks())
  }

  private async dispatchTasks(): Promise<void> {
    const tasks = this.db.getReadyTasks()
    // Dispatch concurrently (up to maxConcurrent)
    const pending = tasks.slice(0, this.maxConcurrent)

    await Promise.all(pending.map(async task => {
      // Tạo worker terminal
      const worker = await this.runtime.createTerminal(this.worktreeSelector)

      // Gửi task spec vào terminal
      await this.runtime.sendTerminalAgentPrompt(
        worker.handle,
        formatDispatchMessage(task.spec)
      )

      // Record dispatch
      this.db.updateTaskStatus(task.id, 'dispatched')
      this.db.createDispatchContext(task.id, worker.handle)
    }))
  }
}
```

---

## 6. Stale Base Detection

```typescript
// Trước khi dispatch task, check nếu worktree đã stale:
const DISPATCH_STALE_THRESHOLD = 20  // commits behind

async probeWorktreeDrift(worktreeSelector: string): Promise<DriftResult | null> {
  // git fetch → count commits behind base
  // Nếu > 20 commits → warn coordinator
  // Nếu task có 'allow-stale-base: true' → bypass check
}
```

---

## 7. Group Addresses

```typescript
// src/main/runtime/orchestration/groups.ts
// Group addressing: gửi tới nhiều agents

isGroupAddress('all')             // → true
isGroupAddress('@workers')        // → true  
isGroupAddress('terminal-123')    // → false (concrete handle)

resolveGroupAddress('@workers', db)
// → danh sách terminal handles của tất cả workers trong run
```

---

## 8. Agent Hooks (`src/main/ipc/agent-hooks.ts`)

Agent hooks là **event hooks** chạy khi agent (Claude, Codex, etc.) thực hiện actions:

```typescript
// src/main/ipc/agent-hooks.ts (~11K)
// Hooks fire khi agent:
// - Bắt đầu/kết thúc session
// - Tạo/xóa file
// - Chạy command
// - Error

type AgentHookEvent =
  | 'session:start'
  | 'session:end'
  | 'file:create'
  | 'file:update'
  | 'file:delete'
  | 'command:run'
  | 'error'

// Registered hooks từ orca.yaml:
// hooks:
//   session:start:
//     - run: ./scripts/setup-dev.sh
//   file:update:
//     - run: prettier --write $ORCA_FILE_PATH
```

### Hook Server

```typescript
// src/main/agent-hooks/server.ts
// Unix socket server nhận events từ agent CLI tools
// (claude_code_hooks, codex hooks)

class AgentHookServer {
  start(): void  // lắng nghe trên ORCA_HOOK_SOCKET env var
  stop(): void

  on('session:start', handler)
  on('file:update', handler)
  // ...
}
```

---

## 9. AI Vault (`src/main/ipc/ai-vault.ts`)

Secure storage cho AI provider credentials:

```typescript
// src/main/ipc/ai-vault.ts (~11K)
// Quản lý API keys cho Claude, OpenAI, Gemini, Grok, etc.
// Sử dụng Electron safeStorage (OS keychain)

class AiVault {
  async setKey(provider: AiProvider, key: string): Promise<void> {
    const encrypted = safeStorage.encryptString(key)
    await this.store.setAiKey(provider, encrypted.toString('base64'))
  }

  async getKey(provider: AiProvider): Promise<string | null> {
    const encrypted = await this.store.getAiKey(provider)
    if (!encrypted) return null
    return safeStorage.decryptString(Buffer.from(encrypted, 'base64'))
  }

  // Providers:
  // 'claude' | 'openai' | 'gemini' | 'grok' | 'codex' | 'minimax' | 'opencode'
}
```

---

## 10. Automation System (`src/main/ipc/automations.ts`)

```typescript
// Automations: scheduled/triggered agent runs
type AutomationTrigger =
  | { type: 'manual' }
  | { type: 'schedule'; cron: string }
  | { type: 'event'; event: string }

// Automation runs được persist với output snapshots
// Pruning: giữ tối đa N runs per automation
// Chạy qua OrcaRuntimeService.runAutomation()
```
