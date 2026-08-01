# BL-PRF-04 — Profile-Aware Agent Execution Routing

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-PRF-04 |
| **Tên** | Profile-Aware Agent Execution Routing |
| **Domain** | Profile Management / Agent Orchestration |
| **Actor** | Developer, Lead (Web Mode) |
| **Priority** | P0 |

---

## Mô tả

Khi user khởi động agent hoặc tạo worktree cho một project, hệ thống tự động: (1) resolve effective profile của user, (2) route đến đúng dev server của project, (3) inject profile environment vào agent session, (4) thêm project context vào prompt.

---

## Luồng chính: Khởi động Agent cho Project

```
User → Projects → vnp-blc-backend → [New Worktree + Start Agent]
    │
    ├── Step 1: Load Project
    │   project = ProjectService.get('vnp-blc-backend')
    │   → { devServerId: 'server-alpha', repoPath: '/srv/projects/vnp-blc' }
    │
    ├── Step 2: Check Server Availability
    │   server = DevServerManager.getStatus('server-alpha')
    │   → status: 'healthy' ✅  (or warn if degraded/unreachable)
    │
    ├── Step 3: Resolve Effective Profile
    │   profile = ProfileResolver.resolve(userId)
    │   → { agent: { preferredModel: 'claude-opus-4-5', trustPreset: 'standard' },
    │         shell: { pathAdditions: ['/usr/local/go/bin'],
    │                  envVars: { EDITOR: 'vim', NODE_ENV: 'development' } } }
    │
    ├── Step 4: Build Worktree on Dev Server
    │   relay.connect('server-alpha')
    │   relay.call('git.worktree.add', {
    │     basePath: '/srv/projects/vnp-blc',
    │     branch: 'feature/new-api',
    │     worktreePath: '/srv/projects/vnp-blc-worktrees/feature-new-api'
    │   })
    │
    ├── Step 5: Build Agent Environment
    │   agentEnv = {
    │     // From resolved profile shell.envVars
    │     EDITOR: 'vim',
    │     NODE_ENV: 'development',
    │     // PATH extension
    │     PATH: `/usr/local/go/bin:${existingPath}`,
    │     // Per-user credential isolation (GitHub)
    │     GH_CONFIG_DIR: `/home/dev/.config/gh/${userId}/`,
    │     GLAB_CONFIG_DIR: `/home/dev/.config/glab-cli/${userId}/`,
    │     // Agent model selection
    │     ANTHROPIC_MODEL: 'claude-opus-4-5',
    │     // Project context
    │     ORCA_PROJECT_ID: project.id,
    │     ORCA_PROJECT_NAME: project.name,
    │   }
    │
    ├── Step 6: Inject Project Context into Initial Prompt
    │   systemPreamble = buildProjectContext(project, user, worktree)
    │   → "You are working on project: vnp-blc-backend
    │       Repository: git@github.com:org/vnp-blc.git
    │       Working directory: /srv/projects/vnp-blc-worktrees/feature-new-api
    │       Branch: feature/new-api
    │       Team: Backend Team"
    │
    ├── Step 7: Spawn Agent on Dev Server
    │   relay.call('pty.spawn', {
    │     cmd: resolveAgentBinary(profile.agent.preferredModel),
    │     args: buildAgentArgs(profile.agent.trustPreset),
    │     cwd: worktree.path,
    │     env: agentEnv,
    │     // Pass system preamble via stdin/file
    │     initFile: systemPreamble
    │   })
    │
    └── Step 8: Stream PTY → WebSocket → Browser terminal
```

---

## Agent Binary Resolution

```typescript
function resolveAgentBinary(model: string): string {
  const AGENT_MAP: Record<string, string> = {
    'claude-opus-4-5': 'claude',
    'claude-sonnet': 'claude',
    'codex': 'codex',
    'gemini': 'gemini',
    'opencode': 'opencode',
  }
  return AGENT_MAP[model] ?? 'claude'  // fallback to claude
}

function buildAgentArgs(trustPreset: string): string[] {
  const TRUST_ARGS: Record<string, string[]> = {
    'minimal':    ['--trust', 'minimal'],
    'standard':   ['--trust', 'standard'],
    'permissive': ['--trust', 'full', '--dangerously-skip-permissions'],
  }
  return TRUST_ARGS[trustPreset] ?? ['--trust', 'standard']
}
```

---

## Project Context Preamble

```typescript
function buildProjectContext(
  project: OrcaProject,
  user: OrcaUser,
  worktree: Worktree
): string {
  return [
    `# Orca Project Context`,
    `Project: ${project.name}`,
    `Description: ${project.description ?? ''}`,
    `Repository: ${project.repoUrl}`,
    `Working directory: ${worktree.path}`,
    `Branch: ${worktree.branch}`,
    `Dev Server: ${project.devServerHostname}`,
    `Developer: ${user.name} (${user.email})`,
    `Team: ${user.departmentName ?? 'No team'}`,
    ``,
  ].join('\n')
}
```

---

## Server Unavailability Handling

```
IF server status === 'unreachable':
  → Show modal: "Dev Server for project 'vnp-blc-backend' is unreachable"
  → Options:
      [Retry] [Use different server] [Cancel]

IF server status === 'degraded':
  → Show toast warning: "Server is degraded (CPU 92%). Proceeding..."
  → Continue with spawn

IF relay connection dropped during session:
  → Auto-reconnect (exponential backoff, max 3 attempts)
  → PTY stream buffered during reconnect
  → Show "Reconnecting..." banner
```

---

## Tiêu chí chấp nhận

- [ ] Auto-route worktree creation đến `project.devServerId`
- [ ] Agent spawn với `project.repoPath` as cwd
- [ ] Effective profile injected vào agent environment (shell.envVars, PATH)
- [ ] `GH_CONFIG_DIR` / `GLAB_CONFIG_DIR` per-user isolation
- [ ] `ANTHROPIC_MODEL` env var từ `profile.agent.preferredModel`
- [ ] Agent trust preset từ `profile.agent.trustPreset`
- [ ] Project context preamble inject vào agent session
- [ ] Server unavailable: modal với retry option
- [ ] Server degraded: warning toast, continue
- [ ] Relay reconnect: buffer PTY + reconnect banner
