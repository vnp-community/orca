# TASK-BIGFILE-056 — Move: Account services (mobile RPC bridge) domain

**Loại:** Move — composition pattern · **Effort:** S · **Phụ thuộc:**
TASK-BIGFILE-054
**Status:** ✅ Done
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

2/8 "đảo an toàn" từ TASK-BIGFILE-054. `accountServices` field hoàn toàn
cô lập, không liên quan Terminal/PTY.

## Phát hiện: 2 đoạn không liền mạch

`setAccountServices` (setter) nằm tách biệt khỏi phần còn lại
(`requireAccountServices`…`onAccountsChanged`) — ở giữa là block wiring
composition của domain `mobile-dictation` (TASK-BIGFILE-055, trước đó vốn
là chính method `listMobileSpeechModels`…, đã tách). Xử lý như 2 đoạn rời,
gộp vào cùng 1 file mới — đúng mẫu "excluded chunk in the middle" đã gặp
nhiều lần trước.

## Kết quả thực thi (2026-08-11)

- Domain: `setAccountServices`, `requireAccountServices` (private),
  `getAccountsSnapshot`, `refreshAccountsForMobile`, `selectClaudeAccount`,
  `selectCodexAccount`, `removeClaudeAccount`, `removeCodexAccount`,
  `onAccountsChanged` (9 method, 2 đoạn: dòng gốc 3956–3961 và
  3999–4059, tổng ~66 dòng).
- **0 host dependency** — domain này không cần host object nào cả (khác
  mọi domain trước, vốn luôn có `host: XCommandHost`), vì
  `RuntimeAccountServicesCommands` tự chứa toàn bộ state
  (`accountServices` field) và không gọi ngược `OrcaRuntimeService`. Class
  không có constructor param — pattern hợp lý hơn 1 host rỗng giả
  (`Record<string, never>`).
- `RuntimeAccountServices` (type nội bộ, dòng 349) — thêm `export`, import
  lại qua `import type { RuntimeAccountServices } from './orca-runtime'`.
- **Phát hiện quan trọng**: `AccountsSnapshot` import tại đầu
  `orca-runtime.ts` (dòng 335, từ `./orca-runtime-types`) tưởng là "STAYS"
  vì có mặt trong khối `export type {...} from './orca-runtime-types'` ở
  cuối file — nhưng khối `export type` đó **re-export TRỰC TIẾP từ
  `./orca-runtime-types`**, không tham chiếu tới import cục bộ ở đầu file.
  Sau khi 2 method dùng `AccountsSnapshot` cục bộ (`getAccountsSnapshot`,
  `onAccountsChanged`) chuyển đi, import cục bộ trở thành thật sự không
  dùng — `tsc` báo `TS6196` (`noUnusedLocals`). Xoá khỏi import cục bộ,
  giữ nguyên khối `export type {...} from './orca-runtime-types'` ở cuối
  file (không đổi). Đây là biến thể mới của mẫu STAYS/MOVE — re-export
  trực tiếp từ module gốc KHÔNG tính là "dùng" import cục bộ cùng tên.
- `ClaudeRateLimitAccountsState`/`CodexRateLimitAccountsState` (từ
  `'../../shared/types'`) — move hẳn, xoá khỏi `orca-runtime.ts`.
- `orca-runtime.ts`: 10,467 → **10,421 dòng** (giảm 46 dòng thực — nhỏ vì
  phần lớn "giảm" bị bù lại bởi 8 dòng forwarding-field composition mới).
  File mới: 80 dòng — dưới ngưỡng 300, KHÔNG cần đăng ký
  `config/max-lines-baseline.txt`.
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi mới, sau khi sửa 1 lỗi tạm thời `TS6196` do import re-export). `oxlint`
  sạch (exit 0) cả 2 config. `max-lines-ratchet`: 647 vi phạm pre-existing
  không đổi.

## Rủi ro còn lại / khuyến nghị

- Move cơ học thuần tuý, rủi ro thấp (field cô lập, 0 host dep). Khuyến
  nghị kiểm thử thủ công luồng AccountsPane/rate-limit polling trên
  mobile trước khi merge.
- Còn 6 domain nhỏ khác từ TASK-BIGFILE-054 (057–062).

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **10,421 dòng (61.0% giảm)** qua 25 task
(TASK-BIGFILE-036 đến 056, không tính TASK-054 Investigate).
