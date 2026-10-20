# Giai đoạn 9: Hoàn thiện

Mục tiêu: chứng minh v2 tốt hơn mốc v1 trên luồng thật, đo hiệu năng, rà bảo mật, cập nhật tài liệu.

---

## T9.1 Kiểm thử đầu-cuối với workflow

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T7.6

Checklist:
- [x] Workflow minipay chế độ `claude` chạy tới `gate_testspec` với v2; lưu đầu ra vào `output-docs/CR-001-v2/`
- [x] So với `output-docs/CR-001-llm/` (v1) và BASELINE-V1.md: ghi `implementation/COMPARE-E2E.md`
- [x] Các tình huống workflow đã chạy được với v1 (từ chối ở `gate_testspec`, supplement, baseline qua MCP) chạy được với v2

Tiêu chí xong: COMPARE-E2E.md cho thấy v2 không kém v1 ở mọi chỉ số và phủ các khoảng trống đã biết.

---

## T9.2 Hiệu năng và vận hành

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T7.5

Checklist:
- [x] Dữ liệu giả 50 feature × 1.000 TC; đo bước luật (NFR-005), p95 truy vấn MCP (NFR-006): TST-20
- [x] Đo RTO snapshot (NFR-009): TST-16
- [x] Đo kích thước DB và WAL sau 10 lần ghi lại Draft; chỉnh `wal_autocheckpoint` nếu cần
- [x] Ghi `implementation/PERF.md`; thay các giá trị "đề xuất" của TDD v2 bằng số đo

Tiêu chí xong: PERF.md có số đo; NFR không đạt có task theo dõi.

---

## T9.3 Rà bảo mật

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T7.3

Checklist:
- [x] Bộ mẫu tiêm chỉ dẫn trong tài liệu nguồn (md, `props` của KG): LLM không làm theo, đầu ra không lọt mã ngoài danh sách trắng
- [x] Quét bí mật trên KG, md, log sau một run đầy đủ
- [x] MCP: không token bị từ chối ngoài dev; Origin lạ bị từ chối; `sql_select` không đọc được bảng thật; đường dẫn ngoài `SOURCE_ROOTS` bị từ chối; mã feature path traversal bị từ chối
- [x] `claude-cli`: không tool, không MCP, thư mục trống
- [x] Ghi kết quả vào `implementation/SECURITY.md`

Tiêu chí xong: SECURITY.md không còn vấn đề mức cao chưa xử lý.

---

## T9.4 Tài liệu

- **Trạng thái:** Xong (2026-10-12)
- **Phụ thuộc:** T9.1

Checklist:
- [x] README của agent: kiến trúc, cài đặt, lệnh, biến môi trường, trạng thái FR, danh sách test
- [x] TDD-TSAGENT-v2.md: trạng thái `baselined`; đưa các dòng DEVIATIONS.md còn hiệu lực vào; số đo từ PERF.md
- [x] TDD-TSAGENT.md (v1), KG-TSAGENT.md, TEST-ONTOLOGY-TSAGENT.md: đánh dấu `superseded`, trỏ sang v2, ghi vị trí lưu trữ v1 (ARCHIVE-V1.md)
- [x] README thư mục `docs/crs/v17/testspec-agent/` cập nhật
- [x] README của minipay-demo: luồng testspec dùng JSON
- [x] Bảng tiến độ trong `implementation/README.md` đầy đủ

Tiêu chí xong: không còn tài liệu nào mô tả v1 như hiện trạng.
