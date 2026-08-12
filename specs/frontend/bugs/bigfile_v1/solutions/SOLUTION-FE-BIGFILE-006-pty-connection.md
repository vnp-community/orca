# SOLUTION-FE-BIGFILE-006 — Tách `pty-connection.ts` (7,600 dòng)

**Bug:** `../BUG-FE-BIGFILE-006-pty-connection.md`
**Trạng thái:** 🔴 Blocked (2026-08-11, sau TASK-BIGFILE-034 — xem Bước 2:
không tìm được ranh giới Move an toàn, cần redesign kiến trúc trước, ngoài
phạm vi cơ học Move/Investigate của đợt `bigfile_v1`)
**Thứ tự thực hiện:** #9 (gần cuối — xem lý do trong `SOLUTION-FE-BIGFILE-001`
mục 3: không có ranh giới export sẵn + vừa dính bug race-condition gần đây)
**Chiến lược:** Barrel/facade cho phần đã tách; **thiết kế lại nội bộ** cho
`connectPanePty` (không có shortcut barrel đơn giản vì chỉ có 1 export)

---

## Khác biệt so với các file khác trong đợt này

`pty-connection.ts` chỉ có 2 export: `STARTUP_CWD_FALLBACK_NOTICE` (const,
dòng 250) và `connectPanePty` (dòng 943 → hết file, ~6,650 dòng cho 1 hàm).
**Không có ranh giới sẵn** như `BrowserPane.tsx`/`TaskPage.tsx` — không thể
chỉ "di chuyển nguyên khối theo export có sẵn". Cần đọc + thiết kế nội bộ
hàm trước khi tách được.

## Bước 0 — Bắt buộc trước khi tách bất kỳ dòng nào: tăng test coverage

File này vừa là trung tâm của investigation `BUG-FE-PTY-001` (race condition
giữa local tab và host-session mirror tab, dẫn tới PTY bị đóng nhầm — xem
memory session `bug-fe-pty-001-investigation.md`). Việc tách 1 hàm 6,650 dòng
KHÔNG có test che phủ tốt có rủi ro regression cao hơn nhiều so với các file
khác trong đợt này.

**Trước khi tách**, xác nhận/bổ sung test cho tối thiểu:
1. Luồng spawn mới (local/SSH/remote-runtime) thành công.
2. Luồng retry khi `SSH_SESSION_EXPIRED` (đã fix trong investigation gần
   đây — đảm bảo có test regression cho fix đó nếu tách vô tình đổi hành vi).
3. Luồng reattach/daemon reattach.
4. Race giữa local pane và host-session mirror pane cho cùng 1 leaf (nếu
   logic liên quan còn nằm trong file này — cần xác nhận khi đọc).

## Bước 1 — Đọc và phân vùng nội bộ `connectPanePty` (kết quả TASK-BIGFILE-034, 2026-08-11)

**Đã đọc toàn bộ hàm** (dòng 943 → 7600, ~6,658 dòng — khớp gần như chính
xác với ước lượng ~6,650 dòng gốc; khác với vụ `ipc/pty.ts` trước đây, con
số trong task doc KHÔNG sai). Kết luận thực tế **khác đáng kể** so với giả
định "3–4 nhánh lớn theo transport" ở trên:

**Không có ranh giới theo transport ở cấp khối lớn.** Toàn bộ hàm chia làm
2 phần rất lệch nhau về bản chất:

1. **~5,800 dòng (943–6737, 87%)**: pipeline dùng CHUNG, giống hệt nhau cho
   mọi loại transport — theo dõi title/agent-status (~1,300 dòng),
   state machine "hidden output restore" (dòng 4885–6325, ~1,440 dòng riêng
   phần này), pipeline render/ghi dữ liệu PTY vào xterm
   (`writePtyOutputToXterm`, `dataCallback`, ...), xử lý replay/snapshot,
   startup-command delivery (paste/sendInput), hibernation/cold-restore
   resume. Phần này KHÔNG rẽ nhánh theo transport ở cấp khối — nó rẽ nhánh
   **rải rác ở cấp dòng lẻ**: `isRemoteRuntimePtyId(...)` xuất hiện 24 lần
   trong toàn hàm, trải từ dòng 1888 đến 7161 (không gom cụm); tương tự
   `connectionId` (39 lần). Tức là "local vs SSH vs remote-runtime" là 1
   mối quan tâm xuyên suốt gần như MỌI closure trong hàm, không phải 1 khối
   tách rời được.
