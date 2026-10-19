# TASK-REQ-039-06: Tích hợp, hợp đồng và e2e với repo fixture tài chính

**From Solution:** [BE-REQ-SOL-039](../solutions/BE-REQ-SOL-039-request-worktree-base-branch.md)
**Priority:** P1
**Service:** `tests/e2e, request-service`
**File:** `tests/stages/e2e/orca-flows/suite_request_base_ref.py` (mới, đăng ký trong `run_acceptance.py`), `tests/stages/e2e/fixtures/ledgerlite-finance/` (repo fixture, xem kế hoạch tổng mục 4), `backend-go/services/request-service/internal/adapter/contracttest/execution_flow_contract.go` (sửa)
**Depends on:** TASK-REQ-039-03, 039-04, 039-05
**Status:** [ ] TODO

---

## Context

- Bộ acceptance chạy qua WebSocket gateway trên stack local; dev server là agent thật (`start-local-agent.sh`).
- Fixture `ledgerlite-finance` (đã dựng, chưa commit; `README.md` của nó là nguồn sự thật) là repo tài chính nhỏ có URD/SRS/SAD, `.tdd-agent/config.toml` và ba kịch bản trong `docs/e2e-scenarios.md`; e2e sao chép vào thư mục git tạm, chạy `tdd-agent generate` + `mcp enable --read-only`, rồi một commit. Tri thức đã sinh không được commit.
- Quy tắc: `run_ui_flow.py --real` không cho sửa checkout orca trong lúc chạy.

## Việc cần làm

1. Suite `suite_request_base_ref.py` (id `BRF-xx`): (a) tạo Request không `baseRef` → `baseRef==""`; (b) tạo với `release/x` → lưu đúng; (c) `INVALID`/`NOT_FOUND`; (d) `setBaseRef` khi chưa khoá thành công, phiên bản tăng; (e) sau khi tạo worktree đề xuất hoặc task đầu thì `LOCKED`; (f) **kiểm nhánh gốc thật**: dùng claude giả (`--ai-stub`) để chạy một Request `task` native trên fixture, đọc `git rev-parse` của worktree trong dev server và so với SHA của nhánh đã chọn; (g) checkout chính của fixture ở nhánh thứ ba khi không chỉ định: worktree rẽ từ nhánh mặc định.
2. Kịch bản OpenSpec (change_request) với `base_ref`: worktree đề xuất rẽ từ ref; chạy với claude giả nếu stub hỗ trợ OpenSpec, nếu không thì ghi rõ ca nào chỉ chạy được với Claude thật và để ở chế độ opt-in `ORCA_E2E_OPENSPEC=1` (tốn tiền, người dùng đồng ý trước).
3. Spec UI (`ui-live/request-base-branch.web.e2e.ts`) được làm ở FE-REQ-TASK-040-05; task này cung cấp fixture và helper tạo repo tạm (`fixture-repo-ledgerlite.ts`).
4. `execution_flow_contract.go`: kịch bản Request có `base_ref` đi hết một Plan với fake task service, kiểm `BaseRef` nhận được ở cả hai đường dispatch.
5. Đăng ký suite `request_base_ref` trong `run_acceptance.py`; thêm vào bộ chạy CI nếu không dùng AI thật.

## Kiểm thử

- `python3 tests/stages/e2e/orca-flows/run_acceptance.py devserver request_base_ref` xanh; `... qa` không hỏng.
- `go test ./services/request-service/internal/adapter/contracttest/... -run 'Execution' -v` xanh trên hai dialect.
- Chạy lại `qa-bug-task-native-flow` (stub) để chắc không hồi quy.

## Tiêu chí hoàn thành

- [ ] Suite `BRF-xx` xanh trên stack local với repo fixture thật.
- [ ] Ca nhánh gốc kiểm bằng `git rev-parse` thật, không mock.
- [ ] Không hồi quy `qa`, `request`, `matrix`.

## Rủi ro và lưu ý

- Ca OpenSpec thật tốn 7 đến 13 USD và cần `.tdd-agent/config.toml` trong fixture; không tự chạy khi chưa được đồng ý.
- Stack local mất Vault khi khởi động lại; chuẩn bị lại bằng `vault-init` trước khi chạy.

## Chuẩn hoàn thành chung (áp dụng cho mọi task, không thương lượng)

- Mã chạy thật, **không stub, không TODO, không `panic` cho lỗi thường, không bỏ qua lỗi**; mọi nhánh lỗi có mã lỗi và test.
- Chạy `gitnexus_impact` trước khi sửa symbol có sẵn và báo blast radius; chạy `detect_changes` trước khi commit (CLAUDE.md).
- Hai dialect: mọi thay đổi lưu trữ có migration và test hợp đồng trên **Postgres 16 và MySQL 8** (testcontainers).
- Không file nào vượt `max-lines` (tách file khi cần, không thêm disable); không đặt tên `helpers`, `utils`, `common`, `misc`.
- Chạy được trên macOS, Linux, Windows và qua SSH; lệnh git theo baseline 2.25 (`guides/reference/git-compatibility.md`).
- Không ghi nội dung Request, Solution, phản hồi hoặc bí mật vào log, span hay khung sự kiện (C9).
- `go build ./...` và `go test` của gói đã đổi xanh; `go vet` xanh; lint xanh; không làm hỏng test hiện có.
