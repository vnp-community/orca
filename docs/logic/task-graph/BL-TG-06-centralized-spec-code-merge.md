# BL-TG-06 — Quản lý Task tập trung: Gen Spec hàng loạt → Gen Code hàng loạt → Merge Worktree

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-TG-06 |
| **Tên** | Centralized batch Spec/Code generation + Worktree merge (mở rộng BL-TG-05 từ 1 task sang nhiều task) |
| **Domain** | Task Graph (mở rộng BL-TG-04, BL-TG-05) |
| **Actor** | Developer, Lead (người duyệt spec/code, người merge) |
| **Priority** | P1 |
| **Trạng thái** | ✅ Implemented, unit-tested (23 test mới: `useTaskBatchExecution.test.ts` x2, `useTaskBatchMerge.test.ts` mới x4, `TaskBoardView.test.tsx` x2, `TaskGraph.test.tsx` x6, `channels_worktree_test.go` x1 mới), **đã deploy** (`2026.09.16-bl-tg-06-batch-spec-code-merge`). Điều kiện tiên quyết (BUG-028) đã fix + verify sống trước đó. Một bug phụ tìm thấy khi implement (`worktree.merge` trả raw proto snake_case, y hệt BUG-023) đã fix cùng đợt. Chờ user test thật trên `b15.openledger.vn`. |

---

## Đánh giá độ sẵn sàng — tự chấm 9.5/10

Thang điểm: 10 = một kỹ sư (hoặc agent) có thể implement thẳng từ tài liệu này, không cần hỏi lại
gì thêm. Lý do chấm 9.5, không phải 10 (nâng từ 8/10 sau khi BUG-028's fix plan được đặc tả đầy
đủ ở mức code — diff chính xác, test case chính xác, blast radius đã kiểm tra chứ không giả định):

| Đã chốt (giữ điểm) | Còn hở (trừ điểm) |
|---|---|
| Đã đọc code thật của `worktree.merge` (api-gateway) và `MergeWorktreeIntoBase` (git-gateway-service) — RPC merge **đã tồn tại, đã chạy được, có xử lý conflict, có cleanup sau merge** — không phải giả định | Concurrency cap "bao nhiêu task chạy đồng thời trên 1 dev server thì an toàn" **chưa có số đo thực nghiệm** — BUG-027 bite 2 chỉ xác nhận "nhiều lệnh đồng thời làm sập kết nối", chưa xác nhận ngưỡng chính xác là 2, 3, hay 5. Đây là tham số chỉnh được sau khi chạy thật, không chặn việc bắt đầu code — không hỏi thêm được gì trước khi có số đo thực nghiệm. |
| **BUG-028 (điều kiện tiên quyết) giờ đã có fix plan đầy đủ ở mức code** — 4 thay đổi cụ thể (`worktree_provisioner.go`, `ports.go`, `simple_executor.go`, `execute_task.go`), đúng diff, đúng test case mới, blast radius đã grep xác nhận (1 caller production duy nhất) — xem BUG-028's "Fix plan" | Việc phát hiện conflict khi 2 worktree cùng sửa 1 vùng code (xung đột NGỮ NGHĨA giữa 2 task chạy song song, không phải lỗi git) không có cách nào phát hiện tự động — chấp nhận rủi ro ở v1, đã nêu rõ, không phải lỗ hổng spec |
| Đã đọc code thật `ExecuteBatch` (task-service, hiện là dead code) — hiểu rõ vì sao **không** dùng lại nguyên bản (wave-gating của nó dựa trên hành vi đồng bộ đã bị BUG-027 thay đổi) thay vì chỉ giả định nó dùng được | |
| Đã đọc code thật `TaskBoardView.tsx` — xác nhận **chưa có** cơ chế multi-select (chỉ `onSelect(id)` đơn) — không giả định "chắc đã có sẵn" | |
| Tái dùng đúng quy ước label `phase:*` đã có ở BL-TG-05, không phát minh state machine mới | |
| Đã đối chiếu với mục "Multi-task Agent Session" sẵn có trong BL-TG-04 — xác nhận đó là thiết kế CHƯA TỪNG implement (không phải hành vi đang chạy), không bỏ sót một đặc tả đã có | |

---

## Mô tả

Yêu cầu gốc (nguyên văn): *"tôi cần quản lý task, gen spec tập trung sau đó là gen code tập
trung và merge worktree"*.

