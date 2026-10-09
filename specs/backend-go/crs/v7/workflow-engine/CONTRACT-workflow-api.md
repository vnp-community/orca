# CONTRACT — Workflow Engine v2: API, sự kiện, mã lỗi (nguồn sự thật cho solution, task và test nghiệm thu)

> 📋 Proposed (chưa triển khai). Mọi solution, task và test trong series này **phải khớp tài liệu này**. Nếu cần đổi hình dạng, sửa tài liệu này trước rồi mới sửa solution/task/test.
> Nguồn: [`docs/crs/v7/workflow-engine/`](../../../../../docs/crs/v7/workflow-engine/README.md) (CR-WFD-001..011). Quy ước: REST dùng **snake_case** (khớp `workflow_routes.go` hiện có: `dag_json`, `parent_template_id`); kênh WebSocket `wscompat` dùng **camelCase** (khớp các kênh `workflow.*` hiện có); proto dùng snake_case.

## 1. REST `api-gateway` (`/v1/workflows`, cần xác thực; `tenant_id` luôn lấy từ danh tính, không nhận từ body)

Giữ nguyên các route hiện có (`POST/GET /templates`, `GET /templates/resolve`, `POST /executions`, `GET /executions/{id}`, `POST /executions/{id}/{pause|resume|cancel}`, `POST /executions/{id}/steps/adhoc`, `GET /{id}/active-executions`). Route **mới hoặc đổi** (mọi thứ chỉ thêm, không đổi tên):

| Method + đường dẫn | Body / query | Thành công | Lỗi chính |
|---|---|---|---|
| `POST /templates/validate` | `{dag_json}` | `200 {valid, diagnostics[]}` (không lưu) | `400 WORKFLOW_DEFINITION_INVALID` chỉ khi JSON hỏng; lỗi nội dung nằm trong `diagnostics` |
| `PUT /templates/{id}` | `{dag_json, expected_version (bắt buộc ≥1), name?, description?, tags?, note?}` | `200 {template}` | `409 WORKFLOW_VERSION_CONFLICT`, `422 WORKFLOW_DEFINITION_INVALID{diagnostics}`, `403 WORKFLOW_FORBIDDEN_STEP_TYPE` |
| `GET /templates/{id}/versions` | `?limit&cursor` | `200 {items:[VersionSummary], next_cursor}` | `404` |
| `GET /templates/{id}/versions/{version}` | | `200 TemplateVersion` | `404` |
| `POST /templates/{id}/versions/{version}/restore` | `{expected_version, note?}` | `200 {template}` (version mới) | `409` |
| `POST /templates/{id}/upgrade` | | `200 {draft_definition, diagnostics[], changes[]}` (không lưu) | `404`, `409 WORKFLOW_ALREADY_V2` |
| `POST /executions` | `{template_id, template_version?, project_id?, root_trace_id?, request_id?, origin_task_id?, inputs?}` | `201 {execution}` | `422 WORKFLOW_INPUT_INVALID{field}`, `429 WORKFLOW_QUOTA_EXCEEDED`, `503 WORKFLOW_DISABLED` |
| `GET /executions` | `?status&template_id&trigger_kind&from&to&limit&cursor` | `200 {items:[ExecutionSummary], next_cursor}` | |
| `GET /executions/{id}` | `?include_steps=true&include_outputs=false` | `200 Execution` | `404` |
| `POST /executions/{id}/rerun` | `{from_step_ids?, confirm_rerun_of_interrupted?}` | `201 {execution}` | `409 WORKFLOW_RUN_NOT_RESUMABLE` |
| `GET /executions/{id}/events` | `?after_seq=0&limit=200`; với `Accept: text/event-stream` trả SSE và `Last-Event-ID` = `seq` | `200 {events:[Event], last_seq}` | `404` |
| `GET /executions/{id}/artifacts` | | `200 {items:[ArtifactRef]}` | |
| `GET /artifacts/{artifact_id}` | `?download=true` | `200 {artifact}` hoặc nội dung | `409 WORKFLOW_ARTIFACT_CHANGED`, `404`, `410 WORKFLOW_ARTIFACT_UNAVAILABLE` |
| `GET /gates` | `?execution_id&status&mine=true&limit&cursor` | `200 {items:[GateSummary], next_cursor}` | |
| `GET /gates/{id}` | | `200 Gate` | `404`, `403` |
| `POST /gates/{id}/decide` | `{decision, comment?}` | `200 Gate` | `403 WORKFLOW_GATE_FORBIDDEN`, `409 WORKFLOW_GATE_CLOSED`, `410 WORKFLOW_GATE_EXPIRED`, `422` quyết định ngoài danh sách |
| `POST /templates/{id}/triggers` | `{kind, config, input_mapping?, pinned_version?, dedupe_window_sec?, enabled?}` | `201 Trigger` (với `webhook`: kèm `secret` **một lần**) | `422`, `429 WORKFLOW_QUOTA_EXCEEDED` |
| `GET /templates/{id}/triggers` | | `200 {items:[Trigger]}` | |
| `PUT /triggers/{id}` / `DELETE /triggers/{id}` | cùng body như tạo | `200` / `204` | `404` |
| `GET /triggers/{id}/firings` | `?limit&cursor` | `200 {items:[Firing], next_cursor}` | |
| `POST /hooks/workflows/{trigger_id}` | **công khai** (không cookie); header `X-Orca-Signature: sha256=<hex>`, `X-Orca-Timestamp: <unix>`, `Idempotency-Key?`; thân ≤ 256 KiB | `202 {firing_id, outcome}` | `401` chữ ký/thời gian sai (thân rỗng), `404` (không lộ tồn tại), `413`, `429` |
| `GET /step-types` | | `200 {types:[{name, deprecated, config_schema, output_schema, capabilities, requires_permission}]}` | |
| `GET /capabilities` | | `200 {engine:"go", schema_versions:[1,2], features:[…], limits:{…}}` | |
| `GET /settings` / `PUT /settings` | `{engine_v2_enabled?, mcp_call_enabled?, gate_admin_override?, strict_versioning?, retention_days?, quotas?}` | `200 Settings` | `403` (chỉ admin tenant) |

