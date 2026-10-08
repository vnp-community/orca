# FE-CV-SOL-089-agent-turn-recorder: Ghi lượt agent lên backend và hiển thị đối chiếu "agent tự báo"

> ✅ **Done.** Trạng thái (cập nhật 2026-10-08): 7/7 task DONE. Chi tiết ở dòng `**Status:**` và "Ghi chú hoàn thiện" của từng task.

**CR:** [CR-CV-089 mục 2.8 và 2.4](../../../../../../docs/crs/v7/quality-gate/CR-CV-089-agent-provenance-and-claim-reconciliation.md) (phần frontend; `BE-CV-SOL-089-agent-turn-provenance` lo kho `agent_turns`)
**Area:** frontend (`frontend/src/renderer/src/components/review-map/turns/`, `hooks`, `lib`)
**Hợp đồng áp dụng:** [CONTRACT-codeintel-ui-api.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-ui-api.md) mục 2.3, 2.4 (giới hạn `args[0]` ≤ 16 KiB), 3.2 (`quality.turn.record|turns|turn`), 4.6 (`ReviewTurnMarker`), 4.7 (`AgentTurn`); [CONTRACT-codeintel-proto-and-data-map.md](../../../../../backend-go/crs/v7/CONTRACT-codeintel-proto-and-data-map.md) PQ-04, PQ-22 (`turnId` = `client_turn_id`), PQ-35 (chỉ nguồn A, renderer), PQ-27, mục 6.1 (cờ `agent_turn_store_prompt_excerpt`, `agent_claim_text_enabled`), 8.3.
**TDD tham chiếu:** [v5/02-state-management](../../../../tdd/v5/02-state-management.md), [v5/07-hooks-and-ipc](../../../../tdd/v5/07-hooks-and-ipc.md).

## 1. Trạng thái hiện tại (re-verify)

Đã đọc: `shared/agent-status-types.ts` (`AgentStatusEntry` dòng 99 đến 159, `AgentStateHistoryEntry` dòng 57, `AGENT_STATE_HISTORY_MAX = 20` dòng 70, `AGENT_STATUS_STATES` dòng 18), `shared/agent-status-field-normalization.ts` (`AGENT_STATUS_MAX_FIELD_LENGTH = 200` dòng 13), `renderer/src/store/slices/agent-status.ts` (`agentStatusByPaneKey` dòng 101, `retainedAgentsByPaneKey` dòng 115, `setAgentStatus` dòng 135/1179, `retainedAgentEntryFromLive` dòng 393 dùng `entry.stateHistory[0]?.startedAt ?? entry.stateStartedAt` cho `startedAt`), `lib/sha256.ts` (sync `sha256(Uint8Array): Uint8Array`, byte-giống `crypto.subtle`), `store/slices/editor.ts` (`gitStatusByWorktree` dòng 612, `gitBranchChangesByWorktree` dòng 695), `shared/types.ts:3793` (`GitBranchCompareSummary.headOid`), `lib/worktree-runtime-owner.ts:162` (`getRuntimeEnvironmentIdForWorktree`).

Xác nhận đúng CR-CV-089 mục 1.1: renderer chỉ giữ `prompt` đã cắt 200 ký tự; `toolName`/`toolInput` chỉ là **công cụ hiện tại** (xem preview), `stateHistory` cố ý bỏ `toolName/toolInput/lastAssistantMessage`; **không có mã thoát** của lệnh và **không có trường `model`** trong `AgentStatusEntry` (đã grep: không có). Slice agent-status "real-time only, lives in renderer memory" (dòng 100). Hệ quả: `commandsSummary` chỉ dựng được nếu renderer **lấy mẫu** các cập nhật `setAgentStatus` trong lúc lượt chạy; model luôn vắng ở nguồn A.

**Correction relative to CR-CV-089 (hợp đồng và mã thật thắng):**

