# TASK-TDD-002-T4-04: Bước 3: chạy luồng Orca cho BL-H01 từ Request đến completed

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 4 (bước 3), 5
**Checklist:** [T4.4](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** orca (stack local) + pilot
**Files:** CLI `orca`; `docs/crs/v5/CR-BL-H01_ai-foundation.md`
**Depends on:** T4.1, T4.3, toàn bộ Giai đoạn 2 và 3 (phần cần cho luồng)
**Status:** `[ ]` TODO

---

## Context

Chạy thật luồng đã thiết kế trên CR thật; chi phí đã được người dùng chấp nhận (một change nhỏ khoảng 2 phút; feature cỡ L lớn hơn nhiều). Chạy dưới `pilot_guard.py watch`.

## Việc cần làm

1. `orca request create --project <id> --title "BL-H01 nền AI" --body-file docs/crs/v5/CR-BL-H01_ai-foundation.md`; ghi `R`.
2. `orca request classify <R>`; `request wait --until awaiting_type_confirmation`; `request confirm-type <R> --type change_request --size L`; trả lời `clarification` nếu có; nhớ danh sách AC có id `AC-n` do Orca ghi từ CR.
3. `orca solution generate <R> --wait`: kiểm `design_references` (snapshot, id đã đọc, xung đột ADR-12); chọn phương án (`solution choose`), duyệt gate Solution.
4. `orca plan generate <R> --wait`: đọc `orca plan proposal <R>` và `orca plan bundle <R>` (proposal, design, spec, tasks); kiểm kết quả kiểm cấu trúc; duyệt gate Plan **sau khi đọc**.
5. Với từng Phase theo thứ tự: duyệt gate Phase (`approval list <R> --subject phase`), `plan start-phase <R> <phase>`, theo dõi `plan tree`; đọc kết quả task (khối `Knowledge`, `Verification`).
6. `request wait <R> --until completed --timeout 4h`; xem bàn giao (T3.4).
7. Sau mỗi giai đoạn lưu đầu ra vào `evidence/T4.4/` (JSON), kèm thời gian và chi phí.

## Tiêu chí hoàn thành

- [ ] Request `completed`; mọi Phase `done`; không can thiệp tay ngoài các gate.
- [ ] Solution có `design_references` và nêu xung đột ADR-12 (hoặc ghi lý do không).
- [ ] Gói OpenSpec đạt kiểm cấu trúc; mỗi task thực thi có khối kết quả với `Verification`.
- [ ] Log cho thấy agent gọi tool `mcp__tdd-agent__*` thật.

## Kiểm thử

- Kiểm bằng báo cáo pilot và `evidence/T4.4/`.

## Rủi ro và ghi chú

- Nếu một bước thất bại: dừng, ghi nguyên nhân (lỗi Orca, skill, KG), không vá bằng thao tác tay không ghi lại.
- Dừng chạy và huỷ nếu `pilot_guard.py watch` báo thư mục chính đổi.
