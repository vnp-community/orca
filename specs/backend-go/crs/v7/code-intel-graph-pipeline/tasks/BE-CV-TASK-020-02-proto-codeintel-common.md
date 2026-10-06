# BE-CV-TASK-020-02: Proto `codeintel_common.proto` (kiểu và enum dùng chung)

**From Solution:** BE-CV-SOL-020-canonical-graph-model
**Priority:** P0
**Service:** `proto`
**File:** `backend-go/proto/orca/codeintel/v1/codeintel_common.proto` (mới); stub sinh `backend-go/proto/gen/go/orca/codeintel/v1/codeintel_common.pb.go` (mới, sinh)
**Depends on:** TASK-020-01
**Status:** [ ] TODO

---

## Context

Hợp đồng §2.1 hàng 2 và §2.3 liệt kê nội dung và số field. Solution mục 2.B là bảng đầy đủ. File này được **mọi** proto codeintel khác import (hợp đồng §2.1 "Quy tắc phụ thuộc import"), nên số field khoá lâu dài; `buf breaking FILE` cấm đổi sau khi merge.

## Việc cần làm

1. Tạo file với `syntax = "proto3"`, `package orca.codeintel.v1;`, `option go_package = "github.com/stablyai/orca-go/proto/gen/go/orca/codeintel/v1;codeintelv1";`, import `google/protobuf/timestamp.proto`.
2. Khai báo enum (mỗi enum có `*_UNSPECIFIED = 0`, tiền tố đầy đủ theo `buf lint STANDARD`): `SymbolKind` (13), `EdgeKind` (14), `ToolName` (`GITNEXUS=1, CODEGRAPH=2`), `IndexState`, `IndexScope`, `Freshness`, `Risk`, `ViewKind` (mục 2.B của solution, 21 giá trị).
3. Khai báo message: `WorktreeSelector`, `SymbolRef`, `SourceInfo` (có `line_base=5`), `SourceRef`, `ServiceRef`, `ClusterRef`, `ResultMeta` (nguyên văn hợp đồng §2.3), `ToolIndexStatus`, `IndexStats`, `PendingChanges`. `ResultMeta.dev_server_id=3` giữ trong proto (gateway **không** chuyển ra WS, hợp đồng §2.3 ghi chú).
4. **Không** khai báo `IndexStatus` tổng hợp, `RepoBinding`, `IndexBasis` (CR-CV-012/080), `ChangeOverlay` (CR-CV-036), bất kỳ `service`.
5. Chạy `cd backend-go/proto && buf generate`; commit stub theo cách repo đang làm (kiểm `git ls-files backend-go/proto/gen/go/orca/infrafleet/v1` để biết stub có được commit không, rồi làm tương tự).
6. Ghi chú đầu file (≤ 3 dòng): file này do CR-CV-020 sở hữu, chỉ thêm, `ViewKind` 11+ phủ tập `graph_snapshots.view`.

## Kiểm thử

- `cd backend-go/proto && buf lint` (xanh).
- `buf breaking --against '.git#branch=main,subdir=backend-go/proto'` (xanh vì là file mới).
- `cd backend-go/proto && go build ./...` (stub biên dịch).
- Test reflection nhỏ (`backend-go/services/code-intel-service/internal/adapter/grpc/proto_surface_test.go`, mới): mọi enum trong package có giá trị 0 tên `*_UNSPECIFIED`; không message nào có field tên chứa `secret|token|password`.
- Không dùng `make proto-lint` làm bằng chứng (Makefile kết thúc bằng `|| true`, hợp đồng §2 đầu).

## Tiêu chí hoàn thành

- [ ] `buf lint` và `buf breaking` xanh khi gọi trực tiếp.
- [ ] Mọi số field khớp bảng solution 2.B; `ResultMeta` khớp hợp đồng §2.3 từng dòng.
- [ ] Không có `IndexStatus`/`ChangeOverlay`/service trong file.
- [ ] Stub sinh được và build.

## Rủi ro và lưu ý

- Số `ToolIndexStatus` 11–18 và `ViewKind` 11–21 là đề xuất (solution 6); nếu CR-CV-012 cần field khác trên `ToolIndexStatus` thì thoả thuận **trước** khi merge, sau đó không đổi được.
- Tên enum value sai tiền tố sẽ làm `buf lint` đỏ: chạy lint cục bộ trước khi đẩy.