| # | CR ghi | Thực tế / hợp đồng | Quyết định trong solution |
|---|---|---|---|
| 1 | Tên kênh `codeIntel.quality.turn.record\|turns\|turn` | Hợp đồng 3.2 giữ đúng ba tên; thêm khoá `{projectId, worktreeId}` (PQ-04) | Dùng bộ khoá `projectId`/`worktreeId`; `repo_binding_id` không xuất hiện ở frontend |
| 2 | Gửi `prompt_digest`, snake_case | Hợp đồng H1: camelCase `promptDigest`, `clientTurnId`, `endHeadCommit`, `treeDirtyEnd`... | camelCase |
| 3 | Hai nguồn A (renderer) và B (hook) | PQ-35: **chỉ nguồn A** trong hợp đồng; `agent.hook` không dùng | Chỉ renderer; không có đường hook; `AgentTurn.source` do backend điền (`renderer`) |
| 4 | `use-review-turn-recorder` (CR-060) "gọi thêm" `RecordAgentTurn` | Hợp đồng không quy định nơi gọi; CR-060 thuộc FE-CV-SOL-060-review-notes-and-turn-compare (agent khác soạn) | Solution này cung cấp hook `useAgentTurnBackendRecorder` mà recorder của 060 gọi sau khi tạo `ReviewTurnMarker`; hai bên chỉ chia sẻ `turnId` và `files[]` (không chung state) |
| 5 | Tham số có `claims` | Hợp đồng `quality.turn.record` liệt kê `claims?`; CR-089 2.4: `claims` do **backend** trích (`agent_claim_extractor.go`) | Frontend **không gửi `claims`** ở MVP (câu hỏi mở 1); chỉ gửi `commandsSummary` |
| 6 | `commands_summary` có `v:1` và ≤ 20 mục | Hợp đồng 4.7 `AgentTurn.commandsSummary` có `v:1`, `totalToolUses`, `commands[{name, sub?, category, count}]`, `toolCounts`, `truncated` | Dựng đúng dạng này ở renderer; `category ∈ test\|lint\|typecheck\|build\|install\|git\|other` |
| 7 | `prompt_excerpt` chỉ khi tenant bật | Hợp đồng 6.1 / `Settings.tenant.agentTurnStorePromptExcerpt` (mặc định `false`); hàm che `maskSensitiveText` chưa có trong code | Đọc từ `settings.get`; `false` hoặc chưa có hàm che thì không bao giờ đưa trích đoạn vào payload |
| 8 | `model` tuỳ chọn | `AgentStatusEntry` không có `model` | Luôn bỏ `model` (backend không lấy được từ renderer; `AgentTurn.model` thường vắng) |
| 9 | `DeleteAgentTurns` (admin) | Không có kênh ở hợp đồng 3.2 | Không có UI xoá (CR-089 Q5 vẫn mở) |

**Chưa kiểm chứng:** (a) mỗi cập nhật hook có tạo đúng một lần đổi `toolName`/`toolInput`/`updatedAt` hay không (đếm công cụ có thể sai); (b) quan hệ chính xác giữa `stateHistory[0]` và thời điểm bắt đầu lượt (rolling log tối đa 20); (c) tên API của FE-CV-SOL-050 và FE-CV-SOL-061/060 (`AgentTurnCompletion`, `selectAgentTurnCompletions`, `ReviewTurnMarker.files`) vì `specs/frontend/crs/v7/` chưa có khi soạn.

## 2. Giải pháp

### 2.1 Cây file

