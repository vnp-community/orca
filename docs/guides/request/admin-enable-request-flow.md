# Bật, tắt và rollout luồng Request (dành cho admin và ops)

**Cập nhật:** 2026-10-09 · Đối chiếu với: `backend-go/services/request-service/internal/adapter/grpc/flow_gate.go` (`flowMethodClass`),
`internal/usecase/flow_settings.go`, `internal/domain/flow_settings.go`, `deploy/dev/docker-compose.yml`, `deploy/dev/.env.example`,
`deploy/dev/scripts/ensure-request-env.sh`, [README triển khai](../../../deploy/dev/README.md),
[CR-REQ-025 mục 2.2, 2.6, 2.7](../../crs/v6/request-quality-rollout/CR-REQ-025-e2e-tests-feature-flag-rollout.md).

## Cờ hai tầng

| Tầng | Nơi đặt | Mặc định |
|---|---|---|
| Tổng | biến `REQUEST_FLOW_ENABLED` của `request-service` (compose dev: `${REQUEST_FLOW_ENABLED:-false}`) | `false` |
| Theo tenant | dòng `request.tenant_settings` của tenant, qua RPC `SetRequestFlowSettings` / `GetRequestFlowSettings` | không có dòng = tắt |

**Hiệu lực = cả hai bật.** Thiếu dòng hoặc lỗi đọc = tắt (fail closed). `GetRequestFlowSettings` trả giá trị **hiệu lực**.
Cờ theo **loại** Request chưa có cơ chế (cờ chỉ theo tenant); câu hỏi mở Q1 của CR-REQ-025. `task-service` không đọc cờ; cờ không ẩn hay xoá dữ liệu.

## Điều kiện trước khi bật

| Cần | Cách kiểm |
|---|---|
| Bốn token nội bộ có giá trị: `REQUEST_INTERNAL_CALLER_TOKEN`, `REQUEST_SERVICE_INTERNAL_TOKEN`, `PROJECT_INTERNAL_CALLER_TOKEN`, `AUTH_DIRECTORY_CALLER_TOKEN` (cái cuối cần khi bật `REQUEST_APPROVAL_ENABLED`) | `bash deploy/dev/scripts/ensure-request-env.sh` (chỉ điền chỗ thiếu, không in giá trị; thêm `--remote` cho server). Thiếu thì mọi RPC trả `REQUEST_FORBIDDEN` và `/readyz` của `request-service` báo chưa sẵn sàng |
| Dev server đã được duyệt, đang kết nối, và project gắn với nó | `devServer.list` thấy `status=connected`, `approvalStatus=approved`; không thì phân loại và các bước AI dừng với `REQUEST_AI_AGENT_UNAVAILABLE` |
| AI trên dev server: Claude Code đã đăng nhập (`claude auth login`) hoặc khoá API trong môi trường agent | không có thì phân loại, Solution, Plan dừng với `REQUEST_AI_NOT_CONFIGURED` (Request đứng yên, không mất dữ liệu) |

## Bật cho tenant

1. Ops đặt `REQUEST_FLOW_ENABLED=true` trong `deploy/dev/.env` (compose dev: `${REQUEST_FLOW_ENABLED:-false}` cho `request-service`) rồi áp dụng:
   `bash deploy/dev/scripts/redeploy-local.sh request-service` trên stack local (build, restart, smoke test), hoặc `docker compose up -d request-service` trên server.
   Để dùng cổng phê duyệt cần thêm `REQUEST_APPROVAL_ENABLED=true` (compose dev mặc định `false`).
