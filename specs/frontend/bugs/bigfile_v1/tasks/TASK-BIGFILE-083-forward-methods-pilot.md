# TASK-BIGFILE-083 — Pilot: forwardMethods (giảm boilerplate composition-wiring)

**Loại:** Kỹ thuật MỚI hoàn toàn — không phải Move/Extract như 82 task trước,
mà thay đổi CÁCH KHAI BÁO forwarding field (giữ nguyên API bề ngoài, giữ
nguyên toàn bộ 39 composition-wiring block hiện có) · Rủi ro cao hơn mọi task
trước (an toàn không còn 100% qua `tsc` + diff nguyên văn — cần thêm test
runtime) · **Effort:** L (thiết kế + verify) cho 1 domain pilot
**Status:** ✅ Done (pilot), chờ quyết định người dùng về mở rộng
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau TASK-081/082, người dùng xác nhận muốn tiếp tục đến khi đạt mục tiêu
<2.000 dòng. Đánh giá lại cho thấy candidate kiểu Move/Extract đã cạn (ví dụ
`resolveWorktreeSelector` — tưởng là candidate nhưng hoá ra là core
dependency dùng khắp file, không tách được). Người dùng yêu cầu đổi phương
pháp; sau khi tôi làm rõ 2 lựa chọn (state-owner extraction — lợi ích thấp vì
closure wiring không đổi số dòng dù field ở đâu; vs. refactor chính phần
composition-wiring boilerplate — lợi ích cao hơn nhưng rủi ro cao hơn), người
dùng chọn refactor wiring boilerplate.

**Vấn đề an toàn cốt lõi**: 82 task trước LUÔN xác minh 100% qua `tsc` + diff
nguyên văn — không có gì thay đổi hành vi runtime mà `tsc` không bắt được.
Kỹ thuật `declare` field + hàm forward runtime PHÁ VỠ bất biến này: `declare
name: Type['name']` không phát sinh code, nên nếu hàm forward sai (bind sai
`this`, quên method, lộ private method, ...) thì `tsc` v·ẫn xanh nhưng runtime
sẽ lỗi ở lần gọi đầu tiên — không có safety net compile-time nào bắt được.

## Cách tiếp cận: pilot trước, đo thật, báo cáo trước khi mở rộng

1. Viết `forwardMethods<Source, K>(target, source, methods: readonly K[])`
   — helper bind từng method trong `methods` (kiểu `keyof Source`, `tsc` chặn
   tên sai) từ `source` sang `target`. KHÔNG dùng reflection tự động liệt kê
   toàn bộ method (rủi ro lộ private method — `private` của TS chỉ là
   compile-time, method vẫn enumerable ở runtime).
2. Viết `orca-runtime-forward-methods.test.ts` — 8 test đơn vị: bind đúng
   `this`, không lộ method ngoài allowlist, không lộ private method dù cố
   tình liệt kê (chứng minh an toàn nằm ở type system chứ không phải filter
   runtime), throw rõ ràng nếu tên không phải function, rebind độc lập nhiều
   instance, tương thích `vi.fn` spy, và **1 test mô phỏng chính xác hình
   dạng thật của `orca-runtime.ts`** (field composition + `declare` field +
   gọi `forwardMethods` trong constructor) — vì `orca-runtime.ts` không thể
   import trực tiếp trong môi trường test (kéo theo `node-pty` native binary
   không có sẵn trong sandbox — lý do file này chưa từng có test nào).
3. Pilot trên `issueTrackingCommands` — domain lớn nhất file (72 method
   forward, 146 dòng) — để đo lợi ích THẬT thay vì domain nhỏ (dễ cho kết quả
   sai lệch do overhead cố định của comment/method wrapper).

## Kết quả pilot (2026-08-12)

- Thay 72 forwarding field kiểu `name: Type['name'] = this.x.name.bind(this.x)`
  (146 dòng) bằng 72 `declare name: Type['name']` (72 dòng, không có `=`).