Mã HTTP ↔ mã lỗi (`error.code`): `WORKFLOW_DEFINITION_INVALID`=422, `WORKFLOW_VERSION_CONFLICT`=409, `WORKFLOW_INPUT_INVALID`=422, `WORKFLOW_EXPRESSION_INVALID`=422 (lúc lưu) hoặc bước `failed` (lúc chạy), `WORKFLOW_STEP_TYPE_UNKNOWN`=422, `WORKFLOW_FORBIDDEN_STEP_TYPE`=403, `WORKFLOW_GATE_FORBIDDEN`=403, `WORKFLOW_GATE_CLOSED`=409, `WORKFLOW_GATE_EXPIRED`=410, `WORKFLOW_GATE_NO_APPROVER`=422/bước `failed`, `WORKFLOW_RUN_NOT_RESUMABLE`=409, `WORKFLOW_QUOTA_EXCEEDED`=429, `WORKFLOW_DISABLED`=503, `WORKFLOW_ARTIFACT_CHANGED`=409, `WORKFLOW_ARTIFACT_UNAVAILABLE`=410, `WORKFLOW_ALREADY_V2`=409, `WORKFLOW_STEP_TIMEOUT`, `WORKFLOW_STEP_INTERRUPTED`, `WORKFLOW_STEP_OUTPUT_INVALID` (ba mã này là lỗi **của bước**, nằm trong `steps[].error_code`, không phải mã HTTP). Envelope lỗi giữ như gateway hiện có: `{"error":{"code","message","details?"}}`. Không bao giờ trả 5xx cho lỗi do người dùng, không bao giờ 501.

## 2. Kênh WebSocket `wscompat` (camelCase)