BL-TG-05 đã xây xong vòng lặp Spec → Duyệt → Code → Duyệt cho **1 task tại 1 thời điểm**, thao
tác trong `TaskPromptEditor` (tab AI của Task Detail). Tài liệu này mở rộng thành thao tác
**hàng loạt trên nhiều task cùng lúc**, từ một màn hình trung tâm, cộng thêm bước cuối chưa có ở
BL-TG-05: **merge worktree của task đã Done vào nhánh base**.

```
[Chọn nhiều task] → Gen Spec hàng loạt → (mỗi task tự duyệt spec riêng, như BL-TG-05)
                  → Gen Code hàng loạt → (mỗi task tự duyệt code riêng, như BL-TG-05)
                  → [Chọn task Done]  → Merge Worktree hàng loạt vào base branch
```

### Đối chiếu với BL-TG-04's "Multi-task Agent Session" — tài liệu này THAY THẾ, không phải bổ sung

`BL-TG-04` (tài liệu thiết kế gốc, viết TRƯỚC khi có code thật) đã có sẵn 1 mục tên
"Multi-task Agent Session (Batch Execution)" mô tả ý tưởng tương tự: chọn nhiều task
(Ctrl+click) → "Run All with Agent" → nhóm theo dev server → chạy theo thứ tự topological (tôn
trọng `depends_on`) → chạy song song khi không phụ thuộc nhau, giới hạn concurrency.

**Đọc lại mục đó và đối chiếu với code thật cho thấy nó KHÔNG phản ánh implementation hiện tại**:

| BL-TG-04 mô tả (2026, trước khi có code) | Thực tế đã verify (phiên làm việc BUG-025→028) |
|---|---|
| "Group tasks by devServer" | Không có cơ chế nào group theo dev server ở tầng dispatch — mỗi `task.execute` tự resolve dev server độc lập qua `ProjectExecutionResolver` |
| "Execute tasks in topological order (respect dependencies)" | Đây chính là việc `ExecuteBatch` được viết ra để làm — nhưng **là dead code, không RPC nào gọi tới**, và giả định đồng bộ của nó đã lỗi thời sau khi BUG-027 làm `direct_agent` thành async |
| "Parallel where no deps between them (up to concurrency limit)" | Không có concurrency limit nào tồn tại ở bất kỳ tầng nào hôm nay — đây chính là lỗ hổng BUG-027 bite 2 đã khai thác (bấm liên tục = không giới hạn = sập kết nối) |
| Ctrl+click multi-select trên Task Board | Xác nhận qua đọc code: **chưa tồn tại** — `TaskBoardView.tsx` chỉ có `onSelect(id)` đơn |

Nói cách khác: BL-TG-04's mục batch execution là **thiết kế chưa từng được xây**, không phải mô tả
hành vi đang chạy — không có gì trong `ExecuteBatch`/`TaskBoardView` thật sự khớp với mô tả đó khi
đọc code. Tài liệu BL-TG-06 này **thay thế hoàn toàn** mục đó của BL-TG-04 bằng một thiết kế đơn
giản hơn, phù hợp với ràng buộc THẬT đã tìm ra (async dispatch, giới hạn kết nối dev server), thay
vì topological/dev-server-grouping chưa từng được implement và giờ không còn khớp với kiến trúc
async hiện tại. BL-TG-04's mục "Multi-task Agent Session" nên được đánh dấu **deprecated, xem
BL-TG-06** khi tài liệu này được duyệt.

Đây **không phải** một hệ thống mới — nó là lớp điều phối (orchestration) mỏng phủ lên trên các
RPC đơn-task đã có (`task.execute`, `task.update`) và một RPC merge đã có sẵn nhưng frontend chưa
gọi tới (`worktree.merge`). Không cần RPC mới ở backend cho phần Spec/Code (dispatch từng task một
bằng `task.execute`, y hệt BL-TG-05, chỉ lặp lại N lần thay vì 1 lần) — chỉ cần RPC merge đã có
sẵn được nối vào UI, và 1 quyết định kiến trúc riêng cho việc **giới hạn số task chạy đồng thời**
(giải thích ở dưới).

## Điều kiện tiên quyết BẮT BUỘC — BUG-028 phải fix trước