2. Admin của tenant gửi kênh WS `request.flowSet` với `{enabled:true}` (hoặc gọi trực tiếp RPC `SetRequestFlowSettings`). Kênh `request.flowStatus` đọc
   giá trị **hiệu lực** (cả hai tầng). Không có UI cho việc này; từ dòng lệnh dùng client của bộ e2e (đăng nhập bằng tài khoản admin trong `deploy/dev/.env`):

   ```bash
   cd tests/stages/e2e/orca-flows && python3 - <<'EOF'
   from orca_stack_config import load_config
   from orca_session import OrcaSession
   s = OrcaSession(load_config("flag")); s.login()
   print(s.ws().call("request.flowSet", {"enabled": True}))   # chỉ role admin; lần Set nào cũng được audit
   print(s.ws().call("request.flowStatus", {}))               # {'enabled': True} khi cả hai tầng bật
   s.close()
   EOF
   ```
3. Với `issue-status-sync`: đặt `REQUEST_SERVICE_ADDR` và `REQUEST_SERVICE_INTERNAL_TOKEN` (trùng `SERVICE_INTERNAL_TOKEN` của `request-service`; compose dev đã nối
   `REQUEST_SERVICE_INTERNAL_TOKEN` của `.env` vào cả hai), xem [Jira ↔ Orca](../jira/jira-orca-mapping.md). Token rỗng thì `LookupRequestBySource` bị từ chối.
4. Kiểm: `python3 tests/stages/e2e/orca-flows/run_acceptance.py request` phải xanh. Suite này tự bật cờ tenant ở `RQL-01` và thử cả đường tắt cờ (`RQL-02..03`).

Chỉ role `admin` được Set; người khác nhận lỗi `REQUEST_FLOW_ADMIN_ONLY`. Mỗi lần Set ghi audit `request.flow.set`, **kể cả lần bị từ chối**
(`outcome=denied`).

## Khi cờ tắt, RPC nào còn chạy

| Nhóm | RPC | Khi tắt |
|---|---|---|
| Đọc | `GetRequest`, `ListRequests`, `GetRequestFlow`, `ListRequestTypeHistory`, `ListRequestLinks`, `ListApprovals`, `GetApproval`, `ListPendingForUser`, `GetRequestFlowSettings`... | chạy |
| Thoát an toàn | `CancelRequest`, `ReturnToBacklog`, `Reject` và `Cancel` của Approval | chạy |
| Nội bộ | `ReportTaskOutcome`, `LookupRequestBySource`, consumer outbox, hết hạn Approval | chạy (không mất kết quả việc đang chạy) |
| Đi tiếp | `CreateRequest`, `ClassifyRequest`, `ConfirmRequestType`, `ChangeRequestType`, `ReopenRequest`, `SpawnChildRequest`, `Approve`, `RequestApproval`, `ExtendApproval` và các RPC đi tiếp khác | `FailedPrecondition` `REQUEST_FLOW_DISABLED` |

Bảng phân loại đầy đủ là `flowMethodClass` trong `flow_gate.go`; test quét proto đảm bảo mọi RPC có một dòng. Một số RPC trong bảng đó
(ví dụ `RequestImpactAssessment`, `CheckReadiness`, `ListContextSources`) chưa có handler nên trả `Unimplemented`; `GenerateSolution`, `GeneratePlan`, `CommitPlan` và `StartPhase` đã có handler.

## Ngôn ngữ đầu ra của AI

Biến `REQUEST_DEFAULT_OUTPUT_LANGUAGE` của `request-service` (CR-REQ-046) chọn ngôn ngữ AI viết phân tích (Solution, chẩn đoán, Findings, Answer) và Plan: `vi` (mặc định khi không đặt biến) hoặc `en`.
Khoá JSON, giá trị enum, id, đường dẫn và ngữ pháp OpenSpec giữ tiếng Anh. Giá trị được đọc một lần lúc khởi động (đổi xong cần `redeploy-local.sh request-service`), và **giá trị khác `vi`/`en` làm service không khởi động**,
khác ba biến `REQUEST_DEFAULT_ENGINE`, `_SOLUTION_AGENT`, `_EXECUTION_MODE` chỉ cảnh báo. Biến này nằm ngoài cài đặt theo dự án: áp cho mọi dự án và tenant của triển khai.

