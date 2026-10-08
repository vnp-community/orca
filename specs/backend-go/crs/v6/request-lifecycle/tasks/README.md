# backend-go Tasks: Request Lifecycle (v6)

Task thực thi của bốn solution trong [`../solutions/`](../solutions/README.md). Trạng thái từng task ở file task và bảng cuối trang. Mỗi task nêu file, tên test và lệnh; đã đối chiếu code thật ngày 2026-10-06.

## Bảng Solution, Task

| Solution | Task | Nội dung | Priority |
|----------|------|----------|----------|
| BE-REQ-SOL-003 | [TASK-REQ-003-01](./TASK-REQ-003-01-flow-registry.md) | Registry `FlowDefinition`, `FlowFor`, `PhasesFor` | P0 |
| | [TASK-REQ-003-02](./TASK-REQ-003-02-triggers-and-transition-table.md) | 16 trigger, `NextStatus`, `HappyPath` | P0 |
| | [TASK-REQ-003-03](./TASK-REQ-003-03-transition-request-usecase.md) | `TransitionRequest`, sự kiện, `InTransaction` | P0 |
| | [TASK-REQ-003-04](./TASK-REQ-003-04-transition-integration-tests-two-dialects.md) | Test tích hợp hai dialect | P0 |
| | [TASK-REQ-003-05](./TASK-REQ-003-05-get-request-flow-rpc.md) | RPC `GetRequestFlow` | P1 |
| | [TASK-REQ-003-06](./TASK-REQ-003-06-status-write-guard-and-readme-contract-tests.md) | Test `go/parser` và test hợp đồng README | P1 |
| BE-REQ-SOL-004 | [TASK-REQ-004-01](./TASK-REQ-004-01-migration-0003-source-hints.md) | Migration `source_hints`, domain, repository **[DONE]** | P0 |
| | [TASK-REQ-004-02](./TASK-REQ-004-02-source-normalization-and-idempotency-key.md) | Chuẩn hoá nguồn, khoá idempotency **[DONE]** | P0 |
| | [TASK-REQ-004-03](./TASK-REQ-004-03-proto-create-request-and-source-filters.md) | Proto `CreateRequest`, bộ lọc nguồn **[DONE]** | P0 |
| | [TASK-REQ-004-04](./TASK-REQ-004-04-create-request-usecase.md) | Use case `CreateRequest`, `CreateWithinTx` **[DONE]** | P0 |
| | [TASK-REQ-004-05](./TASK-REQ-004-05-issue-tracking-client.md) | Client `issue-tracking-service` **[DONE]** | P1 |
| | [TASK-REQ-004-06](./TASK-REQ-004-06-grpc-create-request-and-source-filters.md) | Handler gRPC, lọc nguồn **[DONE]** | P0 |
| | [TASK-REQ-004-07](./TASK-REQ-004-07-create-request-integration-tests.md) | Test tích hợp tranh chấp, rollback **[DONE]** | P0 |
| | [TASK-REQ-004-08](./TASK-REQ-004-08-gateway-request-webhook-route.md) | Route webhook ở `api-gateway` **[DONE (ở request-service)]** | P2 |
| BE-REQ-SOL-005 | [TASK-REQ-005-01](./TASK-REQ-005-01-domain-classification-and-type-change-rules.md) | Domain, luật xác nhận, đường đổi loại, migration `attempts` **[DONE]** | P0 |
| | [TASK-REQ-005-02](./TASK-REQ-005-02-classifier-port-prompt-and-relay-client.md) | Cổng classifier, prompt, client relay **[DONE]** | P0 |
| | [TASK-REQ-005-03](./TASK-REQ-005-03-processed-events-and-status-changed-consumer.md) | `processed_events`, consumer `status_changed` **[DONE]** | P0 |
| | [TASK-REQ-005-04](./TASK-REQ-005-04-propose-request-classification-usecase.md) | Use case đề xuất phân loại **[DONE]** | P0 |
| | [TASK-REQ-005-05](./TASK-REQ-005-05-confirm-request-type-usecase.md) | `ConfirmRequestType` **[DONE]** | P0 |
| | [TASK-REQ-005-06](./TASK-REQ-005-06-change-request-type-usecase.md) | `ChangeRequestType`, lịch sử **[DONE]** | P0 |
| | [TASK-REQ-005-07](./TASK-REQ-005-07-proto-and-grpc-handlers-classification.md) | Proto, handler gRPC **[DONE]** | P0 |
| | [TASK-REQ-005-08](./TASK-REQ-005-08-classification-integration-tests.md) | Test tích hợp hai dialect **[DONE]** | P0 |
| BE-REQ-SOL-006 | [TASK-REQ-006-01](./TASK-REQ-006-01-migration-return-history-and-repositories.md) | Migration return history, repository | P1 |
| | [TASK-REQ-006-02](./TASK-REQ-006-02-return-to-backlog-usecase-and-transition-category.md) | `Category` trong `TransitionRequest`, `ReturnRequestToBacklog` | P1 |
| | [TASK-REQ-006-03](./TASK-REQ-006-03-reopen-and-cancel-usecases.md) | `ReopenRequest`, `CancelRequest` | P1 |
| | [TASK-REQ-006-04](./TASK-REQ-006-04-spawn-child-request.md) | `SpawnChildRequest`, `request_links` | P1 |
| | [TASK-REQ-006-05](./TASK-REQ-006-05-proto-grpc-handlers-and-list-links.md) | Proto, handler, `ListRequestLinks` | P1 |
| | [TASK-REQ-006-06](./TASK-REQ-006-06-lifecycle-exit-integration-tests.md) | Test tích hợp hai dialect | P1 |