- Thêm method `private wireForwardedMethods(): void` (gọi
  `forwardMethods(this, this.issueTrackingCommands, [...72 tên...])`), gọi 1
  lần ở cuối constructor. An toàn về thứ tự: field initializer (bao gồm
  `issueTrackingCommands = new RuntimeIssueTrackingCommands(...)`) LUÔN chạy
  trước constructor body bất kể vị trí trong source — đã xác nhận bằng
  standalone Node.js test từ trước trong effort này, tái xác nhận qua test
  runtime mới ở bước 2.
- **Xác minh khớp 100% tên method + thứ tự** giữa bản gốc (146 dòng) và
  mảng string mới bằng script Python so khớp trực tiếp (không tin tưởng chép
  tay) — `MATCH: True`.
- `orca-runtime.ts`: 4,742 → **4,710 dòng — chỉ giảm 32 dòng thật** (thấp
  hơn ước tính ban đầu ~45-47 dòng, vì domain pilot ĐẦU TIÊN phải gánh chi
  phí cố định một lần: 9 dòng comment giải thích + khai báo method
  `wireForwardedMethods` — chi phí này KHÔNG lặp lại nếu mở rộng thêm domain
  khác vào CÙNG method).
- Xác minh: `tsc --noEmit --composite false` giữ đúng baseline 251 lỗi (0
  lỗi — `declare` field type-check sạch). `oxlint` sạch (exit 0) cả 2 config.
  `max-lines-ratchet`: 647 không đổi. 8/8 test `forwardMethods` pass. Xác
  nhận không có nơi nào trong `orca-runtime.ts` gọi 1 trong 72 method này
  TRƯỚC khi `wireForwardedMethods()` chạy (constructor cuối) — an toàn thứ tự.

## Đánh giá hiệu quả thật để quyết định mở rộng

- Domain pilot (72 method, LỚN NHẤT file) chỉ tiết kiệm 32 dòng ròng ở lần
  đầu. Nếu gộp TẤT CẢ 39 domain vào CÙNG `wireForwardedMethods()` (không lặp
  lại comment/method-wrapper), ước tính mỗi domain tiếp theo tiết kiệm theo
  tỷ lệ ~0.6 dòng/method (45 dòng tiết kiệm thật ÷ 72 method, sau khi trừ chi
  phí cố định đã trả ở pilot). Tổng 473 method còn lại (545 − 72) × 0.6 ≈
  **~280 dòng tiết kiệm thêm nếu mở rộng toàn bộ**.
- **Tổng tiềm năng nếu mở rộng hết**: 4,710 − 280 ≈ **~4.430 dòng** — VẪN
  CÒN CÁCH XA mốc 2.000 dòng hơn 2.400 dòng, dù đã dùng kỹ thuật rủi ro cao
  nhất được người dùng chấp thuận.
- Kết luận: kỹ thuật `forwardMethods` là cải thiện thật, an toàn vừa phải
  (có test bù cho phần `tsc` không bắt được), nhưng **không đủ sức đưa file
  xuống dưới 2.000 dòng** dù mở rộng toàn bộ 39 domain. Để đạt mục tiêu
  <2.000 dòng thật sự cần một thay đổi kiến trúc khác hẳn (vd bỏ hẳn forwarding
  phẳng, chuyển API bên ngoài sang truy cập qua `runtime.xCommands.method()`
  thay vì `runtime.method()` — nhưng đây là thay đổi PUBLIC API, ảnh hưởng
  33-154 caller bên ngoài `orca-runtime.ts` theo `codegraph` impact scan,
  vượt phạm vi "tách nhỏ 1 file").

## Tổng kết luỹ kế

`orca-runtime.ts`: 26,730 → **4,710 dòng (82.4% giảm)** qua 51 task
(51 = 50 Move/Extract + 1 pilot forwardMethods).