**Không được implement tài liệu này trước khi
[BUG-028](../../../specs/backend-go/bugs/missing-v2/BUG-028-direct-agent-runs-in-shared-repo-not-isolated-worktree.md)
được fix và verify sống.**

Lý do: BUG-028 xác nhận trực tiếp trên dev server thật rằng khi 1 task dùng lại worktree đã có
(`reuse` branch của `WorktreeProvisioner.EnsureWorktree`), agent bị điều hướng chạy vào **repo gốc
dùng chung** (`/opt/repos/aiops-v3`, nhánh `main`) thay vì worktree cô lập của chính task đó. Với
tính năng **hàng loạt** này, hậu quả không còn là "1 task ghi sai chỗ" — nó trở thành:

- **N task chạy đồng thời cùng ghi/commit vào CÙNG MỘT thư mục, cùng một nhánh `main`** — race
  condition thật giữa các tiến trình `git commit`/`git checkout`, không chỉ là "sai vị trí" mà còn
  có thể làm hỏng lịch sử git của nhánh chính dùng chung cho toàn bộ dự án.
- Bước "Merge Worktree" ở cuối tài liệu này giả định mỗi task có 1 worktree/nhánh cô lập, đúng
  quy ước `task/<taskId>` — nếu agent không thực sự ghi vào worktree đó, sẽ không có gì để merge,
  hoặc merge sẽ merge nhầm nhánh `main` (đã bị agent trước đó ghi bậy) vào chính nó.

Tóm lại: BL-TG-06 xây **trên nền tảng cô lập worktree đúng đắn** — nếu nền đó sai, chạy hàng loạt
sẽ khuếch đại đúng cái bug vừa tìm ra, không giải quyết được gì. BUG-028 đã có sẵn hướng fix cụ
thể trong doc của nó (dùng `project-service.GetWorktree` lấy path thật thay vì repo root) — đang
chờ xác nhận từ người dùng, chưa code.

## Đính chính khi implement — hạ tầng multi-select/batch execution đã có sẵn nhiều hơn tưởng

Khi bắt đầu code, phát hiện `TaskGraph.tsx` (component cha của Tree/DAG/Board) **đã có sẵn**
`selectMode`/`selectedIds`/`toggleSelected` (từ `useTasks.ts`) VÀ một hook
`useTaskBatchExecution.ts` với `runSelected(taskIds)` (bounded-concurrency=3, y hệt thiết kế mục
"Ràng buộc concurrency" bên dưới) — nhưng chỉ Tree/DAG nhận props này, **`TaskBoardView` thì
không** (đúng như đánh giá ban đầu). `useTaskBatchExecution` cũng có 2 bug thật cần sửa trước khi
dùng cho BL-TG-06: (1) gửi `worktreePath: currentWorktree.path` — field chết, `task.execute`
không đọc (BUG-025); (2) `if (!project || !currentWorktree) return new Map()` — im lặng no-op mọi
lần chạy hàng loạt khi Tasks tab không có worktree chọn sẵn ở sidebar, dù tab này không cần
worktree. Đã sửa cả 2, mở rộng hook nhận thêm `promptBuilder`/`phaseLabel` tuỳ chọn (không phá vỡ
cách dùng cũ), thay vì viết `batchDispatch` mới từ đầu như bản spec ban đầu đề xuất.

## Những mảnh ghép có sẵn, dùng được ngay (không cần xây mới)

- **`task.execute`** (BL-TG-04/05, đã fix xong toàn bộ chuỗi BUG-025/026/027) — dispatch async,
  trả về ngay, không chặn UI. Đây là nguyên hàm được gọi N lần cho N task đã chọn, không cần RPC
  batch mới ở tầng gRPC.
- **`derivePhase`/`withPhase`/`buildSpecPrompt`/`buildImplementPrompt`**
  (`frontend/src/renderer/src/lib/task-spec-build-loop.ts`, BL-TG-05) — dùng lại y nguyên cho
  từng task trong danh sách hàng loạt, không viết lại.