## Thứ tự phụ thuộc

```
[foundation] TASK-REQ-002-04/05 (repository), 001-04 (tx, outbox), 002-07 (GetRequest/List)
        │
        ▼
SOL-003:  003-01 ─▶ 003-02 ─▶ 003-03 ─┬─▶ 003-04 (integration)
                                       ├─▶ 003-05 (GetRequestFlow)
                                       └─▶ 003-06 (go/parser guard)
        │
        ▼
SOL-004:  004-01 ─┐
          004-02 ─┼─▶ 004-04 ─┬─▶ 004-06 ─▶ 004-07 (integration)
          004-03 ─┘     ▲     │         └─▶ 004-08 (gateway webhook, P2)
          004-05 ───────┘     │
                              ▼
SOL-005:  005-01 ─▶ 005-02 ─┐
          005-03 ───────────┼─▶ 005-04 ─▶ 005-05 ─▶ 005-06 ─▶ 005-07 ─▶ 005-08
                            │            (005-04 cần 003-03 và cổng no-op)
        │
        ▼
SOL-006:  006-01 ─▶ 006-02 (merge cùng 006-01) ─▶ 006-03
                              └──────────────────▶ 006-04 (cần 004-04) ─▶ 006-05 ─▶ 006-06
```

Song song được: 003-04 với 003-05 và 003-06; 004-01, 004-02, 004-03, 004-05; 005-02 với 005-03; 006-03 với 006-04.

## Ghi chú

- **Số migration kỳ vọng:** `0003` (`source_hints`, 004-01), `0004` (`classification_attempts`, 005-01), `0005` (return history, 006-01). Mỗi task đọc `ls services/request-service/migrations/postgres` trước khi đặt số; CR-REQ-007, 009, 013 cũng thêm bảng.
- **Sự kiện:** subject `orca.request.request.<event>`; `created` rồi `status_changed` giữ thứ tự nhờ cột `seq` của outbox (TASK-REQ-001-03).
- **Merge cùng đợt:** TASK-REQ-006-01 và 006-02 (CHECK ghép `status`/`returned_category` cần `Category` trong `TransitionRequest`).
- **Chưa kiểm chứng:** mọi thứ liên quan Jira thật, dev server agent thật (`ai.complete`), bí mật webhook trong `credential-broker-service`, MySQL TiDB.
- **Tham chiếu tiến:** CR-REQ-028 (`awaiting_information`) và các CR bổ sung 026 đến 036 sẽ mở rộng trạng thái và phân loại; các task ở đây không phụ thuộc vào chúng.

## Trạng thái (cập nhật 2026-10-08, life-a: CR-REQ-003 và 006)

| Task | Trạng thái |
|---|---|
| TASK-REQ-003-01, 02, 03, 04, 06 | DONE (đã kiểm chứng) |
| TASK-REQ-003-05 | DONE (đã kiểm chứng; handler gRPC nối sau hợp nhất rf/proto) |
| TASK-REQ-006-01, 02, 03, 06 | DONE (đã kiểm chứng; `attempts=0` của 006-03 nối với CR-REQ-005 và kiểm qua gRPC) |
| TASK-REQ-006-04 | DONE (con tạo qua `CreateWithinTx`, `type_hint` lưu cột) |
| TASK-REQ-006-05 | DONE (năm RPC thật) |

Phần CR-REQ-004 và 005 (agent life-b): mọi task DONE, trạng thái ở cột nội dung của bảng trên. 003-05, 004-03, 004-06, 005-07, 006-04, 006-05 đóng sau khi hợp nhất `rf/proto` và `rf/life-a`. Chi tiết: [IMPLEMENTATION-NOTES.md](../IMPLEMENTATION-NOTES.md).
