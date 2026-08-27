> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-003-14 — Orca: `PlannerPromptBuilder` + Worktree Automation + Tool Permission Baseline

**Phase:** 2 — song song, không chặn code Go
**Scope:** 🟠 **Orca TypeScript CONTRACT — KHÔNG thực thi trong repo `vnp-workplace`.** Code mới cần thêm vào repo `orca` (`/opt/repos/orca`): mở rộng `agent/src/shared/task-types.ts`, viết lại/mở rộng `backend/src/main/task/TaskAgentExecutor.ts`, thêm worktree automation mới.
**Source:** [SOL-ORCA-003 §2–§4, §9](../solutions/SOL-ORCA-003-orca-context-prompt-injection.md#2-field-mapping--nguồn-sự-thật-là-sol-orca-002-36)
**Depends On:** [TASK-ORCA-001-13](./TASK-ORCA-001-13-orca-planner-task-routes.md) (cần field `why_chain/anti_patterns/required_patterns/acceptance_criteria/worktree_repo/worktree_branch` lưu được vào `PlannerTaskRecord` trước)
**Người thực thi:** Orca team

---

## Vì sao task này tồn tại

`temporal-worker` (TASK-ORCA-002-04) gửi 4 field `WHYChain/AntiPatterns/RequiredPatterns/AcceptanceCriteria` trong mọi request `POST /api/planner-tasks` — các field này **chỉ có giá trị nếu Orca injects đúng và đầy đủ** vào system prompt của AI agent. Đây cũng là **security boundary**: `planner-service` (`vnp-workplace`) tin tưởng nhưng không kiểm chứng runtime được rằng Orca giới hạn quyền hạn agent đúng baseline khi chạy 2-8 giờ không giám sát.

> **Ghi chú re-scope (2026-08-10):** `WHYChain`/`RequiredPatterns` giờ do `TaskEnricher` của `planner-service` build (`why_chain` local từ DB `planner-service`, `required_patterns` gọi nội bộ `skills-service` :3008) — xem CR-TASK-003. `AntiPatterns` **hiện luôn gửi rỗng** vì `vnp-workplace` chưa có service tương đương `memory-svc`/lessons-learned của thiết kế gốc; Orca vẫn nên giữ nhánh xử lý field này (không xoá code), chỉ là nó sẽ không kích hoạt cho tới khi có CR bổ sung nguồn dữ liệu tương ứng.

---

## Bối cảnh thật — `buildPrompt()` hiện tại đơn giản hơn nhiều

`backend/src/main/task/TaskAgentExecutor.ts:114-131`:

```ts
buildPrompt(task: OrcaTask): string {
  if (task.promptTemplate) { return task.promptTemplate.replace(/\$\{task\.([^}]+)\}/g, ...) }
  const lines = [`# Task: ${task.title}`]
  if (task.description) lines.push(`\n## Description\n${task.description}`)
  if (task.aiContext)   lines.push(`\n## AI Context\n${task.aiContext}`)
  lines.push(`\n## Instructions`, `Complete the task described above. When finished, the task status will be moved to "review".`)
  return lines.join('\n')
}
```

Không có mục "Why This Task Exists", "Anti-Patterns", "Required Patterns", hay "Acceptance Criteria" dạng checklist — các field này **không tồn tại** trong `OrcaTask`/`CreateTaskParams` (`agent/src/shared/task-types.ts:39-80`). Migration `orca_tasks` hiện tại (migration 0010, xem `TaskService.ts:106-160`) cũng không có cột tương ứng.

**Worktree isolation** (SOL-ORCA-003 §3) và **tool permission baseline** (§4) **hoàn toàn chưa có cơ chế tương ứng** — `TaskAgentExecutor.executeTask()` nhận `worktreePath` như input đã tồn tại sẵn (không tự `git worktree add`), và không có allowlist/denylist lệnh shell theo nhãn task trong `ProfileAwareAgentSpawner`.

---

## Acceptance Criteria

- [ ] `OrcaTask`/`CreateTaskParams` mở rộng thêm field (đề xuất tên, Orca team có thể điều chỉnh miễn giữ đúng ý nghĩa): `plannerWhyChain?: string[]`, `plannerAntiPatterns?: string[]`, `plannerRequiredPatterns?: string[]`, `plannerAcceptanceCriteria?: string[]`
- [ ] `PlannerPromptBuilder.build(task)` sinh prompt gồm đủ các mục theo bảng field mapping (§2 dưới đây) — chỉ thêm mục khi mảng tương ứng non-empty, **trừ** Acceptance Criteria luôn bắt buộc xuất hiện nếu có
- [ ] `TaskAgentExecutor.buildPrompt()` gọi `PlannerPromptBuilder` khi task có label `planner:*` (hoặc field đánh dấu nguồn gốc từ planner), fallback về logic cũ cho task thường
- [ ] Agent **không được phép** báo `success: true` (task chuyển `review`/`done`) nếu chưa tick hết acceptance criteria — ghi rõ ràng buộc này trong system prompt (khoanh vùng, không thể enforce 100% runtime, nhưng phải yêu cầu rõ trong prompt)
- [ ] Worktree tạo mới (`git worktree add {path} -b planner/{planner_task_id} {branch}`) cho mọi task gắn nhãn `planner:*` khi `worktreePath` chưa được cung cấp sẵn
- [ ] Git commit identity riêng cho AI agent (không mạo danh user thật)
- [ ] Tool permission baseline áp dụng cho tiến trình PTY do `ProfileAwareAgentSpawner.spawn()` khởi tạo khi task gắn nhãn `planner:*` (allowlist auto-accept: đọc/ghi file trong worktree, `git add/commit`, `go build/test/vet`; denylist: `git push`, `curl`/`wget`, cài package hệ thống)

---

## §2 — Field mapping (nguồn: SOL-ORCA-002 §3.6, khoá cứng)

| Trường JSON (`PlannerTaskRecord`, TASK-ORCA-001-13) | Vai trò trong prompt | Bắt buộc |
|---|---|---|
| `title` + `description` | `# Task: {title}\n\n{description}` | Có |
| `why_chain[]` | `## Context: Why This Task Exists` (breadcrumb task→solution→CR→goal) | Nếu non-empty |
| `anti_patterns[]` | `## ⚠️ Anti-Patterns (DO NOT USE)` | Nếu non-empty |
| `required_patterns[]` | `## ✅ Required Patterns (MUST FOLLOW)` | Nếu non-empty |
| `acceptance_criteria[]` | `## Acceptance Criteria` (checklist `- [ ]`) — agent PHẢI tick hết trước khi báo `done` | **Bắt buộc** |
| `worktree_repo`+`worktree_branch` | Base cho `git worktree add` | Có |
| `agent_type` | Chọn executor | Có |
| `planner_cr_id` | Label `planner:{cr_id}`, không hiển thị trong prompt | Metadata only |

