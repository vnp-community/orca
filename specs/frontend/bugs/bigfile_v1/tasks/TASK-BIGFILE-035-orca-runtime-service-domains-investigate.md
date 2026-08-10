# TASK-BIGFILE-035 — Investigate: `OrcaRuntimeService` domain boundaries (~23,450 dòng)

**Loại:** Investigate (KHÔNG thực thi split ngay) · **Effort:** L
**Phụ thuộc:** TASK-BIGFILE-008, 009 đã xong (file nguồn lúc này còn
~23,450 dòng thay vì 26,730)
**Status:** ⬜ Todo
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` (Giai
đoạn 3) — **làm SAU CÙNG trong toàn bộ `bigfile_v1`** (xem thứ tự #10 trong
`../solutions/SOLUTION-FE-BIGFILE-001-strategy-and-sequencing.md`)

## Input

- File nguồn: `frontend/src/main/runtime/orca-runtime.ts`
- Đọc `class OrcaRuntimeService` (dòng 2,109 gốc → gần hết file, sau
  TASK-008/009 chiếm gần như toàn bộ phần còn lại) — đọc theo từng domain,
  KHÔNG đọc tuyến tính toàn bộ 1 lần (dùng danh sách type đã tách ở
  `orca-runtime-types.ts`, TASK-BIGFILE-009, làm bản đồ định hướng: method
  nào thao tác trên type nào).
- Đọc lại đầy đủ comment dòng 1 (đã trích 1 phần trong bug/solution doc):
  "OrcaRuntimeService still owns the mutable live graph, PTY handles,
  waiters, mobile floor/layout state, and managed-worktree reconciliation."

## Nhiệm vụ

1. Xác định ranh giới method theo 4 domain gợi ý trong solution doc (theo
   đúng thứ tự đề xuất, xác nhận lại khi đọc):
   - Mobile session mirror / floor-layout state (liên quan
     `MobileNotificationDispatchEvent`, `PtyLayoutState`, `ApplyLayoutResult`)
   - Automation (`RuntimeAutomationCreateInput`/`UpdateInput`)
   - PTY liveness/waiters
   - Worktree reconciliation (làm sau cùng — phần lõi còn lại)
2. Với MỖI domain, liệt kê: tên method, dòng bắt đầu/kết thúc, field
   private nào của class được method đó dùng — xác định field nào CHỈ dùng
   trong 1 domain (an toàn để tách theo composition) vs field dùng CHÉO
   nhiều domain (giữ lại ở class chính, truyền qua constructor cho domain
   controller).
3. Thiết kế theo pattern composition đã nêu trong solution doc (field
   `private mobileSession = new MobileSessionMirrorController({...})`,
   forward call, giữ nguyên public method signature).

## Output

- Cập nhật `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md` Giai đoạn
  3 với bảng method→domain→dòng cụ thể (thay thế mô tả định hướng hiện tại).
- Task Move mới (`TASK-BIGFILE-036`, ... tiếp theo dãy số hiện có, HOẶC tiếp
  sau các task được sinh ra từ TASK-BIGFILE-018/019/026/032/034 nếu đã có —
  kiểm tra `TASKS-INDEX.md` để lấy đúng số tiếp theo) — 1 task cho MỖI domain
  (4 task), theo đúng thứ tự: mobile-session → automation → pty-liveness →
  worktree reconciliation.

## Không làm trong task này

Không sửa code. Đây là class rủi ro cao nhất trong toàn bộ `bigfile_v1` —
trung tâm của gần như mọi flow terminal/PTY/worktree/mobile-session, đã liên
quan trực tiếp investigation `BUG-FE-PTY-001` kéo dài nhiều phiên làm việc.
Mỗi task Move sinh ra từ đây bắt buộc: 1 domain/commit, chạy toàn bộ test
liên quan PTY/terminal/worktree (không chỉ test của riêng domain đó) sau mỗi
bước, và cân nhắc test thủ công trên môi trường thật trước khi coi là hoàn
tất.
