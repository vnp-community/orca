# TASK-BIGFILE-035 — Investigate: `OrcaRuntimeService` domain boundaries

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-008, 009 đã xong (file nguồn hiện còn 24,733
dòng, class `OrcaRuntimeService` dòng 2,030–24,617, ~22,587 dòng)
**Status:** ✅ Done (ghi chú thiết kế — sinh 5 task Move mới TASK-BIGFILE-036..040)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3, đã cập nhật với dữ liệu thật thay cho ước đoán ban đầu)

## ⚠️ Phương pháp — thay thế cách đoán domain theo tên type (đã sai lệch)

Bản kế hoạch gốc (4 domain đoán từ tên type ở Giai đoạn 2) **không dùng
được trực tiếp** — dữ liệu thật cho thấy ranh giới thực tế khác đáng kể.
Thay vào đó, phân tích này dùng 2 nguồn dữ liệu khách quan, trích xuất
bằng `grep`/`awk` (không đọc tuyến tính 22,587 dòng):

1. **Bản đồ method**: liệt kê toàn bộ 747 method/property của class (dòng
   bắt đầu + tên), suy ra độ dài gần đúng = khoảng cách tới method tiếp
   theo.
2. **Bản đồ field**: với mỗi trong 90 private field, đếm số lần
   `this.<field>` xuất hiện + dòng nhỏ nhất/lớn nhất tham chiếu tới nó
   ("span"). Field có **span nhỏ** (tham chiếu co cụm trong 1 khoảng dòng
   hẹp) là tín hiệu mạnh cho ranh giới domain tách được; field có **span
   lớn** (~15,000–21,000+ dòng, trải gần hết class) là state lõi dùng
   chéo — KHÔNG tách được bằng cơ học, cần redesign kiến trúc riêng.

## Phát hiện chính

### 1. State "lõi" thật sự dùng chéo toàn class (KHÔNG tách trong task này)

| Field | Số lần dùng | Span (dòng) |
|---|---|---|
| `ptyController` | 67 | 19,049 |
| `notifier` | 49 | 17,009 |
| `ptysById` | 39 | 18,463 |
| `leaves` / `leavesByPtyId` / `handles` / `handleByPtyId` / `handleByLeafKey` | 22–33 | 16,000–19,000 |
| `_orchestrationDb` | 19 | 19,126 |
| `graphStatus` / `rendererGraphEpoch` / `authoritativeWindowId` | 13–17 | 18,700–21,700 |
| `tabs` | 16 | 19,048 |

Đây chính xác là điều comment dòng 1 gốc mô tả ("mutable live graph, PTY
handles, waiters"). Các field này được hầu hết mọi domain (PTY, mobile,
worktree, agent-status...) đọc/ghi trực tiếp — tách domain nào cũng phải
inject lại các field này qua constructor. **Không đề xuất tách trong đợt
này** — cần thiết kế 1 "RuntimeGraphStore" tách riêng trước (thay đổi kiến
trúc, không phải Move cơ học), và cần test coverage tốt hơn hiện tại
(gitnexus báo class này ở bản frontend **không có test bao phủ**).

### 2. 5 domain tách được — ranh giới liền mạch, field co cụm (an toàn)

Khác với dự đoán ban đầu (dựa theo tên type ở Giai đoạn 2), dữ liệu field-
span cho thấy **"mobile session" và "mobile floor/layout" là 2 domain
KHÁC NHAU**, không phải 1: "mobile session tabs" (CRUD/listing tab,
headless hydration) rải rác dòng 3,140–20,479 (KHÔNG co cụm — giữ nguyên,
chưa tách được); còn "mobile floor/layout/remote-desktop" co cụm chặt
trong 1 khối ~1,830 dòng liên tục — tách được ngay.