2. **~860 dòng (6740–7600, 13%)**: phần thực sự ra quyết định theo
   transport —
   - `6740–7017` (277 dòng): nhánh SSH (`connectionId` khác null, không
     phải devServer) — gate passphrase/deferred-reconnect, gọi
     `resolveSshPaneConnectGate`, tự `return` ở cuối.
   - `7022–7328` (306 dòng): cây quyết định reattach/attach/pending-spawn/
     fresh-spawn dùng CHUNG cho local VÀ remote-runtime (phân biệt bằng
     `isRemoteRuntimePtyId()` inline, không phải 2 khối riêng).
   - `7330–7600` (~270 dòng): hạ tầng dùng chung khác (`reconcileIfSessionDead`,
     hibernation wake, dispose) — không theo transport.

**Đo mức độ chia sẻ closure (tương đương "field-span" nhưng cho biến đóng,
theo đúng phương pháp học được ở TASK-BIGFILE-054/057)**: đếm số định danh
ngoại vi (outer-scope) phân biệt mà mỗi khối lớn nhất tham chiếu tới —

| Khối | Dòng | Số định danh ngoại vi phân biệt | Gồm cả closure sâu khác? |
|---|---|---|---|
| Nhánh SSH (`connectionId`) | 6740–7017 (277) | **21** (`deps`, `connectionId`, `disposed`, `startFreshColdRestoreAgentResume`, `reportError`, `cacheKey`, `waitTeardowns`, `pane`, `transport`, `handleReattachResult`, `dataCallback`, `replayDataCallback`, ...) | Có — `startFreshColdRestoreAgentResume`/`handleReattachResult` tự đóng thêm hàng chục biến khác |
| Cây quyết định reattach/attach/spawn | 7022–7328 (306) | **29** (`deps`, `pane`, `transport`, `reportError`, `startFreshSpawn`, `dataCallback`, `replayDataCallback`, `clearHiddenOutputRestoreState`, `bindActivePanePty`, ...) | Có — `startFreshSpawn`/`clearHiddenOutputRestoreState`/`dataCallback` tự đóng vào toàn bộ state machine "hidden output restore" 1,440 dòng ở trên |

21–29 định danh ngoại vi phân biệt cho MỖI khối, trong đó nhiều định danh
là chính các closure khổng lồ khác (không phải giá trị đơn giản) — đây là
mức độ vướng víu **tương đương** trường hợp `ptyTitleTrackersByPtyId`
(TASK-BIGFILE-057, đã huỷ sau khi phát hiện method-body gọi ra ~10 method
core khác) và trường hợp `ipc/pty.ts` (đã blocked toàn bộ nhóm). Khác biệt
so với 057/ipc-pty.ts: ở đây sự vướng víu là closure **cục bộ trong 1 lần
gọi hàm** (không phải module-state dùng chéo bởi nhiều caller ngoài phạm
vi), nhưng hệ quả thực hành giống nhau — không thể "copy nguyên văn" 1 khối
sang file mới mà không kéo theo ~20–30 tham số/callback.

`gitnexus impact({target: "connectPanePty", direction: "upstream"})` không
chạy được trong phiên này (index đang stale — lần index gần nhất trên
`main`, nhánh hiện tại `fix/pty-session-expired-on-pane-remount` đã lệch;
CLI `gitnexus impact` segfault khi thử chạy trực tiếp, MCP tool báo
"Connection closed") — không chặn kết luận Investigate này vì không có
sửa code, nhưng cần chạy lại (sau khi `gitnexus analyze` cập nhật index)
trước khi bất kỳ ai thử Move thật.

## Bước 2 — Quyết định: KHÔNG đủ an toàn để Move theo cả 2 phương án

**Phương án A (strategy pattern, 3 file riêng theo transport): loại bỏ.**
Không có ranh giới theo transport ở cấp khối — 87% hàm là pipeline dùng
chung, rẽ nhánh transport rải rác ở cấp dòng lẻ trong hầu hết closure. Tách
thành 3 strategy riêng đòi hỏi viết lại (không phải di chuyển nguyên văn)
gần như toàn bộ hàm để tách phần "chung" ra khỏi phần "riêng" đang đan xen
— đây là 1 redesign kiến trúc, không phải Move.

