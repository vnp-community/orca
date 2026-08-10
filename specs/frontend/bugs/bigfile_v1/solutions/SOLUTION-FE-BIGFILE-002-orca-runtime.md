# SOLUTION-FE-BIGFILE-002 — Tách `orca-runtime.ts` (26,730 dòng)

**Bug:** `../BUG-FE-BIGFILE-002-orca-runtime.md`
**Trạng thái:** 📝 Proposed
**Thứ tự thực hiện:** #2 (bước 1, helper cuối file) và #10 (bước 2+, class chính)
— xem `SOLUTION-FE-BIGFILE-001` mục 3
**Chiến lược:** Barrel/facade (xem `SOLUTION-FE-BIGFILE-001` mục 1)

---

File lớn nhất repo (26,730 dòng) — chia thành 3 giai đoạn độc lập, KHÔNG làm
chung 1 lần.

## Giai đoạn 1 (rủi ro thấp) — Pure helper function cuối file

| File mới | Nội dung | Dòng gốc | Ước tính dòng |
|---|---|---|---|
| `orca-runtime-tail-buffer.ts` | `appendRecentPtyOutput`, `appendRecentPtyPathCandidates`, `recentTerminalPathCandidatesIncludePath`, `recentTerminalOutputIncludesPath`, `buildPreview`, `type TerminalTailWaitState`, `computeTerminalTailWaitState`, `tailGainedNewerBlockedReason`, `appendNormalizedToTailBuffer`, `appendNormalizedToMultilineTailBufferUnwindowed` | 24,786–cuối file | ~1,945 |

**Bước thực hiện:**
1. `gitnexus impact({target: "computeTerminalTailWaitState", direction: "upstream"})`
   (và các hàm còn lại) — các hàm này rất có thể ĐÃ có test riêng (tên hàm gợi
   ý pure/testable) — kiểm tra `orca-runtime.test.ts` cùng thư mục nếu có,
   xác nhận test hiện tại vẫn import đúng chỗ sau khi tách.
2. Copy nguyên văn khối 24,786→cuối sang `orca-runtime-tail-buffer.ts`.
3. `orca-runtime.ts` thêm `export { ... } from './orca-runtime-tail-buffer'`
   ở cuối file (thay cho định nghĩa gốc).
4. Xác minh: `pnpm run typecheck`, `pnpm run lint`, chạy test liên quan,
   `node scripts/find-frontend-bigfiles.mjs` (kỳ vọng: 26,730 → ~24,785 dòng).

## Giai đoạn 2 (rủi ro trung bình) — Type definitions trước class

| File mới | Nội dung | Dòng gốc | Ước tính dòng |
|---|---|---|---|
| `orca-runtime-types.ts` | `RemoteFetchResult`, `RemoteTrackingBase`, `AccountsSnapshot`, `RuntimeAutomationCreateInput`, `RuntimeAutomationUpdateInput`, `RuntimeTerminalAgentStatusEvent`, `RuntimePtyController`, `MobileNotificationDispatchEvent`, `MobileNotificationDismissEvent`, `MobileNotificationEvent`, `DriverState`, `PtyLayoutTarget`, `PtyLayoutState`, `ApplyLayoutResult` | 773–2,108 | ~1,335 |

**Lưu ý quan trọng:** `RuntimePtyController` (dòng 1,171) là type PUBLIC được
nhiều nơi khác import trực tiếp (không chỉ qua `orca-runtime.ts`) — đã thấy
xuất hiện trong investigation BUG-FE-PTY-001 gần đây. **Bắt buộc chạy
`gitnexus impact({target: "RuntimePtyController", direction: "upstream"})`
trước khi di chuyển** — nếu impactedCount cao, cân nhắc giữ lại re-export ở
CẢ 2 nơi (file gốc và file type mới) trong 1 chu kỳ chuyển tiếp thay vì xoá
hẳn định nghĩa khỏi `orca-runtime.ts`.