| Kênh | Args[0] | Trả về |
|---|---|---|
| `workflow.execute` | `{templateId, templateVersion?, projectId?, rootTraceId?, requestId?, originTaskId?, inputs?}` | `{execution}` |
| `workflow.getExecution` | `{executionId, includeSteps?, includeOutputs?}` | `Execution` (camelCase) |
| `workflow.listExecutions.v2` | `{status?, templateId?, triggerKind?, from?, to?, limit?, cursor?}` | `{items, nextCursor}` (kênh `workflow.listExecutions` cũ giữ nguyên, mảng trần) |
| `workflow.rerun` | `{executionId, fromStepIds?, confirmRerunOfInterrupted?}` | `{execution}` |
| `workflow.template.validate` | `{dagJson}` | `{valid, diagnostics}` |
| `workflow.template.update` | `{id, dagJson, expectedVersion, name?, note?}` | `{template}` |
| `workflow.template.versions.list` / `.get` / `.restore` | `{templateId, …}` | như REST |
| `workflow.template.upgrade` | `{templateId}` | `{draftDefinition, diagnostics, changes}` |
| `workflow.stepTypes.list`, `workflow.capabilities` | `{}` | như REST |
| `workflow.stream.subscribe` | `{executionId, afterSeq?}` | `{subscriptionId}`; sự kiện đẩy qua `PushEvent` kênh `workflow.event` |
| `workflow.stream.unsubscribe` | `{subscriptionId}` | `{ok:true}` |
| `workflow.gate.list` / `.get` / `.decide` | `{…}` | như REST |
| `workflow.trigger.list/create/update/delete`, `workflow.trigger.firings` | `{…}` | như REST |
| `workflow.artifact.get` | `{artifactId}` | `{artifact}` |
| `workflow.settings.get` / `.update` | `{…}` | `Settings` |

Mọi kênh mới có test hợp đồng JSON (đủ trường camelCase) và test từ chối khi thiếu quyền.

## 3. Hình dạng dữ liệu (JSON; REST snake_case, WS camelCase của cùng trường)

```jsonc
// Diagnostic
{ "path": "steps[2].config.script", "code": "CONFIG_SCHEMA", "severity": "error|warning", "message": "…" }

// Execution
{ "id": "uuid", "template_id": "uuid", "template_version": 3, "definition_hash": "sha256hex",
  "status": "pending|running|waiting|paused|completed|failed|cancelled",
  "trigger": { "kind": "manual|schedule|event|webhook|workflow|task", "id": "uuid|null" },
  "run_as": "uuid", "project_id": "uuid|null", "root_trace_id": "…", "origin_task_id": "uuid|null",
  "parent_execution_id": "uuid|null", "parent_step_id": "string|null",
  "inputs": { }, "outputs": { }, "row_version": 7,
  "started_at": "RFC3339", "ended_at": "RFC3339|null", "deadline_ms": 0,
  "steps": [ StepRun ] }

// StepRun
{ "step_id": "a", "type": "agent", "status": "pending|running|waiting|completed|failed|skipped|cancelled",
  "attempt": 1, "wave": 0, "skip_reason": "guard_false|dependency_skipped|upstream_failed|null",
  "error_code": "string|null", "error_message": "string|null",
  "started_at": "…", "ended_at": "…", "outputs_ref": [ { "name": "x", "type": "string" } ],
  "outputs": { }   // chỉ khi include_outputs=true và có quyền; đã che secret
}

// Event (SSE/JSON/NATS)
{ "schema_version": 1, "seq": 12, "ts": "RFC3339", "tenant_id": "uuid", "execution_id": "uuid",
  "type": "execution.started|execution.status_changed|execution.completed|execution.failed|execution.cancelled|step.started|step.attempt_failed|step.completed|step.failed|step.skipped|step.waiting|step.output|gate.opened|gate.decided|gate.expired",
  "data": { } }   // theo bảng CR-WFD-008 mục 2.1

// Gate
{ "id": "uuid", "execution_id": "uuid", "step_id": "review", "attempt": 1,
  "status": "open|decided|expired|cancelled",
  "snapshot": { "title": "…", "instructions": "…", "context": { } },
  "decisions_allowed": ["approve","reject"], "required": { "users": ["uuid"], "mode": "any|all", "min_approvals": 1 },
  "votes": [ { "user_id": "uuid", "decision": "approve", "comment": "…", "voted_at": "…" } ],
  "outcome": "approve|null", "opened_at": "…", "expires_at": "…", "decided_at": "…|null" }

// ArtifactRef
{ "artifact_id": "uuid", "name": "report", "version": 2, "sha256": "hex", "media_type": "text/markdown", "size_bytes": 1234 }

// Trigger
{ "id": "uuid", "template_id": "uuid", "pinned_version": null, "kind": "schedule|event|webhook|workflow",
  "config": { }, "input_mapping": { }, "enabled": false, "dedupe_window_sec": 300, "run_as": "uuid",
  "webhook_url": "…/hooks/workflows/{id}" }

// Firing
{ "id": "uuid", "trigger_id": "uuid", "dedupe_key": "…", "fired_at": "…",
  "outcome": "started|deduplicated|throttled|input_invalid|loop_guard|forbidden|error", "execution_id": "uuid|null", "reason": "…" }
```