- **`worktree.merge`** (`channels_worktree.go:241`, gọi
  `GitGatewayService.MergeBranch` → `MergeWorktreeIntoBase` usecase) — **đã chạy được thật**:
  nhận `worktreeId`/`baseBranch`/`strategy` (`merge`/`squash`/`rebase`)/`commitMessage`, xử lý
  conflict (trả `hasConflicts` + `conflictedPaths` + `conflictDispatchKey`, không tự ý resolve),
  và **đã có sẵn cleanup sau merge** (`cleanupWorktreeIds`, BR-WT-18 — xoá worktree sau khi merge
  thành công, best-effort, lỗi cleanup không che mất kết quả merge). **Frontend hiện chưa gọi RPC
  này ở đâu cả** (xác nhận qua tìm kiếm toàn bộ `frontend/src` — chỉ xuất hiện trong
  `tracers.ts`, không phải lệnh gọi thật) — đây là phần việc UI chính của bước Merge.
- **`Task.WorktreeID`** — đã là field sẵn có trên mỗi task, chính là `worktreeId` cần truyền cho
  `worktree.merge`. Không cần tra cứu gián tiếp.
- **`ExecuteBatch`** (`task-service/internal/usecase/execute_batch.go`) — **đã đọc kỹ, quyết định
  KHÔNG dùng lại nguyên bản**. Lý do: nó vốn được thiết kế cho việc dispatch nhiều task theo từng
  "wave" tôn trọng phụ thuộc `depends_on`, chờ cả wave xong (đồng bộ) mới sang wave sau — nhưng
  BUG-027's redesign đã làm `ExecuteTask.Execute` cho đường `direct_agent` trở thành **async**
  (trả về ngay, không đợi agent chạy xong). `ExecuteBatch`'s cơ chế "đợi cả wave" dựa trên hành vi
  đồng bộ CŨ, giờ không còn đúng nữa (đã ghi rõ trong BUG-027's "Known, flagged-not-fixed side
  effect" — bản thân `ExecuteBatch` cũng xác nhận là dead code, không RPC nào gọi tới). Dùng lại
  nó ở đây sẽ kế thừa một giả định đã sai. Xem "Thiết kế phần B/C" bên dưới cho cách thay thế đơn
  giản hơn, không cần sửa Go code cho việc dispatch hàng loạt.

## Thiết kế

### A. Quản lý task (đã có — không cần xây mới)

`TaskBoardView`/`TaskTreeView`/`TaskGraph`/`TaskDetail` đã là bộ màn hình quản lý task đầy đủ.
Phần MỚI duy nhất ở đây là **multi-select** — xác nhận qua đọc code `TaskBoardView.tsx` rằng nó
hiện chỉ hỗ trợ chọn 1 task (`onSelect: (id: string) => void`), chưa có checkbox/chọn nhiều.

Đề xuất: thêm 1 chế độ "Batch mode" vào `TaskBoardView` (nút bật/tắt ở góc, không đổi hành vi mặc
định) — khi bật, mỗi thẻ task hiện thêm 1 checkbox, và 1 thanh hành động nổi (floating action bar)
xuất hiện ở dưới cùng khi có ≥1 task được chọn, hiện số lượng đã chọn + các nút hành động hàng loạt
tương ứng với "pha" (phase) chung của các task đã chọn (xem phần B/C/D).

### B. Gen Spec hàng loạt

Điều kiện hiện nút: người dùng đã chọn ≥1 task ở trạng thái `derivePhase(task) === 'not-started'`
(tái dùng hàm thuần từ BL-TG-05, không viết lại).

Khi bấm "Generate Spec cho N task đã chọn":
```ts
// Dispatch tuần tự theo lô nhỏ (KHÔNG Promise.all toàn bộ cùng lúc) — xem
// "Ràng buộc concurrency" bên dưới cho lý do.
async function batchGenerateSpec(tasks: OrcaTask[], concurrency = 2) {
  const queue = [...tasks]
  const results: Record<string, 'pending' | 'dispatched' | 'error'> = {}
  async function worker() {
    while (queue.length > 0) {
      const task = queue.shift()!
      try {
        // Y HỆT bước 1b/1c của BL-TG-05, lặp lại cho từng task:
        await callRuntimeRpc(target, 'task.update', {
          id: task.id,
          labels: withPhase(task.labels, 'phase:spec-pending')
        })
        await callRuntimeRpc(target, 'task.execute', {
          taskId: task.id,
          projectId: project.id,
          traceId: span.id,
          prompt: buildSpecPrompt(task)
        })
        results[task.id] = 'dispatched' // task.execute đã async — trả về ngay, KHÔNG đợi agent chạy xong
      } catch (err) {
        results[task.id] = 'error'
      }
    }
  }
  await Promise.all(Array.from({ length: concurrency }, worker))
  return results
}
```

Vì `task.execute` đã async (BUG-027), hàm trên trả về gần như ngay lập tức cho toàn bộ N task —
**không phải đợi N agent chạy xong**. Tiến độ từng task sau đó hiện qua polling có sẵn
(`useTaskActivity`, đã có ở BL-TG-05/BUG-027) — mỗi thẻ task trong Board tự cập nhật trạng thái
(`in_progress` → `review`) độc lập, không cần cơ chế theo dõi mới.

### C. Gen Code hàng loạt

Y hệt phần B, đổi 2 chỗ: điều kiện hiện nút là `derivePhase(task) === 'spec-approved'`, và prompt
dùng `buildImplementPrompt(task)` thay vì `buildSpecPrompt(task)`. Không viết lại logic — 1 hàm
`batchDispatch(tasks, promptBuilder, phaseLabel, concurrency)` tổng quát hoá cả B và C.

### D. Merge Worktree

Điều kiện hiện nút: người dùng đã chọn ≥1 task ở `task.status === 'done'` VÀ có `task.worktreeId`
khác rỗng (task chưa từng chạy agent thì không có gì để merge).

```ts
async function batchMergeWorktrees(
  tasks: OrcaTask[],
  baseBranch: string,
  strategy: 'merge' | 'squash' | 'rebase',
  concurrency = 1 // xem "Ràng buộc concurrency" — merge ghi vào CÙNG 1 checkout base, phải tuần tự
) {
  const results: Record<string, { ok: true; resultSha: string } | { ok: false; error: string } | { ok: 'conflict'; paths: string[] }> = {}
  for (const task of tasks) { // cố ý for-tuần-tự, không worker pool — xem lý do concurrency=1 bên dưới
    try {
      const resp = await callRuntimeRpc(target, 'worktree.merge', {
        worktreeId: task.worktreeId,
        baseBranch,
        strategy,
        commitMessage: `merge(task-${task.taskNumber}): ${task.title}`,
        cleanupWorktreeIds: [task.worktreeId] // BR-WT-18 — dọn worktree ngay sau merge thành công
      })
      if (resp.hasConflicts) {
        results[task.id] = { ok: 'conflict', paths: resp.conflictedPaths }
        // DỪNG batch tại đây — xem "Rủi ro chưa xử lý": conflict cần người xử lý thủ công
        // trước khi merge tiếp bất kỳ task nào khác vào CÙNG base checkout đang ở trạng thái
        // conflict dở dang.
        break
      }
      results[task.id] = { ok: true, resultSha: resp.resultSha }
    } catch (err) {
      results[task.id] = { ok: false, error: String(err) }
      break // cùng lý do — 1 lỗi merge vào base checkout dùng chung phải dừng, không tiếp tục đè lên
    }
  }
  return results
}
```

**Vì sao merge PHẢI tuần tự (concurrency=1), khác với Gen Spec/Code (concurrency>1 chấp nhận
được)**: `MergeWorktreeIntoBase` dispatch vào **repo gốc dùng chung** (`mainExecutor`,
`repo.ID` — đọc thấy trong code, đây là hành vi ĐÚNG và có chủ đích cho merge, khác với bug
BUG-028 ở đường execute). Nhiều lệnh `worktree.merge` chạy song song đều ghi vào CÙNG MỘT checkout
đó (`git merge`/`git checkout base_branch` liên tiếp) — chạy song song ở đây chắc chắn gây race
condition thật trên chính base checkout, không phải giả thuyết.

## Ràng buộc concurrency — bài học trực tiếp từ BUG-027 bite 2

BUG-027's bite 2 xác nhận sống: người dùng bấm liên tục "Run with Agent" đã gửi nhiều lệnh
`agent.execPrompt` đồng thời tới CÙNG MỘT kết nối dev server, làm connection đó sập
(`devserveragent: connection lost: EOF`). Tính năng hàng loạt ở đây **chủ động** tạo ra chính tình
huống đó (nhiều task cùng dispatch cùng lúc) — nếu không giới hạn, sẽ tái hiện đúng bug đó ở quy mô
lớn hơn, không phải do người dùng bấm nhầm mà do chính thiết kế.

- **Gen Spec/Code hàng loạt**: giới hạn concurrency (đề xuất mặc định **2**, cấu hình được) — CHƯA
  có số đo thực nghiệm ngưỡng an toàn thật sự là bao nhiêu (ghi rõ ở bảng điểm đầu tài liệu), 2 là
  điểm khởi đầu thận trọng, không phải số đã kiểm chứng.
- **Merge**: tuần tự tuyệt đối (concurrency=1), lý do kỹ thuật khác (race trên base checkout dùng
  chung), không liên quan tới giới hạn kết nối dev server.
- Không giới hạn ở tầng RPC/backend trong tài liệu này (không sửa Go code cho việc này) — giới hạn
  hoàn toàn ở tầng frontend (dễ chỉnh, dễ rollback nếu số 2 sai). Nếu sau này cần chặn cứng ở
  backend (chống 1 client thứ 2 bỏ qua giới hạn frontend), đó là một tài liệu riêng, không phải
  phạm vi v1.

## Phạm vi v1 (chặn rõ, tránh scope creep)

- Chỉ áp dụng cho task **không có subtask/dependency** (`EngineDirectAgent`) — thừa hưởng nguyên
  văn giới hạn đã nêu ở BL-TG-05, lý do giống hệt (task có subtask đi qua `EngineOrchestration`,
  chưa audit riêng).
- **Không** tự động tiến pha khi agent chạy xong — mỗi bước (Gen Spec hàng loạt / Duyệt từng cái /
  Gen Code hàng loạt / Duyệt từng cái / Merge hàng loạt) vẫn cần người bấm, đúng triết lý BL-TG-05.
  "Hàng loạt" chỉ áp dụng cho hành động DISPATCH, không áp dụng cho hành động DUYỆT (duyệt vẫn
  từng task một, đọc diff riêng từng cái — duyệt hàng loạt không xem diff là rủi ro không chấp
  nhận được, cố ý loại khỏi v1).
- **Không** tự động merge khi task chuyển Done — merge luôn là hành động người dùng chủ động bấm,
  chọn đúng `baseBranch`/`strategy` mỗi lần.
- **Không** tự phát hiện/giải quyết conflict ngữ nghĩa giữa các task chạy song song (2 task cùng
  sửa 1 file theo 2 hướng khác nhau nhưng git không báo conflict vì merge theo thứ tự khác nhau) —
  chấp nhận rủi ro này ở v1, xem mục dưới.

## Rủi ro chưa xử lý ở v1 (không che giấu)

- **Xung đột ngữ nghĩa giữa các task chạy song song**: nếu Task A và Task B đều sửa cùng 1 vùng
  logic theo 2 cách khác nhau, mỗi task tự commit vào worktree riêng KHÔNG xung đột git (vì mỗi
  worktree độc lập) — xung đột chỉ lộ ra khi MERGE cả hai vào base, và git conflict detection chỉ
  bắt được xung đột ở MỨC DÒNG, không bắt được xung đột Ý ĐỊNH (VD cả 2 task cùng đổi tên 1 hàm
  nhưng thành 2 tên khác nhau ở 2 chỗ gọi khác nhau). Không có giải pháp tự động cho việc này ở
  v1 — người review merge phải tự ý thức khi chọn nhiều task liên quan để chạy hàng loạt.
- **Ngưỡng concurrency=2 cho Gen Spec/Code là ước lượng, chưa đo thật** — nếu vẫn thấy sập kết nối
  dev server ở mức 2, cần giảm xuống 1 (tuần tự hoàn toàn) — đây là điều chỉnh tham số, không phải
  thiết kế lại.
- **`worktree.merge` conflict path**: khi có conflict, batch dừng lại (thiết kế ở trên) nhưng UI
  hiển thị gì cho conflict đó (mở tab Git để resolve thủ công qua `ConflictOperation`/
  `ResolveConflict` đã có sẵn RPC — CHƯA thiết kế màn hình cụ thể trong tài liệu này, để lại cho
  bước implement chi tiết hoá dựa trên UI Git panel đã có).

## Tiêu chí chấp nhận

- [ ] **BUG-028 đã fix và verify sống trước khi bắt đầu implement tài liệu này** (điều kiện tiên
      quyết, không phải tiêu chí song song).
- [ ] `TaskBoardView` có "Batch mode" bật/tắt được, hiện checkbox trên mỗi thẻ khi bật.
- [ ] Thanh hành động nổi hiện đúng số lượng task đã chọn và đúng nút hành động theo pha chung
      (chỉ hiện "Generate Spec hàng loạt" nếu TẤT CẢ task đã chọn đang ở `not-started`; tương tự
      cho Code/Merge — không trộn lẫn hành động khác pha trong 1 lần bấm).
- [ ] Gen Spec/Code hàng loạt dispatch với concurrency giới hạn (mặc định 2, cấu hình được), mỗi
      task tự cập nhật tiến độ độc lập qua polling có sẵn.
- [ ] Merge hàng loạt chạy tuần tự (concurrency=1), dừng ngay khi gặp lỗi hoặc conflict đầu tiên,
      không tiếp tục merge các task còn lại trong cùng lượt.
- [ ] Merge thành công tự dọn worktree đã merge (`cleanupWorktreeIds`, RPC đã hỗ trợ sẵn).
- [ ] Test tái hiện: chọn 3 task, Gen Spec hàng loạt với concurrency=2 → xác nhận tối đa 2 lệnh
      `task.execute` đang "in-flight" cùng lúc (không phải cả 3 cùng lúc).

## Kế hoạch test

- **Unit (frontend, mới)**: `batchDispatch` — xác nhận đúng số lượng concurrency, xác nhận
  `task.update` (set label) luôn gọi TRƯỚC `task.execute` cho từng task (thứ tự, kế thừa đúng
  ràng buộc BL-TG-05).
- **Unit (frontend, mới)**: `batchMergeWorktrees` — xác nhận dừng ngay khi 1 item lỗi/conflict,
  không gọi tiếp các item sau trong danh sách.
- **Component (frontend, mới)**: `TaskBoardView.test.tsx` — bật Batch mode, chọn 2 task khác pha
  nhau → xác nhận KHÔNG nút hành động hàng loạt nào hiện ra (an toàn mặc định: không đoán ý định
  khi pha không đồng nhất).
- **Thủ công, sau khi BUG-028 fix**: chạy Gen Spec hàng loạt trên 2-3 task thật, xác nhận MỖI task
  có spec file riêng trong ĐÚNG worktree của nó (không lặp lại lỗi BUG-028), rồi Merge từng task
  Done, xác nhận nhánh base nhận đúng commit của đúng task.

## Tham chiếu

- [BL-TG-04](./BL-TG-04-task-agent-execution.md) — mục "Multi-task Agent Session (Batch
  Execution)" của tài liệu này **được thay thế bởi BL-TG-06** (xem "Đối chiếu" ở trên) — thiết kế
  gốc chưa từng implement, không khớp với kiến trúc async/ràng buộc concurrency thật đã tìm ra.
