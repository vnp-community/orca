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

Sau TASK-008/009, class chiếm dòng 2,030–24,617 (~22,587 dòng) trong file
24,733 dòng. **Không tách trong 1 bước.**

**Cập nhật (TASK-BIGFILE-035, phân tích dữ liệu thật thay cho đoán theo
tên type):** 4 domain liệt kê ban đầu ở bản kế hoạch này (đoán từ tên type
Giai đoạn 2) **không khớp với ranh giới thật** — xác nhận bằng phân tích
field-span (đếm dòng nhỏ nhất/lớn nhất mỗi private field được tham chiếu).
Kết quả thật, xem đầy đủ tại
`../tasks/TASK-BIGFILE-035-orca-runtime-service-domains-investigate.md`:

- **State lõi dùng chéo toàn class** (`ptyController`, `notifier`,
  `ptysById`, `leaves`/`handles`/`handleByPtyId`, `_orchestrationDb`,
  `graphStatus`, `tabs` — span 15,000–21,000+ dòng): KHÔNG tách được bằng
  Move cơ học, cần thiết kế "RuntimeGraphStore" riêng (kiến trúc mới, không
  phải giai đoạn này) + test coverage tốt hơn hiện tại (hiện KHÔNG có test
  bao phủ `OrcaRuntimeService` bản frontend).
- **5 domain tách được ngay** (field co cụm, ranh giới liền mạch — pattern
  composition, KHÔNG phải barrel vì là instance method):

  | Task | Domain | Dòng gốc (ước tính) |
  |---|---|---|
  | TASK-BIGFILE-036 | Automation | ~191 dòng |
  | TASK-BIGFILE-037 | Mobile floor / remote-desktop / layout queue | ~1,830 dòng |
  | TASK-BIGFILE-038 | Remote fetch dedup/cache | ~238 dòng |
  | TASK-BIGFILE-039 | Branch cleanup / managed-worktree removal | ~608 dòng |
  | TASK-BIGFILE-040 | Resolved-worktree cache/lineage | ~247 dòng |

  Tổng ~3,114 dòng (~14% class) — đáng kể nhưng không giải quyết hết; phần
  PTY lifecycle/live-graph lõi (~5,600+ dòng) vẫn ở lại cho tới khi có
  RuntimeGraphStore.
- **Phát hiện quan trọng**: "mobile session tabs" (CRUD/listing,
  headless hydration — 106 method theo cụm từ khoá) và "mobile
  floor/layout" (60 method, co cụm 1 khối liền) tưởng là 1 domain nhưng
  field-span cho thấy chúng KHÁC NHAU — chỉ "mobile floor/layout" tách
  được ngay, "mobile session tabs" còn rải rác, cần phân tích field-span
  riêng trước khi sinh task Move.

Tiền lệ đã tách trước đó (không phải instance method, dùng barrel thuần):

| Domain đã tách trước (tiền lệ) | File |
|---|---|
| Browser command adapters | `orca-runtime-browser.ts` (1,841 dòng) |
| File command adapters | `orca-runtime-files.ts` (1,885 dòng) |

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