**Phương án B (trích khối lớn thành hàm con export riêng): CŨNG không đủ an
toàn ở dạng "Move nguyên văn"** như các task Move khác trong đợt này. Cả 2
khối lớn nhất (SSH gate, cây quyết định reattach/attach/spawn) đều có 21–29
phụ thuộc closure ngoại vi, nhiều cái là chính các closure khổng lồ khác.
Trích xuất đúng nghĩa "hàm con export riêng, gọi từ `connectPanePty`" đòi
hỏi đóng gói 20–30 phụ thuộc đó thành 1 object context tường minh — về bản
chất đây CHÍNH LÀ bước đầu của phương án A (cần 1 `ConnectionContext`/state-
owner tường minh trước, giống cách `RuntimeGraphStore` được tách ra ở
TASK-BIGFILE-041 cho `orca-runtime.ts`), không phải 1 bước Move độc lập,
rủi ro thấp.

**Kết luận: KHÔNG sinh task Move con nào cho `connectPanePty` trong đợt
`bigfile_v1` này.** Xử lý giống các trường hợp đã blocked trước đó
(`ipc/pty.ts`, domain `ptyTitleTrackersByPtyId`) — ghi nhận là cần 1 bước
thiết kế lại kiến trúc riêng (đưa state/closure hiện có vào 1 context object
tường minh) TRƯỚC KHI có thể tách an toàn, việc đó nằm ngoài phạm vi
Investigate/Move cơ học của đợt này.

**Việc thay thế có giá trị thấp nhưng an toàn tuyệt đối** (không tạo task
riêng vì giá trị quá nhỏ so với quy mô file): một số hàm nested dùng
`function` (không phải closure `const`) như `trailingIncompleteCsiSequence`
(dòng 4463–4482) là pure function thật sự (0 phụ thuộc closure ngoài tham
số + hằng số module) — có thể nâng lên module-level bất kỳ lúc nào mà không
rủi ro, nhưng chỉ tiết kiệm ~20 dòng, không đáng 1 task riêng.

## Bước 3+ — KHÔNG áp dụng cho tới khi có redesign kiến trúc (xem Bước 2)

Các bước dưới đây mô tả kế hoạch NẾU 1 redesign kiến trúc (context object
tường minh cho state/closure của `connectPanePty`) được thực hiện trong
tương lai — giữ lại làm tham khảo, không phải việc cần làm ngay trong đợt
`bigfile_v1`.

## Bước 3 — Tách từng nhánh, xác nhận xanh sau MỖI nhánh

Không tách 3+ nhánh trong 1 commit. Thứ tự đề xuất: nhánh ít rủi ro nhất
trước (local) → SSH → remote-runtime (rủi ro cao nhất, vừa có bug gần đây) →
cuối cùng mới rút gọn `connectPanePty` thành hàm điều phối.

## Xác minh (sau MỖI nhánh, không chỉ cuối cùng)

- `pnpm run typecheck`, `pnpm run lint`
- Toàn bộ test đã bổ sung ở Bước 0 + test hiện có
- `gitnexus impact({target: "connectPanePty", direction: "upstream"})` +
  `gitnexus detect_changes({scope: "all"})`
- **Test thủ công trên môi trường thật nếu có thể** (tương tự cách
  investigation `BUG-FE-PTY-001` đã làm — deploy lên môi trường dev, thử tạo
  terminal devServer thật) — vì đây là logic đã có lịch sử race-condition
  khó phát hiện chỉ bằng unit test.
- `node scripts/find-frontend-bigfiles.mjs`

## Rủi ro

**Cao nhất trong toàn bộ 10 file** — không phải vì độ lớn (7,600 dòng, đứng
thứ 5) mà vì: (1) không có ranh giới export sẵn, cần thiết kế lại thay vì chỉ
di chuyển, (2) đã có lịch sử bug race-condition nghiêm trọng, khó phát hiện
qua test thông thường (cần test trên môi trường thật với timing thực). Đây là
lý do đặt thứ tự #9 (gần cuối) — chỉ nên làm sau khi đã có kinh nghiệm từ các
file rủi ro thấp hơn.
