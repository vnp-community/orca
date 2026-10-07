# BE-CV-TASK-033-01: Re-verify cấu trúc hexagonal, bằng chứng `implements` và fixture

**From Solution:** BE-CV-SOL-033-c4-component-view
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/adapter/gopackagegraph/testdata/INVENTORY.md` (mới), `.../testdata/mini-service/` (mới)
**Depends on:** BE-CV-TASK-030-05
**Status:** [x] DONE

---

## Context

Solution mục 1 C1: 23 dòng `_ usecase.` (không phải 29) ở `mysql/shared.go`, 3 ở postgres, 41 interface ở `ports.go`.

## Việc cần làm

1. Chạy chỉ-đọc cho `infra-fleet-service`, `api-gateway`, `git-gateway-service`, `usage-service`: `ls internal/adapter`, đếm file Go không-test mỗi thư mục, `grep -c '^\s*_ usecase\.'`, `grep -c '^type .* interface'`; ghi `INVENTORY.md`.
2. Kiểm import thật của từng adapter (`pgx`, `go-sql-driver/mysql`, `nats-io/nats.go`, `common/eventbus`, `x/crypto/ssh`, `coder/websocket`, `prometheus`, `common/secrets`/vault): xác định Vault có được import ở adapter không (CR: chưa kiểm).
3. Đọc package doc (`// Package …`) của 15 adapter `infra-fleet-service`, đánh giá chất lượng (đúng vai trò/chi tiết) → ghi vào INVENTORY.
4. Dựng `mini-service/` (cây Go rút gọn hexagonal có một vi phạm lớp, một `var _`, một cổng chỉ khớp theo tên) làm fixture.

## Kiểm thử

Không test mã; review. `find` kiểm kích thước fixture.

## Tiêu chí hoàn thành

- [x] INVENTORY có số thật và kết luận về Vault.
- [x] Fixture đủ các ca ở solution mục 6.

## Rủi ro và lưu ý

- Fixture là bản sao; golden chính dùng repo thật.
