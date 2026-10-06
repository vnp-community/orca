# BE-CV-TASK-010-01: Re-verify nền: `buf lint` với `service` rỗng, stream JetStream, danh sách wiring

**From Solution:** BE-CV-SOL-010-scaffold-code-intel-service
**Priority:** P0
**Service:** `code-intel-service` (chưa tạo) · `proto`
**File:** không sửa file sản phẩm; kết quả ghi vào mô tả PR (bảng "đã kiểm / lệch")
**Depends on:** không
**Status:** [ ] TODO

---

## Context

CR-CV-010 mục 6 ghi hai điểm chưa kiểm chứng chặn thiết kế: (1) `buf lint` STANDARD có chấp nhận `service CodeIntelService {}` không RPC hay không; (2) tên stream JetStream `CODEINTEL` có trùng stream đang chạy ở môi trường thật không. Task này kiểm trước khi các task 02–08 cam kết hướng đi. Hợp đồng `CONTRACT-codeintel-proto-and-data-map.md` §2.1 #1 yêu cầu `codeintel.proto` chỉ chứa `service`.

## Việc cần làm

1. Xác nhận hiện trạng: `ls backend-go/proto/orca` (16 thư mục, không `codeintel`); `ls backend-go/services` (19 service); `git ls-files backend-go/proto/gen/go/orca | head` (stub được commit).
2. Trong thư mục tạm **ngoài repo** (scratchpad), sao `backend-go/proto/buf.yaml` và tạo `orca/codeintel/v1/codeintel.proto` chỉ có `package`, `option go_package`, `service CodeIntelService {}`; chạy `buf lint`. Ghi kết quả:
   - xanh → giữ kế hoạch service rỗng;
   - đỏ (ví dụ `SERVICE_SUFFIX`/rule khác) → chọn một: (a) thêm RPC `GetIndexStatus` sớm (cần message của SOL-012, đẩy SOL-012 task 02 lên trước), (b) không đăng ký service ở SOL-010 và để SOL-012 thêm cả file lẫn đăng ký. Ghi quyết định vào SOL-010 mục 7 Q1.
3. Liệt kê stream JetStream ở môi trường dev thật nếu truy cập được (`nats stream ls` chỉ đọc); nếu không truy cập được, ghi "chưa kiểm chứng" và chỉ dựa vào `EnsureStream` của các service (`grep -rn "EnsureStream" backend-go/services`): đã có `TASK`, `MCP`, `PROJECT`, `INFRAFLEET`, trace stream. Xác nhận không có `CODEINTEL` hoặc subject chồng `orca.codeintel.>`.
4. Đọc lại các file wiring (`go.work`, `Makefile` dòng 7–11, hai `init-databases.sh`, `migrate.sh` dòng 27, `build-local.sh` mảng `ALL_SERVICES`, `check-opa-bundle-in-images.sh` mảng `svcs`) và xác nhận số dòng trong SOL-010 mục 2.I còn đúng ở HEAD hiện tại.
5. Kiểm `deploy/dev/docker-compose.yml`: `x-go-common-env` đã có `PROJECT_SERVICE_ADDR`, `INFRA_FLEET_SERVICE_ADDR`, `GIT_GATEWAY_SERVICE_ADDR`, `AUTH_SERVICE_ADDR`, `NATS_URL`.

## Kiểm thử

- Lệnh chỉ đọc: `buf lint` (thư mục tạm), `ls`, `grep -rn "EnsureStream" backend-go/services`, `git ls-files`. Không build, không sinh code vào repo.

## Tiêu chí hoàn thành

- [ ] Có kết luận "xanh/đỏ" của `buf lint` cho service rỗng và quyết định tương ứng ghi vào PR.
- [ ] Có kết luận về tên stream `CODEINTEL` (đã kiểm hoặc "chưa kiểm chứng").
- [ ] Số dòng wiring trong SOL-010 còn khớp, hoặc ghi chỗ lệch.

## Rủi ro và lưu ý

- Nếu `buf` không cài trên máy làm việc, task này chặn; cài bằng cách cấp phép của đội, không dùng `make proto-lint` (có `|| true`).
