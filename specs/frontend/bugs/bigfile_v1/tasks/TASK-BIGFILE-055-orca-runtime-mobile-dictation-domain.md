# TASK-BIGFILE-055 — Move: Mobile dictation/speech domain

**Loại:** Move — composition pattern · **Effort:** S · **Phụ thuộc:**
TASK-BIGFILE-054
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sinh ra từ khảo sát field-span TASK-BIGFILE-054 — 1 trong 8 "đảo an toàn"
tìm thấy trong vùng Terminal/PTY/Agent-status core còn lại. `mobileDictation`
là field private hoàn toàn cô lập (không bị 2 method dọn dẹp
`pruneDisconnectedPtyTranscript`/`pruneDisconnectedPtyRecords` chạm tới
như phần lớn field per-PTY khác).

## Kết quả thực thi (2026-08-11)

- Domain: `listMobileSpeechModels`, `downloadMobileSpeechModel`,
  `deleteMobileSpeechModel`, `configureMobileDictation`,
  `startMobileDictation`, `feedMobileDictation`, `finishMobileDictation`,
  `cancelMobileDictation`, `cancelMobileDictationSession` (private),
  `cancelMobileDictationForConnection`, `cancelMobileDictationForClient`
  (dòng gốc 3989–4295, 307 dòng kể cả comment mở đầu).
- Chỉ 1 host dependency thật: `getStore()`. Field `mobileDictation` chuyển
  hẳn thành private field của class mới (không dùng ngoài domain).
- 1 method cần public + forwarding: `cancelMobileDictationForClient` — được
  gọi từ host wiring của `orca-runtime-mobile-floor.ts` (domain đã tách từ
  TASK-BIGFILE-037, `cancelMobileDictationForClient: (clientId) =>
  this.cancelMobileDictationForClient(clientId)`), phát hiện trước khi viết
  (không phải sau khi tsc báo lỗi, nhờ kiểm tra `grep -rn` cross-file trước).
- Import free-function/type: `getDefaultVoiceSettings` (move hẳn),
  `getSpeechModelManager`/`getSpeechSttService`/`getCatalogModel`/
  `isLocalSpeechModel`/`SPEECH_MODEL_CATALOG`/`deleteLocalSpeechModel`/
  `getSpeechModelDeletionErrorCode`/`RuntimeSpeechModelSummary`/
  `RuntimeSpeechSetupState` (move hẳn, không dùng nơi khác trong
  `orca-runtime.ts`). `VoiceSettings`/`FLOATING_TERMINAL_WORKTREE_ID` (giữ
  nguyên ở `orca-runtime.ts` — vẫn dùng nơi khác, ví dụ `voice?:
  VoiceSettings` ở kiểu `ClientSettings` dòng 439 — suýt bỏ sót vì lần đếm
  đầu gộp nhầm usage ngoài cụm vào trong cụm, phát hiện lại khi đối chiếu kỹ
  trước khi xoá import).
- `orca-runtime.ts`: 10,760 → **10,467 dòng** (giảm 293 dòng, gồm cả field
  `mobileDictation` bị bỏ sót lúc đầu — gây lỗi `tsc TS6133` tạm thời, phát
  hiện và xoá ngay). File mới: 349 dòng (304 dòng non-blank/non-comment,
  nhỉnh hơn ngưỡng 300 một chút) — đã đăng ký
  `config/max-lines-baseline.txt` + `eslint-disable max-lines` inline.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới). `oxlint` sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi
  phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý, rủi ro thấp hơn nhiều so với phần lớn PTY-core
  (field hoàn toàn cô lập, chỉ 1 host dep). Khuyến nghị kiểm thử thủ công
  luồng mobile dictation (start/feed/finish/cancel, cả khi client ngắt kết
  nối giữa chừng) trước khi merge.
- Còn 7 domain nhỏ khác được TASK-BIGFILE-054 xác định an toàn (056–062):
  account-services, pty-title-tracker, connection-subscription-notify,
  pty-wait-blocked-check, pty-foreground-agent-refresh,
  terminal-message-waiter, remote-terminal-view-subscriber.

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **10,467 dòng (60.8% giảm)** qua 24 task
(TASK-BIGFILE-036 đến 055, không tính TASK-054 là Investigate).
