# TASK-E2E-029: Đưa `orca-flows` vào CI (smoke hằng đêm, `--strict` định kỳ)

**From Solution:** SOL-E2E-006 · **Resolves:** toàn bộ
**Priority:** P2 · **Service/Area:** ci
**Files:**
- .github/workflows/orca-flows-e2e.yml (mới: hằng đêm, hằng tuần, `workflow_dispatch`)
- .github/workflows/backend-go-request-contracts.yml (mới, chặn PR: guard tĩnh, hợp đồng gateway, `claude` giả)
- tests/stages/e2e/orca-flows/ci/run-orca-flows-ci.sh (mới: dựng stack, chạy suite, thu log; đặt ở `tests/` thay vì `backend-go/ci/` để không đụng backend-go)
- tests/stages/e2e/orca-flows/run_acceptance.py (`--ai-stub`, `--keep-stub-agent`, `--allow-blocked`)
**Depends on:** TASK-E2E-026
**Status:** `[~]` MỘT PHẦN: workflow đã viết và kiểm cú pháp, **chưa chạy trên runner thật** (cập nhật 2026-10-09, chưa commit)

---

## Context

Bộ e2e xuyên tầng chỉ chạy tay nên lỗi RC-A chỉ lộ khi có người chạy.

## Việc cần làm

- [~] Job dựng stack bằng compose (không Vault ngoài), dev server là agent chạy ngay trong job với repo tạm. — _`run-orca-flows-ci.sh`: sinh `.env` (mật khẩu ngẫu nhiên), `docker-compose.yml` + `docker-compose.local.yml` (Vault dev-mode), build, migrate, đợi gateway và admin, trỏ infra-fleet tới tenant của admin, build agent, chạy runner (runner tự khởi động agent trên runner và dùng repo tạm). Chưa chạy trọn trên runner_
- [~] Hằng đêm: `platform workflow request matrix exits` với `--ai-stub`; hằng tuần: `--strict`. — _đã viết: đêm Thứ Hai tới Thứ Bảy `platform workflow request matrix exits ai plan` (thêm `ai plan` vì stub làm chúng rẻ, đây là chỗ dùng stub), Chủ nhật `--strict --allow-blocked MCP-01,MCP-02`; `workflow_dispatch` chọn `nightly|weekly|custom`_
- [~] Lưu `reports/last-run.{json,md}` làm artifact; thông báo khi đỏ. — _artifact (kèm log stack, log agent, `calls.jsonl` của stub khi đỏ) và tóm tắt vào `$GITHUB_STEP_SUMMARY`; khi bản chạy theo lịch đỏ thì mở (hoặc bình luận vào) một issue; phần issue chưa thử_
- [x] Chạy `make check-agent-methods-called`, `make check-grpc-dials` trong CI, cùng các test Go parity/golden chưa có workflow nào chạy. — _`backend-go-request-contracts.yml`: không workflow nào chạy test parity/golden của `api-gateway` (không có `backend-go-api-gateway.yml`; `backend-go-mcp-conformance.yml` chỉ chạy `wscompat -run Mcp`) hay test của `common/grpcmw`, nên workflow này chạy `TestRequestRPCChannelParity` + `TestChannelInventoryMatchesGolden`, `grpcmw`, `make check-agent-capability-golden` và kiểm chính `claude` giả; chạy từng bước tay ở máy local, xem dưới_

## Tiêu chí hoàn thành

- [ ] Job xanh 5 đêm liên tiếp trước khi thành cổng chặn. — _chưa làm: chưa chạy lần nào trên runner_

## Kiểm thử

Chạy thử bằng `workflow_dispatch`.

## Kết quả thực thi

**Một phần.** Mã và workflow đã có, kiểm tĩnh và mô phỏng cục bộ; **chưa chạy trên GitHub Actions** và tôi không chạy được nó ở đây.

Đã kiểm thật:

| Lệnh | Kết quả |
|---|---|
| `actionlint` v1.7.7 trên hai workflow | không lỗi (shellcheck không có trên máy nên các khối `run:` chưa qua shellcheck) |
| `python3 -c "yaml.safe_load(...)"` | hợp lệ |
| `make check-agent-methods-called check-grpc-dials check-agent-capability-golden` (trong `backend-go`) | xanh (24 khoảng trống đã biết của agent) |
| `go test ./grpcmw/...` (common) | xanh |
| `go test ./internal/adapter/wscompat/ -run 'TestRequestRPCChannelParity\|TestChannelInventoryMatchesGolden'` | xanh |
| `bash run-orca-flows-ci.sh --prepare-env-only <đường dẫn>` | sinh `.env` đủ khoá, không trùng khoá, đọc được bằng `. file`; token/mật khẩu không in ra |
| `bash run-orca-flows-ci.sh nightly` ngoài CI | từ chối (mã 2) như thiết kế: script kết thúc bằng `docker compose down -v` |
| `run_acceptance.py --ai-stub ...` trên stack local | 109 PASS, xem TASK-035 |

Chưa kiểm, có thể hỏng trên runner (nói thẳng):

- `build-local.sh` ở runner (`pnpm install` cả monorepo rồi `vite build`) và thời gian của nó; `timeout-minutes: 90` là ước lượng.
- Trình tự dựng stack từ volume rỗng: `up -d --wait postgres nats vault`, `docker wait orca-go-vault-init`, `migrate.sh`, đợi `/healthz` và đăng nhập admin; bước đọc id tenant từ `tenant.companies` (đã thử câu SQL trên stack local) rồi `up -d infra-fleet-service` và restart nhóm service Go.
- Phiên bản action (`checkout@v4`, `setup-go@v5`, `setup-node@v4`, `pnpm/action-setup@v4`, `setup-python@v5`, `upload-artifact@v4`, `github-script@v7`): lấy theo các workflow khác trong repo, không kiểm được.
- Suite `workflow` đọc kết quả step bằng `docker exec orca-go-postgres psql` (tên container cố định trong compose).
- MCP: stack CI để `MCP_ENABLED=false` (cần chính sách Vault và proxy TLS mà Vault tạm không có), nên `MCP-01/02` BLOCKED và được `--allow-blocked`.
- Test `AST-05` đang FAIL (lỗi sản phẩm, xem TASK-035), nên bản chạy hằng đêm sẽ đỏ cho tới khi lỗi đó được sửa. Đó là việc đúng của nó, nhưng cần biết trước khi bật.
