# TASK-BIGFILE-075 — Move: terminal-send domain

**Loại:** Move — composition pattern, rủi ro thấp (3 host dependency, cụm
sạch nhất từ trước đến nay) · **Effort:** M · **Phụ thuộc:** không (chỉ
phụ thuộc core `graph`/`ptyController` sẵn có)
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Cụm thứ ba theo yêu cầu người dùng ("cả 3 cụm đề xuất"): rà quét
method utility nhỏ lẻ còn lại sau TASK-073/074. Sweep gap-analysis đã sửa
lỗi phương pháp lần nữa — regex tìm method signature trước đó bỏ sót tổ hợp
`private async methodName(` (chỉ khớp một modifier), khiến vài gap bị đánh
giá thiếu chính xác số dòng "thật". Sau khi mở rộng kiểm tra thủ công, phát
hiện cụm `sendTerminal`/`sendTerminalAgentPrompt` (public) +
`writeTerminalAction`/`writeTerminalInputChunks`/`writeTerminalAgentPrompt`
(private, độc quyền) — tách biệt rõ với cụm `getTerminalAgentStatus` nằm kế
bên (đọc trạng thái agent, không phải ghi PTY, dùng chung
`getPaneKeyForTerminalHandle`/`getTerminalAgentStatusSnapshot` với các
method khác ở xa — không đủ gọn để tách an toàn, để lại).

## Kết quả thực thi (2026-08-12)

- Domain: `sendTerminal`, `sendTerminalAgentPrompt` (dòng gốc 2860–2937) +
  `writeTerminalAction`, `writeTerminalInputChunks`, `writeTerminalAgentPrompt`
  (dòng gốc 3190–3304) — 2 đoạn không liền mạch (cụm `getTerminalAgentStatus`
  + noise forwarding-field của 4 domain khác nằm giữa).
- Chỉ 3 host dependency — cụm sạch nhất từ trước đến nay: `getPtyController`,
  `getLivePtyForHandle`, `getLiveLeafForHandle`.
- 2 method public giữ forwarding field, không có internal caller nào khác
  cần sửa ngoài 1 chỗ (`handleAgentTeamsTmuxCompat`'s `sendTerminal: (handle,
  action) => this.sendTerminal(handle, action)` — vẫn hoạt động nguyên vẹn
  vì field vẫn public).
- 7 import move-only dọn sạch (xác nhận qua `tsc` TS6133/TS6192/TS6196, toàn
  bộ đều độc quyền cụm này): `iterateTerminalInputChunks`,
  `AGENT_PROMPT_BRACKETED_PASTE_END`, `AGENT_PROMPT_SUBMIT`,
  `AGENT_PROMPT_SUBMIT_DELAY_MS`, `buildAgentPromptPasteBytes`,
  `assertTerminalInputWithinLimitWithYield`, `buildSendPayload`. 1 type
  move-only: `RuntimeTerminalSend`.
- Xác minh fidelity bằng diff nguyên văn (chuẩn hoá `this.host.X` → `this.X`)
  so với `git show HEAD:...` — khớp byte-for-byte.
- `orca-runtime.ts`: 6,528 → **6,336 dòng**. File mới
  `orca-runtime-terminal-send.ts`: 230 dòng (202 non-blank/non-comment) —
  dưới ngưỡng 300, không cần đăng ký `max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi thật, 5 lỗi move-only ban đầu). `oxlint` sạch (exit 0) cả 2 config.
  `max-lines-ratchet`: 647 vi phạm pre-existing không đổi.

## Rủi ro còn lại / khuyến nghị

- Rủi ro thấp — cụm 3 host dependency, không hot path (`onPtyData`/
  `onPtyExit`), logic độc lập rõ ràng (chunk text trước khi ghi PTY/ConPTY,
  tách suffix Enter/Ctrl-C, phục hồi bracketed-paste-end khi ghi agent
  prompt lỗi giữa chừng). Khuyến nghị test thủ công: gửi text dài (paste-sized)
  qua `terminal.send`, gửi agent prompt (agent teams / mobile), gửi
  Enter/Ctrl-C riêng lẻ, gây lỗi giữa chừng khi ghi paste để xác nhận
  bracketed-paste-end phục hồi đúng.
- Đã hoàn tất cả 3 cụm người dùng yêu cầu (073: createTerminal/splitTerminal/
  launchAgentTerminal, 074: onPtyData, 075: terminal-send là cụm utility đầu
  tiên tìm được). Sweep tiếp tục nếu người dùng muốn — `getTerminalAgentStatus`
  cluster (getTerminalAgentStatusPtyId, assertTerminalAgentStatusPtyBinding,
  getFreshExplicitAgentStatusForHandle) là candidate tiếp theo nhưng phức tạp
  hơn (dùng chung `getPaneKeyForTerminalHandle`/`getTerminalAgentStatusSnapshot`
  với method khác ở xa trong file — cần audit kỹ hơn trước khi tách).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **6,336 dòng (76.3% giảm)** qua 43 task
(TASK-BIGFILE-036 đến 075, trừ 057 đã huỷ rồi tái thực thi ở 067; 041 và 063
là state-container Extract).