```
frontend/src/renderer/src/components/review-map/turns/
  agent-turn-record-params.ts               (mới) hàm thuần: dựng tham số quality.turn.record
  agent-turn-record-params.test.ts          (mới)
  agent-tool-use-command-summarizer.ts      (mới) hàm thuần + lớp thu thập theo pane
  agent-tool-use-command-summarizer.test.ts (mới)
  agent-turn-record-queue.ts                (mới) hàng đợi gửi, thử lại, khử trùng lặp
  agent-turn-record-queue.test.ts           (mới)
  use-agent-turn-backend-recorder.ts        (mới) hook gắn mọi thứ
  use-agent-turn-backend-recorder.test.tsx  (mới)
  agent-turn-verification-view-model.ts     (mới) hàm thuần: AgentTurn -> dòng đối chiếu
  agent-turn-verification-view-model.test.ts(mới)
  AgentTurnVerificationLine.tsx             (mới) component một dòng (+ test)
frontend/src/renderer/src/lib/
  agent-turn-digest.ts                      (mới) sha256 hex cho prompt/tệp (dùng lib/sha256.ts)
  agent-turn-digest.test.ts                 (mới)
frontend/src/renderer/src/i18n/
  agent-turn-verification-locale-coverage.test.ts (mới); locales/{en,es,ja,ko,zh}.json (sửa)
```

Chạm FE-CV-SOL-060: `use-review-turn-recorder.ts` (của 060) thêm đúng **một** lời gọi hook của solution này; không sửa nội dung `ReviewTurnMarker`.

### 2.2 Chữ ký TypeScript

```ts
// agent-turn-record-params.ts
import type { AgentStatusEntry } from '../../../../../shared/agent-status-types'

export type AgentTurnRecordParams = {            // khớp hợp đồng 3.2, camelCase; KHÔNG có trường prompt
  projectId: string; worktreeId: string
  clientTurnId: string                           // `${paneKey}:${doneAt}` (PQ-22)
  agentType: string; endedAt: string; startedAt?: string; interrupted?: boolean
  endHeadCommit: string; treeDirtyEnd: boolean
  filesChangedCount: number; filesDigest: string
  promptDigest: string; promptExcerpt?: string
  commandsSummary?: AgentTurnCommandsSummary     // = AgentTurn['commandsSummary'] (hợp đồng 4.7)
}

export function buildAgentTurnRecordParams(input: {
  projectId: string; worktreeId: string
  entry: Pick<AgentStatusEntry, 'paneKey'|'agentType'|'prompt'|'stateStartedAt'|'stateHistory'|'interrupted'>
  headOid: string | null                         // thiếu => trả null, không ghi
  treeDirty: boolean
  fileIdentities: readonly string[]              // từ ReviewTurnMarker.files (060): `${p}|${o ?? ''}|${h}`
  commands: AgentTurnCommandsSummary | null
  storePromptExcerpt: boolean                    // Settings.tenant.agentTurnStorePromptExcerpt
}): AgentTurnRecordParams | null

// agent-tool-use-command-summarizer.ts
export type AgentTurnCommandsSummary = {
  v: 1; totalToolUses: number
  commands: { name: string; sub?: string; category: 'test'|'lint'|'typecheck'|'build'|'install'|'git'|'other'; count: number }[]
  toolCounts: Record<string, number>; truncated: boolean
}
export function normalizeToolInput(toolName: string, toolInput: string): { name: string; sub?: string; category: ... } | null
export function createAgentTurnToolCollector(): {
  observe(paneKey: string, entry: Pick<AgentStatusEntry,'state'|'toolName'|'toolInput'|'updatedAt'>): void
  take(paneKey: string): AgentTurnCommandsSummary | null   // lấy và xoá khi lượt kết thúc
  reset(paneKey: string): void
}

// agent-turn-record-queue.ts
export function createAgentTurnRecordQueue(deps: {
  send: (params: AgentTurnRecordParams) => Promise<void>   // ném CodeIntelRpcError (FE-CV-SOL-050)
  now: () => number; sleep: (ms: number) => Promise<void>
}): { enqueue(p: AgentTurnRecordParams): void; pending(): number; dispose(): void }

// use-agent-turn-backend-recorder.ts
export function useAgentTurnBackendRecorder(): void        // gọi một lần ở mức ReviewWorkspace/recorder của 060

// agent-turn-verification-view-model.ts
export type AgentTurnVerificationViewModel = {
  ranLine: string | null            // "Agent đã chạy: test (3 lần), lint"
  checks: { kind: AgentTurn['claims']['items'][number]['kind']
            agreement: 'consistent'|'contradicted'|'unverified'|'not_claimed'|'unknown'
            textKey: string; params: Record<string,string>; verifyingRunId?: string }[]
  hasContradiction: boolean
}
export function buildAgentTurnVerificationViewModel(turn: AgentTurn): AgentTurnVerificationViewModel
```

