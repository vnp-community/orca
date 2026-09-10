# CR-WF-006 — Frontend: Mount Workflow Builder, Xây Library/Sharing UI, Pause/Resume, Sửa Step-Type Vocabulary

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WF-006 |
| **Tên** | Đóng gap frontend chính của F36 — **Workflow Template Library/Sharing UI mới** (gap ma trận hoàn thành đã nêu), mount Builder, Pause/Resume, sửa vocabulary lệch |
| **Loại** | Feature |
| **Priority** | P0 (Library/Sharing UI) / P1 (phần còn lại) |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-09-09 |
| **Trạng thái** | 🔵 Proposed |
| **Phụ thuộc** | [CR-WF-005](./CR-WF-005-template-sharing-library-and-list-executions.md) (Library UI cần RPC sharing/search); Pause/Resume + step-type fix KHÔNG phụ thuộc gì, có thể ship sớm hơn độc lập |
| **Áp dụng** | `specs/frontend/bugs/workflow-orchestration/BUG-FE-WF-001-workflow-builder-ui-not-implemented.md`, `specs/frontend/bugs/task-v1/BUG-FE-TASKV1-007-workflow-builder-rpc-shape-mismatch.md` |
| **Tác động** | `frontend/src/renderer/src/components/workflow/*` (file hiện có + file mới), `WorkspaceLayout.tsx`, `types/workflow-types.ts` |

---

## 1. Vấn đề

### C1 — `WorkflowBuilder.tsx` tồn tại nhưng không mount ở đâu

`grep -rn "WorkflowBuilder"` (loại trừ test) chỉ khớp định nghĩa component
và 2 comment nhắc tới nó — **không có import/render site nào**.
`WorkspaceLayout.tsx:21-22,93` chỉ lazy-import và render `WorkflowMonitor`
cho tab `'workflows'` — không có route/action "New Workflow" nào dẫn tới
Builder. Chính hook của Builder tự xác nhận: `useWorkflow.ts:65-69` —
*"a live bug, just invisible because `WorkflowBuilder.tsx` (this hook's only
caller) isn't mounted anywhere in the app yet."* Đây là điều chỉnh so với
`BUG-FE-WF-001`'s khung ban đầu ("file không tồn tại") — file **có tồn tại**,
vấn đề thật là **không thể truy cập được**.

### C2 — Không có Workflow Template Library/Sharing UI ở đâu cả

Xác nhận trực tiếp: không component library/browse/share nào trong
`components/workflow/`. Đây chính là gap mà
[`docs/roadmap/feature-completion-matrix.md`](../../../roadmap/feature-completion-matrix.md)
gọi tên cho F36 ("thiếu Workflow Template Library/Sharing UI").

### C3 — `ExecutionMonitor.tsx` không có Pause/Resume dù backend đã hỗ trợ đầy đủ

`ExecutionMonitor.tsx:39-43` chỉ có nút Cancel. `workflow.pause`/
`workflow.resume` đã hoạt động thật ở backend-go (`PauseExecution`/
`ResumeExecution` real, wire đủ wscompat) — đây là gap frontend thuần, không
cần chờ backend, có thể ship sớm nhất trong series này.

### C5 — Step-type vocabulary lệch: `'notify'|'approval'` (FE) vs `'notification'`-only (BE)

`frontend/src/renderer/src/types/workflow-types.ts:3` —
`WorkflowStepType = 'agent' | 'shell' | 'notify' | 'approval'`. Backend-go's
`StepType` (`step.go:16-23`) là `agent|shell|notification|webhook|condition`
— không có `notify`, không có `approval` nào tương đương. `StepEditor.tsx`
cho chọn `approval` trong dropdown dù backend không thể chạy loại step này.

## 2. Giải pháp đề xuất

### 2.1 Mount `WorkflowBuilder`

```tsx
// WorkspaceLayout.tsx — thêm action "New Workflow" cạnh tab Workflows hiện có
<Button onClick={() => setActiveView('workflow-builder')}>+ New Workflow</Button>
{activeView === 'workflow-builder' && <WorkflowBuilder projectId={projectId} onSave={() => setActiveView('workflows')} />}
```

Sửa `useWorkflow.ts`'s RPC call theo shape backend-go mới nhất từ
CR-WF-003/004/005 (`inputs`, `overrides`/`injectSteps`, `visibility`) khi các
CR đó merge.

### 2.2 **Workflow Template Library (mới)**

