# BL-TG-02 — AI-Assisted Task Planning & Decomposition

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-TG-02 |
| **Tên** | AI-Assisted Task Planning & Decomposition |
| **Domain** | Task Graph |
| **Actor** | Developer, Lead |
| **Priority** | P0 |

---

## Mô tả

AI (ưu tiên model mạnh như Claude Opus) phân tích một task và đề xuất cách chia nhỏ thành subtasks có cấu trúc, dependency edges, estimates, và prompt templates cho từng subtask.

---

## Luồng: AI Decompose Task

```
User → Task Detail → [AI: Plan this task]
    │
    ├── Collect context:
    │   - task.title + task.description + task.aiContext
    │   - project.name + project.repoUrl
    │   - tech stack (từ dev server: package.json, go.mod, pom.xml...)
    │   - recent completed tasks in project (velocity data)
    │   - existing subtasks (if any, to avoid duplicates)
    │
    ├── Build planning prompt:
    │   """
    │   You are a senior tech lead planning a software task.
    │
    │   Task: {{task.title}}
    │   Description: {{task.description}}
    │   Project: {{project.name}} ({{tech_stack}})
    │   Context: {{task.aiContext}}
    │
    │   Break this task into concrete, implementable subtasks.
    │   For each subtask provide:
    │   - title (action verb + what)
    │   - type: story|task|subtask|bug|spike
    │   - estimated_hours: realistic number
    │   - depends_on: list of subtask titles this needs first
    │   - prompt_template: ready-to-use prompt for AI coding agent
    │
    │   Return JSON: { subtasks: [...], dependencies: [...], notes: string }
    │   """
    │
    ├── Spawn AI call:
    │   provider = resolveProviderAccount({
    │     devServerId: project.devServerId,
    │     provider: 'anthropic',  // prefer most capable
    │     model: 'claude-opus-4-5'
    │   })
    │   result = await relay.call('ai.complete', { provider, prompt, max_tokens: 4096 })
    │
    ├── Parse + validate JSON response
    │
    ├── Show AI Plan Modal to user:
    │   - Subtask list với checkboxes (chọn accept/reject từng subtask)
    │   - Dependency graph preview
    │   - Total estimate display
    │   - Edit trực tiếp trong modal
    │
    ├── User clicks "Accept Selected":
    │   - INSERT accepted subtasks → orca_tasks (parentId = current task)
    │   - INSERT dependency edges → orca_task_edges
    │   - UPDATE task.aiPlanJson = raw AI response
    │
    └── Task graph updates với subtasks mới
```

---

## Luồng: AI Generate Agent Prompt

```
User → Task Detail → [Generate Agent Prompt]
    │
    ├── AI nhận:
    │   - task.title, task.description, task.aiContext
    │   - parent task context (if subtask)
    │   - project tech stack
    │   - relevant file structure từ dev server (optional)
    │
    ├── AI generates ready-to-use agent prompt:
    │   """
    │   Implement bcrypt password hashing in the auth module.
    │
    │   File to modify: src/auth/auth-manager.ts
    │   Function signature: hashPassword(plain: string): Promise<string>
    │   Use bcrypt with 12 rounds.
    │
    │   Also add unit tests in src/auth/auth-manager.test.ts
    │   covering: hash generation, verify correct, verify incorrect.
    │
    │   Follow the existing patterns in the file.
    │   Do not modify the function signature.
    │   """
    │
    ├── Hiển thị trong prompt editor (editable)
    │
    └── User có thể edit + save vào task.promptTemplate
```

---

## Luồng: Critical Path Analysis

```typescript
function calculateCriticalPath(tasks: OrcaTask[], edges: TaskEdge[]): string[] {
  // Build adjacency: task → dependencies
  // For each task: earliestStart = max(earliestEnd of all dependencies)
  // earliestEnd = earliestStart + estimatedHours
  // Critical path = longest path through DAG

  const topoOrder = topologicalSort(tasks, edges)
  const earliestEnd: Record<string, number> = {}

  for (const taskId of topoOrder) {
    const task = tasks.find(t => t.id === taskId)!
    const deps = edges
      .filter(e => e.fromTaskId === taskId && e.edgeType === 'depends_on')
      .map(e => earliestEnd[e.toTaskId] ?? 0)
    const earliestStart = Math.max(0, ...deps)
    earliestEnd[taskId] = earliestStart + (task.estimatedHours ?? 0)
  }

  // Trace back from task with max earliestEnd
  const maxEnd = Math.max(...Object.values(earliestEnd))
  // ... backtrack to find critical path tasks
  return criticalPathTaskIds
}
```

---

## Context Collection từ Dev Server

```typescript
async function collectProjectContext(projectId: string): Promise<string> {
  const project = await ProjectService.get(projectId)
  const relay = DevServerManager.getRelay(project.devServerId)

  // Check tech stack markers
  const files = await Promise.allSettled([
    relay.call('fs.readFile', { path: `${project.repoPath}/package.json` }),
    relay.call('fs.readFile', { path: `${project.repoPath}/go.mod` }),
    relay.call('fs.readFile', { path: `${project.repoPath}/pom.xml` }),
    relay.call('fs.readFile', { path: `${project.repoPath}/pyproject.toml` }),
    relay.call('fs.readDir', { path: `${project.repoPath}/src`, depth: 2 }),
  ])

  // Build tech stack summary
  return buildTechStackSummary(files)
  // e.g. "Node.js 22 / TypeScript / Express / Prisma / PostgreSQL"
  //      "top-level dirs: src/routes, src/models, src/services, tests/"
}
```

---

## Tiêu chí chấp nhận

- [ ] AI decompose: gửi task + project context → nhận JSON subtask suggestions
- [ ] AI plan modal: checkbox per subtask, dependency preview, edit inline
- [ ] Accept selected → INSERT subtasks + edges vào graph
- [ ] AI prompt generation từ task context → editable, saveable
- [ ] Tech stack detection từ dev server files
- [ ] Critical path calculation từ DAG + estimates
- [ ] Error handling: AI timeout, invalid JSON response → show raw + retry