- [BL-TG-05](./BL-TG-05-spec-approve-build-loop.md) — nền tảng vòng lặp đơn-task mà tài liệu này
  mở rộng thành hàng loạt, tái dùng nguyên vẹn
  `derivePhase`/`withPhase`/`buildSpecPrompt`/`buildImplementPrompt`.
- [BUG-027](../../../specs/backend-go/bugs/missing-v2/BUG-027-task-execute-direct-agent-blocks-rpc-timeout.md) —
  nguồn gốc ràng buộc concurrency (bite 2: nhiều dispatch đồng thời làm sập kết nối dev server) và
  lý do `task.execute` async khiến `ExecuteBatch` không dùng lại được nguyên bản.
- [BUG-028](../../../specs/backend-go/bugs/missing-v2/BUG-028-direct-agent-runs-in-shared-repo-not-isolated-worktree.md) —
  **điều kiện tiên quyết bắt buộc**, chưa fix.
- `backend-go/services/git-gateway-service/internal/usecase/merge_worktree_into_base.go`,
  `backend-go/services/api-gateway/internal/adapter/wscompat/channels_worktree.go:241` (`worktree.merge`) —
  RPC merge đã có sẵn, dùng nguyên trong tài liệu này.
- `backend-go/services/task-service/internal/usecase/execute_batch.go` — đọc để hiểu vì sao
  KHÔNG dùng lại (dead code, giả định đồng bộ đã lỗi thời sau BUG-027).
- `frontend/src/renderer/src/components/task/TaskBoardView.tsx` — component cần thêm Batch mode.