| # | Task mới | Domain | Dòng gốc (ước tính, xác nhận lại khi Move) | Số method | Field sở hữu (span hẹp) |
|---|---|---|---|---|---|
| 1 | TASK-BIGFILE-036 | Automation | 2,636–2,827 (~191 dòng) | 8 | `automationService` (span 75) |
| 2 | TASK-BIGFILE-037 | Mobile floor / remote-desktop viewport / layout queue | 7,781–9,611 (~1,830 dòng) | ~60 | `terminalFitOverrides`, `remoteDesktopOwners`, `mobileDictation`, `pendingRestoreTimers`, `freshSubscribeGuard`, `remoteDesktopHostReclaimTargets`, `layouts`, `layoutQueues`, `lastRendererSizes`, `mobileDisplayModes`, `resizeListeners`, `mobileSubscribers`, `currentDriver`, `currentBrowserDriver`, `remoteDesktopViewers`, `remoteDesktopViewerRevisions`, `remoteDesktopActivity` |
| 3 | TASK-BIGFILE-038 | Remote fetch dedup/cache | 15,772–16,010 (~238 dòng) | 9 | `canonicalFetchKeyCache`, `fetchInflight`, `fetchLastCompletedAt`, `remoteFetchQueueTail` |
| 4 | TASK-BIGFILE-039 | Branch cleanup / managed-worktree removal | 16,711–17,319 (~608 dòng, gồm 1 method `removeManagedWorktree` ~490 dòng — xem cảnh báo dưới) | 5 | `preservedBranchCleanupByWorktreeId`, `removeManagedWorktreeInFlight` |
| 5 | TASK-BIGFILE-040 | Resolved-worktree cache/lineage | 19,660–19,907 (~247 dòng) | 9 | `resolvedWorktreeCache`, `resolvedWorktreeInFlight`, `resolvedWorktreeGeneration` |

**Tổng tách được đợt này: ~3,114 dòng** (~14% của 22,587 dòng class) —
đáng kể nhưng KHÔNG giải quyết hết vấn đề; phần lõi "live graph" + PTY
lifecycle (184 method, ~5,596 dòng theo cụm từ khoá, span rất lớn) vẫn ở
lại `orca-runtime.ts` cho tới khi có thiết kế "RuntimeGraphStore" riêng.

**Cảnh báo phụ**: `removeManagedWorktree` (TASK-039) tự nó dài ~490 dòng —
1 method khổng lồ. Tách file không giải quyết việc này; ghi nhận lại làm
ứng viên refactor nội bộ (chia nhỏ method) ở 1 task riêng sau này, không
gộp vào TASK-039.

### 3. Pattern thực thi cho cả 5 task — composition, KHÔNG phải barrel thuần

Vì đây là instance method (dùng `this`), không phải free function, nên
**không thể `export { ... } from ...` trực tiếp** như TASK-008/009. Áp
dụng đúng pattern composition đã có trong `SOLUTION-FE-BIGFILE-002` Giai
đoạn 3: mỗi domain thành 1 class riêng nhận dependency qua constructor,
`OrcaRuntimeService` giữ 1 field instance + forward gọi, GIỮ NGUYÊN toàn
bộ chữ ký public method cũ (không phá caller ngoài).

## Việc CHƯA làm (ngoài phạm vi Investigate)

- Không sửa code trong task này.
- Không thiết kế "RuntimeGraphStore" cho phần lõi dùng chéo — đây là thay
  đổi kiến trúc lớn, cần 1 Investigate riêng SAU KHI 5 task Move ở trên
  hoàn tất và có thêm test coverage cho `OrcaRuntimeService`.
- "Mobile session tabs" (CRUD/listing/headless hydration, rải rác
  3,140–20,479, 106 method theo cụm từ khoá) chưa có ranh giới field co
  cụm rõ ràng — cần phân tích field-span riêng (tương tự cách làm ở đây)
  trước khi sinh task Move cho domain này.

## Nguyên tắc bắt buộc cho MỖI task Move sinh ra (036–040)

1. `gitnexus impact` cho method đại diện của domain trước khi tách.
2. Đọc đúng dải dòng đã ước tính ở trên để XÁC NHẬN lại ranh giới thật
   (dữ liệu trên suy từ grep/awk, chưa đọc code — có thể lệch vài dòng).
3. 1 domain = 1 commit riêng, chạy toàn bộ test PTY/terminal/worktree liên
   quan (không chỉ test riêng domain) sau mỗi bước — class này là trung
   tâm của investigation `BUG-FE-PTY-001` kéo dài nhiều phiên.
4. Thứ tự thực hiện đề xuất: **036 (Automation) trước** — nhỏ nhất, ít
   field lõi nhất, dùng để xác nhận pattern composition hoạt động đúng
   trước khi làm task 037 (lớn nhất, ~1,830 dòng).
