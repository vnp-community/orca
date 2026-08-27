# TASK-BIGFILE-063 — Extract: `RuntimePtyTranscriptStore` (state container, không phải Move theo domain)

**Loại:** Extract — state container, KHÔNG phải Move theo domain (giống
TASK-BIGFILE-041's `RuntimeGraphStore`: bước dọn dẹp kiến trúc để CHUẨN BỊ
cho các domain Move sau này, không tự nó là 1 domain nghiệp vụ) ·
**Effort:** M (rủi ro kỹ thuật thấp nhờ tsc bắt lỗi toàn bộ, phạm vi tham
chiếu vừa phải — 60 chỗ sửa, nhỏ hơn nhiều so với 225 chỗ ở TASK-041) ·
**Phụ thuộc:** TASK-BIGFILE-041, 054
**Status:** ✅ Done (commit theo sau ghi chú này)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau khi hoàn tất cả 8 domain "đảo an toàn" từ TASK-BIGFILE-054 (055–062,
trừ 057 đã huỷ), phần còn lại của `orca-runtime.ts` (~10,000 dòng) là
đúng phần TASK-054 gọi là "lõi thật": `graph` (đã tách state ở TASK-041),
`headlessTerminals`, và **10 field OSC-parsing/transcript-per-PTY**
(`recentPtyOutputById`, `ptyOutputSequenceById`,
`recentPtyPathCandidatesById`, `agentStatusOscProcessorsByPtyId`,
`ptyTitleTrackersByPtyId`, `terminalSpawnCommandsByPtyId`,
`oscTitleScanTailByPtyId`, `osc7ScanTailByPtyId`, `terminalCwdByPtyId`,
`terminalFileUriHostnameByPtyId`) — bị 2 method dọn dẹp chung
(`dropDisconnectedPtyRecord`, `onPtyExit`) chạm trực tiếp VÀ bị các method
xử lý byte-PTY nóng nhất (`onPtyData`, `applyTrackedPtyTitle`,
`processAgentStatusOscForPty`, `extractLastOsc7CwdForPty`, ...) dùng dày
đặc — không tách được bằng Move (không có ranh giới domain rõ).

Áp dụng đúng chiến lược TASK-041 đã dùng cho `graph`: **tách theo vai trò
kỹ thuật** (10 field cùng "shape" — `Map<ptyId, X>`, cùng bị dọn dẹp khi
PTY ngắt kết nối) vào 1 class chứa dữ liệu thuần, không có method/logic.
Đây KHÔNG phải bước cuối — không giảm được nhiều dòng (chỉ đổi chỗ truy
cập `this.X` → `this.ptyTranscripts.X`) nhưng tách rời được state khỏi
hành vi, tiền đề cho các domain-Move sau này (title-tracker, OSC-status
processing) có thể nhận `RuntimePtyTranscriptStore` qua constructor thay
vì đọc field private trực tiếp của `OrcaRuntimeService`.

## 10 field đã chuyển

`recentPtyOutputById`, `ptyOutputSequenceById`, `recentPtyPathCandidatesById`,
`agentStatusOscProcessorsByPtyId`, `ptyTitleTrackersByPtyId`,
`terminalSpawnCommandsByPtyId`, `oscTitleScanTailByPtyId`,
`osc7ScanTailByPtyId`, `terminalCwdByPtyId`, `terminalFileUriHostnameByPtyId`.

**Cố ý KHÔNG chuyển**: `headlessTerminals` (vai trò khác — headless
terminal emulator instance, không phải OSC-scan cache; cụm method riêng
`seedHeadlessTerminal`…`disposeHeadlessTerminal`, ~470 dòng, có thể là
ứng viên state-container/domain riêng sau này), `latestAgentStatusByPaneKey`
(khoá theo `paneKey` chứ không phải `ptyId` — index khác bản chất),
`headlessHydrationState` (theo dõi hydration mobile-session, không phải
transcript). Cả 3 field này span rộng tương tự nhưng KHÔNG cùng vai trò
kỹ thuật với 10 field trên — để lại cho task riêng nếu cần.

## Cách làm — mass-reference-rewrite có xác minh bằng compiler