---

## Code mẫu tham khảo

### `backend/src/main/task/PlannerPromptBuilder.ts` [NEW]

```ts
/**
 * PlannerPromptBuilder — builds the AI agent system prompt for tasks
 * originating from vnp-workplace's planner-service (CR-ORCA-003 / SOL-ORCA-003 §2).
 *
 * Contract source of truth: backend/specs/crs/v1/orca/solutions/
 *   SOL-ORCA-003-orca-context-prompt-injection.md §2 (vnp-workplace repo)
 *
 * Only invoked for tasks whose OrcaTask.labels includes 'planner:*' — regular
 * (non-planner) tasks keep using TaskAgentExecutor's existing buildPrompt().
 */

export type PlannerPromptFields = {
  title: string
  description: string
  whyChain?: string[]
  antiPatterns?: string[]
  requiredPatterns?: string[]
  acceptanceCriteria?: string[]
}

export class PlannerPromptBuilder {
  build(fields: PlannerPromptFields): string {
    const lines: string[] = [`# Task: ${fields.title}`, '', fields.description]

    if (fields.whyChain?.length) {
      lines.push('', '## Context: Why This Task Exists')
      lines.push(...fields.whyChain.map((step, i) => `${i + 1}. ${step}`))
    }

    if (fields.antiPatterns?.length) {
      lines.push('', '## ⚠️ Anti-Patterns (DO NOT USE)')
      lines.push(...fields.antiPatterns.map((p) => `- ${p}`))
    }

    if (fields.requiredPatterns?.length) {
      lines.push('', '## ✅ Required Patterns (MUST FOLLOW)')
      lines.push(...fields.requiredPatterns.map((p) => `- ${p}`))
    }

    // Acceptance criteria is MANDATORY when present — the agent must not
    // self-report success without satisfying every item (SOL-ORCA-003 §2).
    if (fields.acceptanceCriteria?.length) {
      lines.push('', '## Acceptance Criteria', 'You MUST verify every item below before finishing:')
      lines.push(...fields.acceptanceCriteria.map((c) => `- [ ] ${c}`))
      lines.push(
        '',
        'Do NOT mark this task as complete / move it to review unless every acceptance ' +
          'criterion above is satisfied. If you cannot satisfy one, explain why in your final output.'
      )
    }

    lines.push('', '## Instructions', 'Complete the task described above.')
    return lines.join('\n')
  }
}
```

### `TaskAgentExecutor.buildPrompt()` [MODIFY]

```ts
// backend/src/main/task/TaskAgentExecutor.ts — extend the existing method
import { PlannerPromptBuilder } from './PlannerPromptBuilder'

