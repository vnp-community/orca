# BL-PW-04 — Workspace Integration (Agent + Git + Tasks + Workflows)

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-PW-04 |
| **Tên** | Workspace Integration — Cross-panel Data Flow |
| **Domain** | Project Workspace |
| **Actor** | Developer, Lead |
| **Priority** | P0 |

---

## Mô tả

Các panels trong Project Workspace không hoạt động độc lập — chúng chia sẻ state và trigger nhau. BL này đặc tả các **cross-panel integration flows** quan trọng.

---

## Integration Flow 1: Agent → Git → Tasks

```
Agent hoàn thành tác vụ
    │
    ├── Agent tab: "✅ Done — 3 files modified"
    │
    ├── Auto-refresh Git tab:
    │   - git status poll triggers immediately (không đợi 5s)
    │   - Git changes badge update: "● 3 changes"
    │
    ├── IF task was attached to agent session:
    │   Task.status → 'review'
    │   Tasks panel badge update
    │
    ├── Explorer tab:
    │   - File tree git decorations refresh
    │   - Modified files highlighted
    │
    └── Notification (if user on different tab):
        "Agent complete. 3 files changed → Git tab"
        [Go to Git]
```

---

## Integration Flow 2: Git Commit → Task Status

```
User commits: "feat(auth): implement bcrypt..."
    │
    ├── IF commit message references task ID (#TG-123):
    │   auto-detect via regex: /#(TG-\w+)/
    │   TaskService.update(taskId, { status: 'done' })
    │   TaskService.recordActualHours(taskId, agentSession.duration)
    │
    ├── Parent task progress recalculate
    │
    └── Push completed → IF PR created:
        Task.prUrl = PR.url
        Task activity: "PR #42 created"
```

---

## Integration Flow 3: Workflow Step → Git

```
Workflow step "create-pr" completes
    │
    ├── Git tab: sync remote refs (auto-fetch after push step)
    ├── Show new remote branch in Branch Manager
    └── IF PR created: show notification + link
```

---

## Integration Flow 4: Task → Agent → Workspace

```
User: Task Detail → [Run Agent]
    │
    ├── WorkspaceContext.currentWorktree = task.worktreeId (or create new)
    │
    ├── Agent tab gets focus
    │
    ├── Prompt editor pre-filled với task.promptTemplate
    │
    ├── [Run Agent] auto-triggered (or user confirms)
    │
    └── Agent stream → Agent tab output panel
        Explorer git decorations → real-time update during agent
```

---

## Shared State Architecture

```typescript
// WorkspaceContext (React Context) — shared across all panels
const WorkspaceContext = createContext<WorkspaceContextValue>({
  project,          // used by: all panels
  relay,            // used by: Explorer, Git, Agent, Terminal
  resolvedProfile,  // used by: Agent panel (provider, model, env)
  gitStatus,        // used by: Git panel, Explorer decorations, header badge
  currentWorktree,  // used by: Git panel, Agent panel, Terminal cwd
  setCurrentWorktree, // used by: Worktree switcher
  activeAgentSession, // used by: Agent panel, Tasks panel
  setActiveAgentSession,
})

// Event bus (within workspace):
workspaceEvents.on('agent.complete', () => {
  refreshGitStatus()
  refreshExplorer()
  checkLinkedTaskUpdate()
})

workspaceEvents.on('git.commit', (commitHash) => {
  checkTaskReferenceInMessage(commitHash)
  refreshBranchInfo()
})

workspaceEvents.on('worktree.switched', (worktreePath) => {
  reloadGitStatus(worktreePath)
  reloadExplorer(worktreePath)
  updateTerminalCwd(worktreePath)
})
```

---

## Tiêu chí chấp nhận

- [ ] Agent complete → auto-refresh Git tab + Explorer decorations
- [ ] Agent complete → linked task advance to 'review'
- [ ] Commit message task ID reference → auto-close task
- [ ] Worktree switch → all panels update (Git/Explorer/Terminal cwd)
- [ ] WorkspaceContext shared: relay, gitStatus, worktree, profile
- [ ] Event bus: agent.complete, git.commit, worktree.switched
- [ ] Notification cross-tab khi relevant event happens