**Bước thực hiện:** tương tự Giai đoạn 1 — copy nguyên văn, barrel re-export,
xác minh từng bước. Kỳ vọng sau giai đoạn này: ~24,785 → ~23,450 dòng.

## Giai đoạn 3 (rủi ro cao — làm SAU CÙNG) — Class `OrcaRuntimeService`

Class bắt đầu dòng 2,109, sau 2 giai đoạn trên còn lại ~23,450 dòng, trong đó
class chiếm gần như toàn bộ. **Không tách trong 1 bước** — đề xuất áp dụng
pattern composition theo domain trách nhiệm, theo đúng tiền lệ đã có trong
chính thư mục này:

| Domain đã tách trước (tiền lệ) | File |
|---|---|
| Browser command adapters | `orca-runtime-browser.ts` (1,841 dòng) |
| File command adapters | `orca-runtime-files.ts` (1,885 dòng) |

**Domain còn lại trong class, ứng viên tách tiếp** (dựa trên tên type ở Giai
đoạn 2 — đọc lại comment dòng 1 gốc: "mutable live graph, PTY handles,
waiters, mobile floor/layout state, and managed-worktree reconciliation"):

1. Mobile session mirror / floor-layout state (liên quan
   `MobileNotificationDispatchEvent`, `PtyLayoutState`, `ApplyLayoutResult`)
   → `orca-runtime-mobile-session.ts`
2. Automation (`RuntimeAutomationCreateInput`/`UpdateInput`) →
   `orca-runtime-automation.ts`
3. PTY liveness/waiters (nếu tách được khỏi phần graph chính) →
   `orca-runtime-pty-liveness.ts`
4. Worktree reconciliation — làm SAU CÙNG, sau khi 3 domain trên đã tách,
   để nhìn rõ ranh giới còn lại của "live graph" cốt lõi.

**Cách tách 1 domain khỏi class lớn (composition, không đổi public API):**

```ts
// orca-runtime-mobile-session.ts — nhận các dependency cần thiết qua tham số,
// KHÔNG import ngược lại OrcaRuntimeService (tránh circular)
export class MobileSessionMirrorController {
  constructor(private deps: { /* các field/method OrcaRuntimeService cần chia sẻ */ }) {}
  // các method mobile-session di chuyển từ OrcaRuntimeService sang đây,
  // đổi `this.xxx` → `this.deps.xxx`
}
```

```ts
// orca-runtime.ts — class OrcaRuntimeService giữ 1 field, uỷ quyền gọi
export class OrcaRuntimeService {
  private mobileSession = new MobileSessionMirrorController({ /* ... */ })
  // các public method cũ giữ nguyên chữ ký, forward sang this.mobileSession
}
```

Giữ nguyên TOÀN BỘ public method signature của `OrcaRuntimeService` (forward
call) để KHÔNG phá bất kỳ caller nào bên ngoài — đây là điểm khác biệt so với
barrel pattern thuần tuý (vì đây là instance method, không phải free
function/export, nên không thể re-export trực tiếp).

## Xác minh sau MỖI giai đoạn (không phải mỗi domain nhỏ)

- `pnpm run typecheck` (3 target)
- `pnpm run lint`
- Toàn bộ test hiện có liên quan PTY/terminal/worktree (phạm vi rộng — class
  này là trung tâm của rất nhiều flow)
- `gitnexus detect_changes({scope: "all"})` + `gitnexus impact` cho từng
  public method bị di chuyển
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

- Giai đoạn 1: **Thấp** — pure function, không `this`.
- Giai đoạn 2: **Trung bình** — cần xác nhận `RuntimePtyController` và các
  type khác không bị vỡ import ở nơi khác.
- Giai đoạn 3: **Cao** — đây là class trung tâm của gần như mọi flow
  terminal/PTY/worktree/mobile-session; đã là nơi bắt nguồn/liên quan trực
  tiếp tới investigation BUG-FE-PTY-001 (điều tra kéo dài nhiều phiên làm
  việc). **Bắt buộc có test coverage tốt trước khi tách**, làm từng domain
  một, không gộp nhiều domain trong 1 PR.
