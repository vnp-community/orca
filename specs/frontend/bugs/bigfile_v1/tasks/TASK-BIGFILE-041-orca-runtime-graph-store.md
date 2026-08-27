# TASK-BIGFILE-041 — Extract: `RuntimeGraphStore` (state container, not a domain move)

**Loại:** Extract — state container, KHÔNG phải Move theo domain (khác
TASK-036–040: đây là bước dọn dẹp kiến trúc để CHUẨN BỊ cho các domain
move sau này, không tự nó là 1 domain nghiệp vụ) · **Effort:** L (rủi ro
kỹ thuật thấp nhờ tsc bắt lỗi toàn bộ, nhưng phạm vi tham chiếu rất rộng —
225 chỗ sửa) · **Phụ thuộc:** TASK-BIGFILE-008, 009, 035
**Status:** ✅ Done (commit theo sau ghi chú này)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3 — phần "state lõi", trước đây đánh giá "không tách được")

## Bối cảnh

TASK-BIGFILE-035 xác định state "lõi" (`ptysById`, `handles`, `leaves`,
`tabs`, ...) dùng chéo 15,000–21,000+ dòng trong `OrcaRuntimeService`,
đánh giá "không tách được bằng Move cơ học". Task này **không tách theo
domain nghiệp vụ** (không thể — chưa có ranh giới domain nào sở hữu state
này riêng) mà tách theo **vai trò kỹ thuật**: 13 field tạo thành "graph"
(leaf/tab/handle/waiter) được gom vào 1 class chứa dữ liệu thuần
(`RuntimeGraphStore`, không có method/logic), `OrcaRuntimeService` giữ 1
field `private readonly graph = new RuntimeGraphStore()` và truy cập qua
`this.graph.<field>` thay vì `this.<field>`.

**Đây KHÔNG phải bước cuối** — nó không giảm được nhiều dòng
(24,733 → 24,723, chỉ ~10 dòng) vì bản chất là đổi chỗ truy cập
(`this.X` → `this.graph.X`), không xoá logic. Giá trị thật: **tách rời
được state khỏi hành vi**, là tiền đề bắt buộc để sau này có thể (a) viết
test cho riêng phần graph, (b) hoặc tiếp tục tách các domain còn lại
(pty-lifecycle, worktree reconciliation) vì giờ chúng có thể nhận
`RuntimeGraphStore` qua constructor thay vì đọc field private trực tiếp
của `OrcaRuntimeService`.

## 13 field đã chuyển

`rendererGraphEpoch`, `graphStatus`, `authoritativeWindowId`, `tabs`,
`leaves`, `leavesByPtyId`, `handles`, `handleByLeafKey`, `handleByPtyId`,
`detachedPreAllocatedLeaves`, `graphSyncCallbacks`, `waitersByHandle`,
`ptysById`.

