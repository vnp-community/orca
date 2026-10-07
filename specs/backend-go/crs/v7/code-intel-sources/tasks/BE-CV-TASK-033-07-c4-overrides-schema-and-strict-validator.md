# BE-CV-TASK-033-07: Schema `c4.yaml` v1 và bộ xác thực nghiêm (không alias, giới hạn, trường lạ)

**From Solution:** BE-CV-SOL-033-c4-overrides-yaml
**Priority:** P0
**Service:** `code-intel-service`
**File:** `backend-go/services/code-intel-service/internal/domain/c4/overrides_schema.go` (mới), `.../overrides_validator.go` (mới) và `_test.go`
**Depends on:** BE-CV-TASK-033-04 (id, kind)
**Status:** [x] DONE

---

## Context

Solution mục 2.A, 2.B. `yaml.v3` chỉ xác nhận trong `infra-fleet-service`/`api-gateway`; thêm dependency vào `code-intel-service/go.mod` (đã dùng trong repo, không phải thư viện mới).

## Việc cần làm

1. Kiểu Go cho schema; giải mã vào `yaml.Node` bằng `Decoder.KnownFields(true)`.
2. Duyệt node: từ chối `AliasNode`, anchor, tag tuỳ chỉnh, key trùng.
3. Giới hạn ≤ 64 KiB, 200 component, 40 external, 400 quan hệ, mô tả ≤ 500, tên ≤ 120, id regex.
4. `version==1`, `container` khớp; cảnh báo (không chặn) mẫu `password|token|secret|dsn=`.
5. Lỗi → `CODEINTEL_INVALID_PARAMS` (`field`) / `PAYLOAD_TOO_LARGE`.

## Kiểm thử

`go test ./services/code-intel-service/internal/domain/c4/... -run Overrides` (chưa chạy): mẫu CR §2.5 hợp lệ; alias, `version:2`, trường lạ, 65 KiB, 201 component, key trùng đều bị từ chối; fuzz ngắn.

## Tiêu chí hoàn thành

- [x] Mọi ca độc hại bị từ chối; mẫu hợp lệ qua.
- [x] Không alias nào lọt.

## Rủi ro và lưu ý

- `KnownFields` chỉ áp trên struct; việc duyệt Node cần bù cho key trùng.