Hạn chế: ngôn ngữ không theo ngôn ngữ giao diện (giao diện chưa có locale `vi`) và không được phát hiện từ nội dung Request; đặt `en` thì AI viết tiếng Anh ngay cả khi Request bằng tiếng Việt.
Chi tiết triển khai: [README triển khai](../../../deploy/dev/README.md) mục "Ngôn ngữ đầu ra của AI".

## Rollout đề xuất (5 giai đoạn)

Số ngày và số Request là **đề xuất ban đầu, chưa có số đo thực**. Hiện chưa có tenant nào chạy các giai đoạn này.

| GĐ | Phạm vi | Điều kiện vào | Điều kiện chuyển tiếp |
|---|---|---|---|
| 0. Nội bộ | `REQUEST_FLOW_ENABLED=true` ở dev; một tenant thử | T1 xanh hai dialect; `check-request-service-wiring.sh` xanh | không có alert `RequestOutboxLag`, `RequestStuck` trong 3 ngày |
| 1. Dogfood | tenant đội Orca, mọi loại | T2 xanh 5 đêm liên tiếp; một issue Jira thật chạy hết kịch bản E18 bằng tay | 2 tuần không mất dữ liệu; `orca_request_stuck` về 0 sau mỗi lần xử lý |
| 2. Beta | tenant opt-in (admin bật `request.flowSet`); loại rủi ro thấp trước: `task`, `docs`, `question`, `spike` | review quyền duyệt; runbook được diễn tập | `orca_request_returned_total` không tăng bất thường; không Jira nào bị chuyển sai |
| 3. Mở rộng loại | thêm `bug`, `refactor`, `change_request`; sau cùng `security`, `performance`, `ops_request`, `hotfix` | các loại này cần năng lực agent chưa kiểm chứng | mỗi loại có ít nhất 5 Request thật hoàn tất |
| 4. GA | cờ theo tenant bật được; mặc định `false` của biến tổng không đổi cho tới quyết định riêng | review bảo mật độc lập | |

"Bật theo loại" ở GĐ 2 và 3 **chưa có cơ chế** (cờ chỉ theo tenant); hiện chỉ có thể hướng dẫn tenant chỉ dùng một số loại khi xác nhận.
Các giai đoạn Solution và Plan đã chạy thật trong bộ e2e `orca-flows`; thực thi Phase sau khi duyệt Plan có mã nhưng **chưa được chạy trên stack thật** ([README](./README.md)), nên GĐ 1 cần một lượt thực thi thủ công có chủ đích trước khi coi là xong.

## Rollback (7 bước)

1. **Tắt cờ**: `request.flowSet {enabled:false}` cho tenant, hoặc `REQUEST_FLOW_ENABLED=false` rồi khởi động lại `request-service`. Có hiệu lực ngay cho ghi mới.
2. Request đang `executing`: cho chạy xong (callback vẫn xử lý) hoặc `ReturnToBacklog`.
3. Approval đang `pending`: `Reject` hoặc `Cancel` được; sau khi bật lại, người duyệt xử lý tiếp.
4. **Không chạy `down` migration** của `task-service` (`CHECK` loại `plan`, `phase`) khi đã có dòng loại đó. Dữ liệu giữ nguyên.
5. Jira: trạng thái đã chuyển không tự lùi; đổi tay nếu cần.
6. Kill switch riêng cho tool MCP `request_*`: chính sách tenant của MCP hoặc `MCP_ENABLED=false` ([Request từ agent](./requests-from-agents-mcp.md)).
7. Diễn tập rollback là điều kiện của GĐ 2. **Chưa diễn tập** (xem cuối [runbook](./runbook-request-flow.md)).

Sự cố khẩn: xem [runbook](./runbook-request-flow.md) mục "Khẩn cấp: tắt cờ".