## 4. Chủ đề NATS (JetStream, outbox)

Giữ nguyên chủ đề và trường hiện có (`orca.workflow.step.completed|failed`, `orca.workflow.execution.completed|failed`) và **chỉ thêm** trường. Chủ đề mới: `orca.workflow.execution.started|cancelled|status_changed`, `orca.workflow.step.started|skipped|waiting|attempt_failed`, `orca.workflow.gate.opened|decided|expired`. Payload NATS **không** nhúng output hay `snapshot`, chỉ tham chiếu (CR-WFD-003 2.5).

## 5. Metric (Prometheus)

`workflow_executions_total{status,trigger_kind}`, `workflow_step_duration_seconds{step_type,status}` (histogram), `workflow_step_attempts_total{step_type,outcome}`, `workflow_gates_open`, `workflow_trigger_firings_total{kind,outcome}`, `workflow_quota_rejections_total{quota}`, `workflow_dispatch_queue_depth`, `workflow_events_lag_seconds`. Không dùng id người dùng hay tenant làm nhãn.

## 6. Cờ và cấu hình

| Tên | Loại | Mặc định |
|---|---|---|
| `WORKFLOW_ENGINE_V2_ENABLED` | biến môi trường (cờ tổng) | `false` |
| `workflow_engine_v2`, `workflow_mcp_call_enabled`, `workflow_gate_admin_override`, `workflow_strict_versioning` | `workflow.tenant_settings` (theo tenant) | `false` |
| `retention_days` | `tenant_settings` | 90 (7…730) |
| hạn mức | `tenant_settings.quotas` JSON | theo CR-WFD-010 mục 2.5 |

Hiệu lực = cờ tổng ∧ cờ tenant, **fail closed**.

## 7. Bất biến mà mọi task phải giữ

1. Hai dialect (Postgres, MySQL) cho mọi bảng, migration, repository; test hai dialect.
2. Postgres bật `ENABLE` + `FORCE ROW LEVEL SECURITY`; MySQL lọc `tenant_id` ở mọi `WHERE`; test đọc chéo tenant.
3. `schemaVersion=1` chạy **y như cũ** (đường `legacydispatch`); thay đổi nào làm lệch hành vi v1 bị từ chối.
4. Không chạy bằng quyền cao hơn người khởi chạy; mọi bước dùng danh tính người chạy hoặc `run_as`.
5. Không nhúng giá trị secret vào định nghĩa, output, sự kiện, log; dùng `{"$secret": …}` và bộ che.
6. Mọi thay đổi trạng thái execution dùng CAS theo `row_version` và ghi sự kiện **trong cùng giao dịch**.
7. Thêm mới, không đổi tên: proto (số trường mới), chủ đề NATS, kênh `wscompat`, cột DB.