`AgentTurn` và `GateResult` do FE-CV-SOL-050 sao chép từ hợp đồng 4.7.

### 2.3 Thu thập lượt và ghi

1. **Điều kiện bật:** `useQualityFeatureFlags().quality` (FE-CV-TASK-085-01) **và** có `AgentTurnCompletion` (FE-CV-SOL-061). Cờ tắt: không đăng ký, không gọi kênh. Nhóm kênh `quality.*` nằm sau cờ `qualityGateEnabled` (PQ-01), nên không ghi lượt khi chỉ `codeIntelEnabled` bật.
2. **Collector công cụ.** Một `createAgentTurnToolCollector` sống trong module (không persist), được `useAppStore.subscribe` cấp dữ liệu: với mỗi pane có `state === 'working'`, mỗi lần `updatedAt` tăng **và** `toolName` khác rỗng và cặp `(toolName, normalized preview)` khác lần ghi cuối của pane, tính một lần dùng công cụ. Bỏ qua `interactivePrompt`, `lastAssistantMessage`, `prompt`. Chỉ giữ tên chương trình + tối đa một tiểu lệnh (`pnpm test`, `go test`, `git commit`); bỏ đối số, đường dẫn, biến môi trường, chuyển hướng, URL, mọi chuỗi sau tiểu lệnh (thuật toán ở CR-089 2.3, bảng category ở file mới, có `v`). Tối đa 20 mục `commands`; vượt thì `truncated=true`. `toolCounts` đếm theo `toolName` (ví dụ `Edit`, `Bash`) ≤ 32 khoá.
3. **Khi lượt kết thúc** (nhận `AgentTurnCompletion` từ 061, đã khử trùng lặp): chờ trạng thái git ổn định bằng chính debounce của recorder 060 (đề xuất ~3 s, chưa đo); đọc `headOid` từ `gitBranchCompareSummaryByWorktree[worktreeId]`, `treeDirty` từ `gitStatusByWorktree[worktreeId].length > 0`, `fileIdentities` từ `ReviewTurnMarker.files` vừa tạo; gọi `buildAgentTurnRecordParams`. Trả `null` (không ghi) nếu thiếu `headOid`, `projectId` hoặc `worktreeId` thuộc `FLOATING_TERMINAL_WORKTREE_ID`/không có môi trường.
4. **Quyền riêng tư.** `promptDigest = sha256Hex(normalizePrompt(entry.prompt))` (`normalizePrompt`: `trim`, gộp khoảng trắng, NFC); `prompt` đã bị `AGENT_STATUS_MAX_FIELD_LENGTH = 200` cắt ở nguồn nên digest là của bản đã cắt (đủ để biết hai lượt cùng prompt đầu). `promptExcerpt` chỉ có khi `Settings.tenant.agentTurnStorePromptExcerpt === true` **và** có hàm che bí mật phía client: hợp đồng 4.4 nhắc `maskSensitiveText` nhưng đã grep `frontend/src` và `backend-go`: **chưa tồn tại** (FE-CV-SOL-058-storage-lens dự kiến tạo). Cho tới khi có, `promptExcerpt` KHÔNG BAO GIỜ được gửi (backend vẫn che lớp hai, CR-089 2.5); khi có thì che rồi cắt 160 ký tự. Không gửi `lastAssistantMessage`, `toolInput` thô, `interactivePrompt`, đường dẫn tuyệt đối. Payload < 16 KiB (hợp đồng 2.4).
5. **Hàng đợi.** `enqueue` gọi `quality.turn.record`; khử trùng lặp theo `clientTurnId` trong bộ nhớ (backend cũng idempotent theo `(binding, client_turn_id)`); thử lại tối đa 3 lần với backoff 2 s, 6 s, 18 s cho lỗi `offline|timeout|tool-failed|unknown|rate-limited`; bỏ ngay (không thử lại) với `quality-disabled|disabled|unsupported|forbidden|validation|not-found|no-binding`. Lỗi **không bao giờ** chặn UI hay ghi mốc cục bộ của 060 (tiêu chí CR-089). Không persist hàng đợi: đóng ứng dụng thì mất lượt chưa gửi (CR-089 chấp nhận "lượt thiếu vẫn hợp lệ"). SSH: gửi nền, không spinner.