const plannerPromptBuilder = new PlannerPromptBuilder()

buildPrompt(task: OrcaTask): string {
  if (task.labels?.some((l) => l.startsWith('planner:'))) {
    return plannerPromptBuilder.build({
      title: task.title,
      description: task.description ?? '',
      whyChain: task.plannerWhyChain,
      antiPatterns: task.plannerAntiPatterns,
      requiredPatterns: task.plannerRequiredPatterns,
      acceptanceCriteria: task.plannerAcceptanceCriteria
    })
  }
  // ... existing promptTemplate / aiContext fallback logic unchanged ...
  if (task.promptTemplate) { /* unchanged */ }
  const lines = [`# Task: ${task.title}`]
  if (task.description) lines.push(`\n## Description\n${task.description}`)
  if (task.aiContext) lines.push(`\n## AI Context\n${task.aiContext}`)
  lines.push(`\n## Instructions`, `Complete the task described above. When finished, the task status will be moved to "review".`)
  return lines.join('\n')
}
```

### Worktree automation — điểm tích hợp đề xuất

```ts
// backend/src/main/task/TaskAgentExecutor.ts — inside executeTask(), before step 4
// (status → in_progress), when params.worktreePath is not yet provisioned for
// a planner:* task:

import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
const execFileAsync = promisify(execFile)

async ensurePlannerWorktree(task: OrcaTask, repoUrl: string, branch: string, plannerTaskId: string): Promise<string> {
  const worktreePath = `/workspace/worktrees/${task.id}` // confirm real convention with Orca ops before hardcoding
  const featureBranch = `planner/${plannerTaskId}`
  await execFileAsync('git', ['worktree', 'add', worktreePath, '-b', featureBranch, branch], { cwd: repoRootFor(repoUrl) })
  // Set AI-specific git identity — do not impersonate the real user (SOL-ORCA-003 §3.3).
  await execFileAsync('git', ['-C', worktreePath, 'config', 'user.name', 'Orca AI Agent'])
  await execFileAsync('git', ['-C', worktreePath, 'config', 'user.email', 'orca-agent@noreply.internal'])
  return worktreePath
}
```

> Cleanup (`git worktree remove`) **chỉ** thực hiện sau khi `PlannerResultCollector` (TASK-ORCA-004-15) đã thu thập xong `files_created`/`files_modified`/`commit_hash` — cleanup sớm hơn sẽ làm rỗng sai kết quả (SOL-ORCA-003 §3 điểm 4).

### Tool permission baseline — điểm tích hợp đề xuất

```ts
// backend/src/main/project/ProfileAwareAgentSpawner.ts — extend spawn() options
// when the task carries a 'planner:*' label:

const PLANNER_TOOL_POLICY = {
  autoAccept: [/^git (add|commit) /, /^go (build|test|vet|generate)\b/],
  requireConfirmation: [/^git push\b/, /^(curl|wget)\b/, /^(apt|apt-get|brew) install\b/]
  // Destructive commands outside worktreePath remain blocked absolutely by the
  // sandbox layer, independent of this per-task policy (SOL-ORCA-003 §4 table,
  // last row) — do not attempt to re-implement that guarantee here.
}
```

---

## Verification (phía Orca team)

```bash
cd /opt/repos/orca
npm run build
npm test -- PlannerPromptBuilder
npm test -- TaskAgentExecutor

# Manual check: submit a planner:* task, confirm buildPrompt() output contains
# the "Acceptance Criteria" checklist section when acceptance_criteria is non-empty.
```