```tsx
// WorkflowLibrary.tsx (MỚI)
// 3 tab: Company Standards / Team Templates / My Workflows — theo spec F36
// dòng 184-207. Mỗi card: name, description, tags, rating, usageCount,
// action Use/Preview/Clone/Share.
export function WorkflowLibrary({ projectId }: WorkflowLibraryProps) {
  const [tab, setTab] = useState<'company' | 'team' | 'personal'>('company')
  const { templates, search, setSearch } = useWorkflowLibrary(tab) // hook mới, gọi task.searchTemplates (CR-WF-005)
  return (
    <div>
      <SearchBar value={search} onChange={setSearch} />
      <Tabs value={tab} onChange={setTab}>...</Tabs>
      <div className="grid grid-cols-3 gap-3">
        {templates.map(t => <WorkflowTemplateCard key={t.id} template={t} onUse={...} onPreview={...} onClone={...} onShare={...} />)}
      </div>
    </div>
  )
}
```

`WorkflowTemplateCard`'s "Share" action mở dialog nhỏ: toggle visibility +
nút "Generate share link" (gọi CR-WF-005's RPC) + Copy button.

Dùng token màu/spacing theo [`docs/STYLEGUIDE.md`](../../../STYLEGUIDE.md),
tái sử dụng `Card`/`Badge`/`Tabs` primitives đã có ở
`frontend/src/renderer/src/components/ui/` — không tự vẽ component cơ bản
mới.

### 2.3 Pause/Resume buttons

```tsx
// ExecutionMonitor.tsx — thêm cạnh nút Cancel hiện có
{execution.status === 'running' && (
  <Button size="sm" variant="outline" onClick={pauseExecution} data-testid="pause-btn">Pause</Button>
)}
{execution.status === 'paused' && (
  <Button size="sm" variant="outline" onClick={resumeExecution} data-testid="resume-btn">Resume</Button>
)}
```

```tsx
// hooks/useWorkflowExecution.ts — thêm 2 hàm gọi RPC đã có sẵn ở backend
const pauseExecution = () => callRuntimeRpc(target, 'workflow.pause', { executionId })
const resumeExecution = () => callRuntimeRpc(target, 'workflow.resume', { executionId })
```

### 2.4 Sửa step-type vocabulary

```ts
// workflow-types.ts
export type WorkflowStepType = 'agent' | 'shell' | 'notification' | 'webhook' | 'condition' | 'action' | 'parallel'
// bỏ 'notify' (đổi tên thành 'notification' cho khớp backend)
// bỏ 'approval' — không có backend tương đương; nếu sản phẩm cần approval-gate
// thật, đó là 1 tính năng mới cần CR riêng (có thể map vào orchestration-
// service's Gate mechanism đã có ở docs/crs/v4/task-graph/CR-TG-004), không
// tự chế 1 step type không ai thực thi được.
```

`StepEditor.tsx`'s dropdown cập nhật theo enum mới, thêm option cho `action`/
`parallel` (CR-WF-003) khi 2 step type đó merge ở backend.

## 3. Rủi ro / Không thuộc phạm vi

- Loại bỏ `'approval'` khỏi vocabulary là quyết định sản phẩm (không chỉ kỹ
  thuật) — cần xác nhận với team trước khi xoá, vì có thể đang có người dùng
  kỳ vọng tính năng này dù chưa hoạt động thật.
- Library UI's phần "Team Templates"/"Company Standards" filter theo
  `visibility`/`scope` — không tự thiết kế lại RBAC hiển thị, dựa hoàn toàn
  vào field backend trả về (CR-WF-005).
- Không thuộc phạm vi: real-time step output trong `ExecutionMonitor` — vẫn
  dùng polling 4s hiện có cho tới khi CR-WF-007 (streaming) hoàn thành.

## Acceptance Criteria

- [ ] `WorkflowBuilder` truy cập được từ UI thật (không chỉ tồn tại trong
      source) — có 1 đường dẫn rõ ràng "+ New Workflow".
- [ ] **Library UI mới** hiển thị đủ 3 tab, search/filter hoạt động, action
      Use/Preview/Clone/Share gọi đúng RPC tương ứng.
- [ ] Share dialog tạo được share-link, Copy button hoạt động; import từ
      share-link tạo đúng 1 bản Clone (không sửa được template gốc).
- [ ] Pause/Resume button hiển thị đúng theo `execution.status`, gọi đúng
      RPC, cập nhật UI ngay khi backend xác nhận (không chờ tới lần poll kế
      tiếp).
- [ ] `WorkflowStepType` khớp 1-1 với backend `StepType` enum (kể cả sau khi
      CR-WF-003 thêm `action`/`parallel`); không còn giá trị FE-only nào
      không map được sang backend.
- [ ] Toàn bộ UI mới tuân theo [`docs/STYLEGUIDE.md`](../../../STYLEGUIDE.md).