### 2.4 Đọc và hiển thị đối chiếu

Dữ liệu: `quality.turns {projectId, worktreeId, limit ≤ 50}` và `quality.turn {turnId}` (quyền `quality_read`); do `ReviewTurnSwitcher` của 060 và ngăn lượt của 087 gọi; solution này chỉ cấp view-model và `AgentTurnVerificationLine`.

Quy tắc chữ (chung cho `claims.items[].agreement`, `consistent|contradicted|unverified|not_claimed`, enum lạ → `unknown`):

| `agreement` | Dòng hiển thị (ví dụ) | Ghi chú |
|---|---|---|
| `consistent` | "Chạy lại: kiểm tra {kind} đạt" | Chỉ nói về lần **chạy lại độc lập** (`verifyingRunId`), không nói "agent đúng" |
| `contradicted` | "Kết quả chạy lại khác với lời agent nói: {N} lỗi" + liên kết run | Cấm "nói dối", "giả mạo", "đánh lừa", "sai sự thật" (CR-089 2.4) |
| `unverified` | "Chưa đối chiếu" + nút "Chạy lại kiểm tra" (nếu `canRun`) | `agreementReason` (ví dụ `tree_may_differ`) hiện ở tooltip; `unverified` KHÔNG thành `consistent` |
| `not_claimed` | không hiển thị mặc định | |

`ranLine` dựng từ `commandsSummary.commands[].category` đếm nhóm, ví dụ "Agent đã chạy: test (3 lần), lint". Câu này nói **đã chạy lệnh**, không nói "đã báo đạt" (không có mã thoát; CR-089 quyết định 3). `claims.items[].basis === 'stated'` (chỉ khi tenant bật `agentClaimTextEnabled`) hiện thêm nhãn "Suy luận từ lời agent" và luôn ở mức tin cậy thấp. Bắt buộc ghi chú ở tooltip: "Ghi nhận, không phải xác thực: tiến trình agent có thể báo sai" (CR-089 1.2 điểm 3). `AgentTurn.gate?.verdict` hiển thị qua huy hiệu cổng chung (FE-CV-SOL-087), `unknown` luôn "Chưa đủ dữ liệu để kết luận".

Wireframe (một thẻ lượt trong `ReviewTurnSwitcher`):

```
┌ Lượt 14:32 · claude ────────────────────────────┐
│ Agent đã chạy: test (3 lần), lint               │  muted-foreground, 11px
│ ⚠ Kết quả chạy lại khác với lời agent nói: 2 lỗi │  destructive nhẹ, kèm icon + chữ (không chỉ màu)
│   [Xem lần chạy]                                │
│ ◌ typecheck: chưa đối chiếu   [Chạy lại kiểm tra]│
└─────────────────────────────────────────────────┘
```

Màu: token `muted-foreground`, `destructive`, `border`; icon `lucide-react` (`TriangleAlert`, `CircleHelp`, `Terminal`), cỡ `size-3.5`; không hex, không emoji. Mọi chuỗi qua `translate()` với khoá `auto.components.review.map.turns.<tên>` đọc theo tên; test phủ năm locale.

## 3. Quyết định thiết kế

