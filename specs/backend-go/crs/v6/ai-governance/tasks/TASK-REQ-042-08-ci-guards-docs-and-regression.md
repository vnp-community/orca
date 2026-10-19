# TASK-REQ-042-08: Kiểm soát CI, tài liệu và hồi quy cuối

**From Solution:** [BE-REQ-SOL-042](../solutions/BE-REQ-SOL-042-prompt-catalog-shared-agent-relay-and-stdin.md)
**Priority:** P2
**Service:** `backend-go/ci`
**File:** `backend-go/ci/{check-prompt-golden.sh,check-agent-relay-single-source.sh}` (mới/sửa), `.github/workflows/` (sửa), `backend-go/prompts/README.md`, `specs/backend-go/crs/v6/ai-governance/` (cập nhật trạng thái), `deploy/agent/README.md`
**Depends on:** TASK-REQ-042-02, 03, 04, 06, 07
**Status:** [ ] TODO

---

## Context

- Sau khi gom, cần ngăn quay lại bản sao: một kiểm tra tìm chuỗi `"agent.execPrompt"`/`"ai.complete"` dùng làm method của `Relay`/`RelayByDevServer` ngoài `common/agentrelay`.
- Cần ngăn prompt mới nằm ngoài catalog.

## Việc cần làm

1. `check-agent-relay-single-source.sh`: thất bại nếu có `Method: "agent.execPrompt"` hoặc `"ai.complete"` ngoài `common/agentrelay` (cho phép trong test và `infra-fleet-service`).
2. Kiểm tra "prompt mới phải vào catalog": thất bại nếu có `strings.Builder` dựng chuỗi chứa mẫu `<feedback>`/`<request>` ngoài `backend-go/prompts` (heuristic, ghi rõ giới hạn).
3. Kiểm tra kích thước prompt: test đo prompt lớn nhất của mỗi luồng với fixture lớn và khẳng định dưới ngưỡng (ghi ngưỡng trong README).
4. Workflow CI chạy các kiểm tra và golden.
5. `backend-go/prompts/README.md`: cách thêm luồng/mẫu, cách sinh golden, quy tắc `fence`/`redact`, ID gói.
6. Hồi quy cuối: acceptance toàn bộ (`run_acceptance.py` bộ không dùng AI thật), UI stub, `--real native-execute`; so số liệu trước/sau.
7. Cập nhật trạng thái các CR và README thư mục.

## Kiểm thử

- Hai script chạy ở CI và thất bại đúng khi cố ý vi phạm (thêm một lời gọi `Method: "agent.execPrompt"` giả ở nhánh thử).
- Bộ hồi quy cuối xanh.

## Tiêu chí hoàn thành

- [ ] CI chặn quay lại bản sao relay và prompt ngoài catalog.
- [ ] README catalog và `deploy/agent/README.md` cập nhật.
- [ ] Hồi quy cuối xanh, số liệu trước/sau ghi trong PR.

## Rủi ro và lưu ý

- Heuristic "prompt ngoài catalog" có thể báo nhầm; cho phép danh sách loại trừ có lý do.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
