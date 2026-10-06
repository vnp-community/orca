# Tasks backend: request-artifact-model (v6)

> 📋 Proposed. Mỗi task 0,5 đến 2 ngày. Đường dẫn tương đối tới `backend-go/services/request-service/` (mới) trừ khi ghi khác; task 027-02 và các phần ghi rõ ở `task-service`. Tất cả `Status: [ ] TODO`, chưa chạy test nào.

## Bảng Solution, Task

| Solution | Task | Nội dung | Ưu tiên |
|----------|------|----------|---------|
| [BE-REQ-SOL-027](../solutions/BE-REQ-SOL-027-artifact-schema-ontology-and-task-specs.md) | [TASK-REQ-027-01](./TASK-REQ-027-01-request-artifact-migration.md) | Migration `NNNN_request_artifact_model` (cột Request, Solution, 4 bảng mới), domain `RequestRevision` | P0 |
| | [TASK-REQ-027-02](./TASK-REQ-027-02-task-service-task-specs.md) | `task-service`: `task_specs`, `SetTaskSpec`, `GetTaskSpecs`, `LockTaskSpecs`, `TASK_SPEC_LOCKED` | P0 |
| | [TASK-REQ-027-03](./TASK-REQ-027-03-schema-registry-canonical-json-provenance.md) | Registry JSON Schema (spike thư viện), `CanonicalJSON`, digest, `Provenance` | P0 |
| | [TASK-REQ-027-04](./TASK-REQ-027-04-markdown-yaml-projection.md) | Bản chiếu Markdown/YAML, `RenderArtifact`, `ParseProjection`, export | P1 |
| | [TASK-REQ-027-05](./TASK-REQ-027-05-request-content-validation-and-revisions.md) | AC, trường theo loại, `ValidateRequestContent`, `AppendRequestRevision`, chốt chặn ghi | P0 |
| | [TASK-REQ-027-06](./TASK-REQ-027-06-artifact-ids-relations-and-semantic-validation.md) | ID hiển thị, `artifact_index`, quan hệ, bảng phủ, 10 mã ngữ nghĩa | P0 |
| | [TASK-REQ-027-07](./TASK-REQ-027-07-wire-solution-plan-phase-and-approval.md) | Nối SOL-007, 012, 013, 009: `options`, provenance, bảng phủ, khoá spec | P0 |
| | [TASK-REQ-027-08](./TASK-REQ-027-08-artifact-proto-grpc-and-integration.md) | `artifact.proto`, gRPC, quyền đọc, tích hợp hai dialect | P0 |
| [BE-REQ-SOL-028](../solutions/BE-REQ-SOL-028-clarification-decision-and-awaiting-information.md) | [TASK-REQ-028-01](./TASK-REQ-028-01-clarifications-decisions-migration.md) | Migration 12 trạng thái, 5 bảng, chỉ mục duy nhất hai dialect | P0 |
| | [TASK-REQ-028-02](./TASK-REQ-028-02-domain-and-state-machine-awaiting-information.md) | Domain Clarification, Decision, Readiness; máy trạng thái 12 nhân 18 | P0 |
| | [TASK-REQ-028-03](./TASK-REQ-028-03-clarification-and-decision-repositories.md) | Repository hai dialect, `ClaimExpired`, `ListPendingForUser`, `ResumeStatus` | P0 |
| | [TASK-REQ-028-04](./TASK-REQ-028-04-request-clarification-readiness-and-waive.md) | `RequestClarification`, sẵn sàng trong `ConfirmRequestType`, `WaiveReadiness` | P0 |
| | [TASK-REQ-028-05](./TASK-REQ-028-05-answer-cancel-clarification-and-hooks.md) | `AnswerClarification`, huỷ, hook đổi loại, huỷ Request, backlog | P0 |
| | [TASK-REQ-028-06](./TASK-REQ-028-06-resume-consumer-expiry-reminder-and-notifications.md) | Consumer kích hoạt lại, hết hạn, nhắc, `notification-service` | P0 |
| | [TASK-REQ-028-07](./TASK-REQ-028-07-decision-record-confirm-and-approval-gates.md) | `RecordDecision`, `ConfirmDecision`, chặn duyệt | P0 |
| | [TASK-REQ-028-08](./TASK-REQ-028-08-clarification-decision-proto-grpc-and-integration.md) | Proto, gRPC, `ListPendingClarificationsForUser`, tích hợp đầu cuối | P0 |

## Thứ tự phụ thuộc

```
TASK-REQ-002-01 ─▶ 027-01 ─────────────┬─▶ 027-05 ─┬─▶ 027-06 ─┐
TASK-REQ-001-01 ─▶ 027-03 ─┬─▶ 027-04 ─┘           │           │
                           └─────────────▶ 027-05  │           ▼
TASK-REQ-011-01/-02 ─▶ 027-02 ───────────────────────────────▶ 027-07 ─▶ 027-08
                                      (cần 007-05/-06, 012-05/-06, 013-05, 009-04)

027-01 ─▶ 028-01 ─▶ 028-03 ─▶ 028-04 ─▶ 028-05 ─▶ 028-06 ─┐
027-05, 003-02 ─▶ 028-02 ─▶ 028-03                          ├─▶ 028-08
                            028-02, 028-03, 007-06 ─▶ 028-07 ┘
(028-06 chờ TASK-REQ-010-05; 028-04 cần 005-05; 028-05 cần 005-06, 006-xx)
```

Làm song song được: 027-02 với 027-01 và 027-03; 027-04 với 027-05; 028-01 với 028-02; 028-06 với 028-07 (sau 028-05).

## Ghi chú

- **Số migration:** mọi task migration ghi `NNNN` (request-service) hoặc `00NN` (task-service) và bắt buộc `ls migrations/postgres migrations/mysql` lúc làm; không tin số trong tài liệu (CR-REQ-001/002/004/006 chồng số). 027-01 phải đi trước 028-01.
- **Thứ tự với SOL-007 và SOL-012:** làm 027-03 đến 027-05 trước TASK-REQ-007-02 và TASK-REQ-012-03; 027-07 sửa mã của hai solution đó sau khi chúng có mã.
- **Chốt chặn ghi:** hai test `go/parser` (trạng thái ở SOL-003, nội dung ở 027-05) chạy trong CI; thêm trường mới ghi ngoài use case chỉ định sẽ làm test đỏ.
- **`notification-service`:** 028-06 sửa cùng hai tệp với TASK-REQ-010-05; làm sau hoặc cùng PR.
- **Không kiểm chứng được ở đây:** thư viện JSON Schema (spike 027-03); hành vi task quay lại sau `needs_info` (CR-REQ-013/029); mọi test chưa chạy.
- **Sau feature này:** CR-REQ-029 (hợp đồng thực thi, `TaskSpec` v2) dùng `task_specs` và `needs_info`; BE-REQ-SOL-026 dùng `RenderArtifact` (027-04).