- Renderer chỉ ghi **metadata** và digest; mặc định không có prompt. Đồng nhất với CR-CV-060 ("không lưu prompt lên backend") và F8 của feature.
- Lấy mẫu công cụ ở renderer vì nguồn hook backend không nằm trong hợp đồng (PQ-35); chấp nhận độ chính xác thấp và ghi rõ trong tooltip.
- Không gửi `claims`: tránh trích văn bản agent ở phía client; backend suy `ran_command` từ `commandsSummary`.
- Hàng đợi chỉ trong bộ nhớ: không thêm lưu trữ bền ở frontend (không đụng `storage/`).
- Chữ trung lập: "khác với lời agent nói", không quy kết; `unknown` là trạng thái thật.
- Không thêm thư viện; sha256 dùng `lib/sha256.ts` (an toàn cả ngữ cảnh web không bảo mật, nơi `crypto.subtle` không có, theo chú thích đầu file đó).

## 4. Phụ thuộc chéo khu vực

| Cần | Nơi | Ghi chú |
|---|---|---|
| Kênh `quality.turn.record|turns|turn` | `BE-CV-SOL-040-codeintel-quality-channels`; kho `BE-CV-SOL-089-agent-turn-provenance` | Trước G3: fake backend G4 với fixture `AgentTurn` đủ bốn `agreement`, lượt không `claims` |
| Run độc lập để `verification` có nghĩa | `BE-CV-SOL-082-quality-run-storage-and-ingest`, `AG-CV-SOL-081-quality-runner-core` | Không có thì mọi claim là `unverified` |
| `AgentTurnCompletion`, `selectAgentTurnCompletions` | `FE-CV-SOL-061-review-entry-points` | |
| `ReviewTurnMarker`, `turn-file-identity`, `ReviewTurnSwitcher`, debounce git | `FE-CV-SOL-060-review-notes-and-turn-compare` | Solution này cắm vào, không sửa |
| Cờ `quality`, hook cờ | `FE-CV-TASK-085-01` (trong FE-CV-SOL-085-source-control-quality-notice) | |
| Bridge, slice, phân loại lỗi, kiểu `AgentTurn` | `FE-CV-SOL-050-types-and-runtime-bridge`, `FE-CV-SOL-050-store-and-query-hooks` | G4 trước |
| Huy hiệu cổng và nút chạy lại | `FE-CV-SOL-087-quality-scorecard-and-state` | |
| AG | Không có solution AG; `agent.hook` không dùng (PQ-35, O-8 mở) | |

## 5. Tiêu chí chấp nhận

- [ ] `buildAgentTurnRecordParams` không bao giờ đưa `prompt` thô, `lastAssistantMessage`, `toolInput` thô, đường dẫn tuyệt đối vào đầu ra (test snapshot khoá tập khoá đầu ra bằng tập khoá hợp đồng).
- [ ] `promptExcerpt` chỉ xuất hiện khi cờ tenant bật, ≤ 160 ký tự, đã che; tắt thì không bao giờ có.
- [ ] `commandsSummary` ≤ 20 mục, không chứa đối số/đường dẫn/URL/biến môi trường (bảng ca: `pnpm test --filter x`, `cd /tmp && rm -rf …`, `curl -H "Authorization: …" https://…`, lệnh 200 ký tự cắt giữa ký tự UTF-8).
- [ ] Hai lần `enqueue` cùng `clientTurnId` chỉ gửi một lần; lỗi tạm thời thử lại tối đa 3 lần; lỗi `quality-disabled`/`forbidden` không thử lại.
- [ ] Cờ `quality` tắt: 0 lời gọi, không subscribe store.
- [ ] Lỗi RPC không chặn việc ghi mốc lượt cục bộ của 060 (test: `send` ném, `ReviewTurnMarker` vẫn tạo).
- [ ] `contradicted` hiện đúng chữ trung lập; `unverified` có nút chạy lại khi `canRun`; không chuỗi nào chứa "nói dối", "giả mạo", "đánh lừa".
- [ ] Hoạt động ở Electron và web (web `crypto.subtle` có thể vắng; sha256 dùng thư viện nội bộ).
- [ ] Năm locale có đủ khoá; không hex cứng.