1. Tạo `orca-runtime-pty-transcript-store.ts`: class
   `RuntimePtyTranscriptStore` chứa đúng 10 field, GIỮ NGUYÊN type +
   initializer gốc (tất cả `readonly` — xác nhận qua
   `grep -n "this\.<field>\s*=[^=]"` KHÔNG có kết quả cho cả 10 field,
   nghĩa là không có reassignment nguyên khối nào, khác 3/13 field ở
   TASK-041 phải giữ non-readonly).
2. `RuntimePtyTitleTrackerEntry` (type private dùng trong field
   `ptyTitleTrackersByPtyId` NHƯNG cũng dùng ở nơi khác trong class —
   dòng ~2360/2590/2681, các method title-tracker KHÔNG chuyển) → thêm
   `export`, import lại vào file mới — pattern giống hệt TASK-041's
   `TerminalHandleRecord`/`TerminalWaiter`.
3. Xoá 10 dòng khai báo field khỏi `OrcaRuntimeService`, thêm 1 dòng
   `private readonly ptyTranscripts = new RuntimePtyTranscriptStore()`.
4. Script Python thay thế toàn bộ `this.<field>` → `this.ptyTranscripts.<field>`
   cho cả 10 tên field, trên TOÀN BỘ file — 60 chỗ. Xác nhận trước KHÔNG
   có bracket access (`this['<field>']`) hay destructuring
   (`const { <field> } = this`) — cả 2 đều rỗng.
5. `tsc --noEmit` làm lưới an toàn: 251 lỗi pre-existing không đổi → 0 lỗi
   mới **ngay từ lần chạy đầu** (không phải sửa sót nào, khác TASK-041 —
   phạm vi nhỏ hơn nhiều nên rủi ro sót thấp hơn).
6. `createAgentStatusOscProcessor` cần trong file mới chỉ để lấy kiểu
   `ReturnType<typeof createAgentStatusOscProcessor>` — `oxlint`
   (`consistent-type-imports`) bắt lỗi ngay, sửa thành `import type`.

## Xác minh đã làm

- [x] `tsc --noEmit --composite false`: 251 lỗi pre-existing không đổi →
      0 lỗi mới, sạch ngay từ lần chạy đầu.
- [x] `oxlint` (cả 2 config): sạch (exit 0) trên cả 2 file, sau khi sửa
      `import` → `import type` cho `createAgentStatusOscProcessor`.
- [x] `pnpm check:max-lines-ratchet`: 647 vi phạm pre-existing không đổi.
- [x] `orca-runtime.ts`: 10,006 → **9,980 dòng** (giảm ~26 dòng — như
      TASK-041, giá trị thật KHÔNG phải số dòng mà là tách rời state
      khỏi hành vi). File mới: 50 dòng.

## Việc tiếp theo

Với `RuntimePtyTranscriptStore` đã tách, các cụm method sau giờ CÓ THỂ
xem xét tách thành domain riêng (nhận `RuntimePtyTranscriptStore` qua
constructor thay vì đọc field private) — nhưng vẫn cần đọc kỹ method-body
dependency trước (bài học TASK-057):
- Title-tracking: `getTrackedRawTitleForPty`, `preferTrackedLastTitle`,
  `makeMobileTitleGateKey`, `getOrCreatePtyTitleTrackerEntry`,
  `applyTrackedPtyTitle`, `disposePtyTitleTracker` (~5 method core dùng
  `ptyTitleTrackersByPtyId`, gọi ngược sang `pty-foreground-agent-refresh`
  đã tách — TASK-060).
- OSC-status processing: `processAgentStatusOscForPty`,
  `emitTerminalAgentStatusEvents` và các helper OSC 7 (`extractLastOsc7CwdForPty`,
  `recordOsc7MetadataForPty`, `pathFlavorForPty`) — trung tâm của
  `onPtyData`, rủi ro cao nhất trong toàn bộ file (hiệu năng nhạy cảm,
  "đo ~85% chi phí onPtyData dưới TUI flood").
- `headlessTerminals` cluster (`seedHeadlessTerminal`…`disposeHeadlessTerminal`,
  ~470 dòng) — vai trò khác, ứng viên state-container riêng.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **9,980 dòng (62.7% giảm)** qua 31 task
(TASK-BIGFILE-036 đến 063, trừ 057 đã huỷ; 041 và 063 là state-container
Extract, không phải domain Move).