**Cố ý KHÔNG chuyển** `ptyController`, `notifier`, `_orchestrationDb`,
`stats`, `agentDetector` dù cũng có span lớn — đây là **collaborator
reference** (interface được inject qua setter, không phải dữ liệu graph),
khác bản chất với 13 field trên. Để riêng cho 1 task khác nếu cần (không
có trong phạm vi TASK-035's danh sách "graph").

## Cách làm — mass-reference-rewrite có xác minh bằng compiler, KHÔNG grep-and-hope

1. Tạo `orca-runtime-graph-store.ts`: class `RuntimeGraphStore` chứa đúng
   13 field, GIỮ NGUYÊN type + initializer gốc (không đổi `readonly` vì 3
   field — `tabs`, `leaves`, `leavesByPtyId` — bị gán lại nguyên khối ở vài
   chỗ, xác nhận qua `grep -E "this\.<field>\s*=[^=]"` trước khi quyết định
   readonly hay không).
2. 2 type private dùng riêng bởi các field này (`TerminalHandleRecord`,
   `TerminalWaiter`) nhưng ALSO dùng ở nơi khác trong class (dòng
   ~21,446/21,466 và ~10,695/21,721/21,808/21,965/22,006) → thêm `export`,
   import type ngược lại vào `orca-runtime.ts` — pattern giống hệt
   `RuntimeLeafRecord`/`RuntimePtyWorktreeRecord`/`ResolvedWorktree` ở
   TASK-008.
3. Xoá 13 dòng khai báo field khỏi `OrcaRuntimeService`, thêm 1 dòng
   `private readonly graph = new RuntimeGraphStore()`.
4. **Script Python thay thế toàn bộ `this.<field>` → `this.graph.<field>`**
   cho cả 13 tên field, trên TOÀN BỘ file (không giới hạn theo method) —
   225 chỗ. Trước khi chạy, xác nhận KHÔNG có pattern `this['<field>']`
   (bracket access) hay destructuring `const { <field> } = this` bằng
   grep riêng — nếu có, phải sửa tay trước/sau khi chạy script.
5. `tsc --noEmit` làm lưới an toàn: bất kỳ chỗ nào sót lại dùng
   `this.<field>` (không qua `this.graph.`) sẽ báo lỗi "Property does not
   exist on type 'OrcaRuntimeService'" ngay lập tức — không cần tự tin
   100% vào bước 4, để compiler xác nhận. Kết quả: 251 lỗi pre-existing
   (baseline không đổi, xác nhận qua so sánh trước/sau) → 0 lỗi mới ngay
   từ lần chạy đầu (không phải sửa sót).
6. 2 import bị thừa sau khi field rời đi (`RuntimeGraphStatus`,
   `RuntimeSyncedTab` — giờ chỉ dùng trong file mới) → xoá khỏi
   `orca-runtime.ts`.

## Xác minh đã làm

- [x] `tsc --noEmit --composite false`: 251 lỗi pre-existing không đổi
      (baseline xác nhận qua so sánh trước/sau thay đổi) → 0 lỗi mới.
- [x] `oxlint` (cả 2 config): sạch (exit 0) trên cả 2 file.
- [x] `node scripts/find-frontend-bigfiles.mjs`: `orca-runtime.ts`
      24,733 → 24,723 dòng (giảm không đáng kể — ĐÚNG NHƯ DỰ KIẾN, xem
      "Bối cảnh" ở trên). File mới `orca-runtime-graph-store.ts`: 35 dòng.
- [ ] `gitnexus detect_changes` — **KHÔNG dùng được**: phát hiện quan
      trọng trong lúc làm task này — `orca-runtime.ts` (cả 3 bản
      backend/desktop/frontend) bị GitNexus loại khỏi index hoàn toàn do
      giới hạn kích thước file mặc định 512KB (`orca-runtime.ts` bản
      frontend ~905KB) — KHÔNG PHẢI vấn đề index cũ như cảnh báo hook vẫn
      lặp lại. Cần `GITNEXUS_MAX_FILE_SIZE` lớn hơn + `analyze --force`
      nếu muốn dùng gitnexus cho file này trong tương lai — ngoài phạm vi
      task này.
- [ ] Test PTY/terminal/worktree liên quan — **CHƯA CHẠY trong task này**
      (môi trường test cần xác nhận riêng); class này không có test
      coverage sẵn theo báo cáo trước đó — khuyến nghị chạy test thủ công
      trên môi trường thật (spawn/kill PTY, mở nhiều tab, disconnect
      mobile client) trước khi coi thay đổi này là an toàn để deploy.

## Rollback

```
git checkout -- frontend/src/main/runtime/orca-runtime.ts
rm frontend/src/main/runtime/orca-runtime-graph-store.ts
```

## Việc tiếp theo (không nằm trong task này)

- `ptyController`/`notifier`/`_orchestrationDb` (collaborator reference,
  không phải graph data) — cân nhắc 1 task riêng nếu muốn tiếp tục giảm
  field trực tiếp trên `OrcaRuntimeService`.
- TASK-BIGFILE-036–040 (domain move) giờ CÓ THỂ inject `RuntimeGraphStore`
  qua constructor thay vì phải truyền từng field private lẻ — thiết kế
  domain controller ở các task đó nên tham chiếu lại quyết định này.