## 6. Kiểm thử (Vitest + Testing Library)

- Thuần: `normalizeToolInput` (bảng ca trên, fuzz ngắn chuỗi ngẫu nhiên không ném), `buildAgentTurnRecordParams` (thiếu `headOid` → `null`, `interrupted`, `startedAt` từ `stateHistory`), `agent-turn-digest` (vector SHA-256 đã biết, NFC, chuỗi rỗng), view-model đối chiếu (bốn `agreement` + enum lạ).
- Collector: giả `updatedAt` tăng/không đổi, hai ping cùng công cụ, pane đổi lượt, `take` xoá trạng thái.
- Hàng đợi: đồng hồ giả, backoff, khử trùng lặp, huỷ khi `dispose`.
- Hook (`renderHook`, store giả theo `createTestStore`): cờ tắt, nhận completion → một lần `quality.turn.record`, thiếu `projectId` bỏ qua.
- Component `AgentTurnVerificationLine` (`// @vitest-environment happy-dom`): bốn trạng thái, tooltip "ghi nhận, không phải xác thực", nút chạy lại gọi callback.
- Chạy: `pnpm --filter orca-frontend test -- src/renderer/src/components/review-map/turns src/renderer/src/lib/agent-turn-digest` (chưa chạy).

## 7. Rủi ro và điểm chưa kiểm chứng

- Không có mã thoát và không có lịch sử công cụ (CR-089 mục 6): "agent tự báo" thực chất là "agent đã chạy lệnh loại X". Giá trị của tính năng thấp hơn mô tả CR nếu renderer đóng giữa lượt.
- Đếm công cụ bằng lấy mẫu có thể lệch (ping lặp, hai công cụ giữa hai lần lấy mẫu).
- Digest prompt trên bản đã cắt 200 ký tự: hai prompt dài khác nhau ở phần đuôi có thể trùng digest.
- `ReviewTurnMarker.files` và debounce git thuộc 060 chưa tồn tại; chữ ký ở 2.2 là giả định.
- Hook tin cậy: tiến trình agent có thể giả cập nhật trạng thái; dữ liệu là ghi nhận, không xác thực.
- Đã đọc `agent-status.ts` chỉ ở các dòng nêu trên; chưa đọc toàn bộ 2 590 dòng (có `eslint-disable max-lines` cũ ở dòng 1).

## 8. Câu hỏi mở

1. Frontend có nên gửi `claims` (chỉ loại `ran_command`) cho backend, hay để backend suy từ `commandsSummary` (hợp đồng chưa nói rõ ai điền `claims`)?
2. `promptDigest` khi prompt rỗng: gửi digest của chuỗi rỗng hay bỏ trường (hợp đồng không đánh dấu `?`)?
3. Có cần `codeIntel.hintAgentTurnFinished` (P1, hợp đồng 3.3) để backend biết "agent xong" ngay cả khi không có `quality.turn.record`?
4. Quy tắc `startedAt`: dùng mốc `working` gần nhất trong `stateHistory`, hay bỏ (CR-089 2.1: "không đoán")? Solution dùng mốc `working` gần nhất nếu có, nếu không thì bỏ.

## 9. Tham chiếu

`/opt/repos/orca/docs/crs/v7/quality-gate/CR-CV-089-agent-provenance-and-claim-reconciliation.md`, `/opt/repos/orca/docs/crs/v7/review-frontend/CR-CV-060-review-notes-send-to-agent-and-turn-compare.md`, `CR-CV-061-review-entry-points.md`, `/opt/repos/orca/frontend/src/shared/agent-status-types.ts`, `/opt/repos/orca/frontend/src/renderer/src/store/slices/agent-status.ts`, `/opt/repos/orca/frontend/src/renderer/src/lib/sha256.ts`, `/opt/repos/orca/guides/STYLEGUIDE.md`, `/opt/repos/orca/AGENTS.md`.
