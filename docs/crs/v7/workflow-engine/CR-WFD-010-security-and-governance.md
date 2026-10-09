# CR-WFD-010 — An toàn và quản trị: quyền, secret, SSRF, hạn mức, audit, retention, feature flag

| Trường | Giá trị |
|--------|---------|
| **CR ID** | CR-WFD-010 |
| **Tên** | Mô hình quyền cho template/execution/gate/trigger, quản lý secret, chống SSRF cho webhook, hạn mức theo tenant, che dữ liệu nhạy cảm, retention, cờ `workflow_engine_v2` theo tenant |
| **Loại** | Feature (xuyên suốt) |
| **Priority** | 🔴 P0 |
| **Effort** | Medium |
| **Phiên bản** | v1.0 |
| **Ngày tạo** | 2026-10-08 |
| **Trạng thái** | 📝 Đề xuất, chưa triển khai |
| **Phụ thuộc** | CR-WFD-001 |
| **Mở khoá** | Điều kiện bật tính năng cho tenant của CR-WFD-002…009 |
| **Tác động** | `workflow-service` (authz, usecase, executor `webhook`/`shell`/`agent`, config), `api-gateway` (kiểm quyền kênh), `backend-go/policy/orca-authz` (Rego), `credential-broker-service` (secret), Vault (secret trigger), migration |

---

## 1. Bối cảnh và vấn đề

1. **Phạm vi template chưa đủ mịn.** Template có `scope` (company/team/personal) và `visibility` (private/team/company/public); cơ chế nâng phạm vi cần `Approval` và quyền admin qua OPA (`WSI/domain/approval.go:41-55`). Chưa thấy quyền **chạy**, **sửa**, **quyết cổng**, **quản trigger** tách bạch (chưa kiểm chứng đầy đủ các điểm kiểm quyền hiện có).
2. **Bước thực thi có tác động mạnh mà chưa có chốt chặn nào ngoài người soạn.** `shell` chạy `script` trên dev server, `agent` chạy AI với `trustPreset`, `webhook` gọi URL tuỳ ý, `commit_push` đẩy mã. Quyền ai được tạo template chứa loại bước nào chưa phân.
3. **Webhook chống SSRF còn "stub"** theo README của service (chưa kiểm chứng chi tiết): URL nội bộ, địa chỉ metadata đám mây, `file://`, chuyển hướng.
4. **Secret.** Bước có `env` và `config` có thể chứa secret dạng chữ thường; `step_executions.output`, sự kiện, log có thể lộ (chưa có che dữ liệu cho workflow).
5. **Không có hạn mức.** Chỉ có pool 10 bước/wave; không giới hạn số execution, bước, kích thước, trigger toàn tenant.
6. **Không có retention.** Execution, output, artifact, sự kiện tồn tại vô hạn.
7. **Không có cờ.** Không có cách bật dần tính năng mới theo tenant, trong khi series thay đổi hành vi dispatcher (CR-WFD-006).

## 2. Giải pháp đề xuất

### 2.1 Cờ tính năng

| Cờ | Mặc định | Phạm vi |
|---|---|---|
| `workflow_engine_v2` | `false` | tenant; hiệu lực = cờ tổng (biến môi trường) ∧ cờ tenant, **fail closed** (như mẫu `code_intel_enabled` của v7) |
| `workflow_mcp_call_enabled` | `false` | tenant (CR-WFD-005) |
| `workflow_gate_admin_override` | `false` | tenant (CR-WFD-004) |
| `workflow_strict_versioning` | `false` | tenant (CR-WFD-001) |

Cờ tắt: template `schemaVersion=2` không lưu/chạy được (`WORKFLOW_DISABLED`); template `schemaVersion=1` chạy bằng dispatcher cũ **y như hiện nay** (CR-WFD-011). Bảng `workflow.tenant_settings` lưu cờ; chỉ admin tenant đổi, có audit.

### 2.2 Mô hình quyền

Hành động và vai trò tối thiểu (kiểm ở **backend và Rego**, không dựa vào UI):

| Hành động | Ai |
|---|---|
| Xem template | người có quyền theo `visibility/scope` |
| Tạo/sửa template cá nhân | người dùng; **chỉ loại bước thuộc danh sách "được phép cho người dùng"** của tenant |
| Dùng loại bước "nguy hiểm" (`shell`, `webhook`, `commit_push`, `mcp_call`, `action` có tác động ghi) | cần quyền `workflow.step.<type>` cấp bởi admin tenant; lưu template chứa loại này mà thiếu quyền bị từ chối (`WORKFLOW_FORBIDDEN_STEP_TYPE`) |
| Chạy template | người có quyền xem + quyền `workflow.run`; **mỗi bước dùng danh tính người chạy** (hoặc `run_as` của trigger) |
| Sửa/khôi phục phiên bản | chủ template hoặc người được cấp quyền sửa |
| Quyết cổng | CR-WFD-004 2.4 |
| Tạo/bật trigger | quyền `workflow.trigger.manage`; chỉ trigger cho template mà người đó chạy được |
| Xem output/artifact/log | người xem execution; **che secret** (2.4) |
| Công bố company/public | giữ cơ chế `Approval` hiện có |

Quy tắc xuyên suốt: **không bao giờ chạy bằng quyền cao hơn người khởi chạy**. Bước `sub_workflow` và `foreach` kế thừa danh tính đó. Quyết định quyền sống trong Rego (`backend-go/policy/orca-authz`) để kiểm thử được bằng `opa test`, như các service khác.

### 2.3 Chống SSRF cho `webhook` và `mcp_call`

| Biện pháp | Nội dung |
|---|---|
| Lược đồ | Chỉ `https` (và `http` khi tenant bật rõ cho môi trường phát triển) |
| Giải DNS | Giải tên **một lần**, kiểm địa chỉ đích, rồi kết nối **vào đúng IP đã kiểm** (chống DNS rebinding) |
| Chặn dải | Loopback, link-local, dải riêng (RFC 1918, ULA), `169.254.0.0/16` (metadata), `0.0.0.0`, multicast; cấu hình danh sách cho phép theo tenant cho đích nội bộ hợp lệ |
| Chuyển hướng | Mặc định không theo; nếu bật, kiểm lại từng bước và tối đa 3 lần |
| Giới hạn | Thời gian chờ, kích thước phản hồi ≤ 1 MiB, không gửi header xác thực của Orca |
| Secret trong URL/header | Chỉ qua tham chiếu secret (2.4), không chữ thường |

### 2.4 Secret và che dữ liệu

- `config` được phép chứa **tham chiếu secret** `{"$secret": "<tên>"}` thay vì giá trị; giá trị lưu ở kho secret hiện có (`credential-broker-service`/Vault) và chỉ giải ở **thời điểm gọi executor**, không ghi vào `definition`, `step_executions`, sự kiện hay log.
- `ValidateDefinition` cảnh báo (`warning`) khi phát hiện chuỗi giống secret trong `config`/`env` (mẫu khoá API, token) và đề nghị dùng tham chiếu; tenant có thể nâng thành lỗi.
- Bộ che dữ liệu (`redaction rules`, đã có mẫu ở `mcpserver/tools/redaction_rules.go`): áp lên `output`, `step.output`, `error_message`, `snapshot.context` của cổng, trước khi lưu và trước khi phát. Giá trị secret đã giải được đăng ký vào bộ che của lượt chạy để thay bằng `***`.
- Giới hạn trung thực: bước tự in secret ra dạng đã biến đổi (base64, cắt khúc) có thể lọt; ghi vào tài liệu cho người viết workflow, không hứa che tuyệt đối.

### 2.5 Hạn mức theo tenant

Cấu hình trong `workflow.tenant_settings`, mặc định đề xuất (cần xác nhận, O7):

| Hạn mức | Mặc định |
|---|---|
| Template | 500 |
| Bước/template | 200 |
| Execution `running` đồng thời | 20 |
| Execution `waiting` đồng thời | 200 |
| Bước chạy đồng thời toàn tenant | 50 |
| Trigger | 500 |
| Lần bắn trigger mỗi phút/trigger | 60 |
| Artifact `inline` tổng | 1 GiB |
| Kích thước input / output bước / output execution | 256 KiB / 1 MiB / 16 MiB |
| Độ sâu `sub_workflow` | 5 |
| Số execution con trong `foreach` | 200 |

Vượt hạn mức: `WORKFLOW_QUOTA_EXCEEDED` kèm tên hạn mức; số liệu hiển thị trong UI. Quản trị viên xem mức dùng.

### 2.6 Retention

`workflow.tenant_settings.retention_days` (mặc định 90, 7…730): tác vụ nền xoá execution đã kết thúc, `step_executions`, `step_attempts`, `execution_events`, `run_artifacts` (kể cả tham chiếu `host_file`; **không xoá tệp trên dev server**), `trigger_firings`, và cổng đã đóng quá hạn. Phiên bản template **không** bị xoá khi execution còn tham chiếu. Xoá theo lô nhỏ, có giới hạn tốc độ, ghi audit tổng.

### 2.7 Audit

Nhật ký audit chuẩn của nền tảng ghi: ai, làm gì, trên đối tượng nào, kết quả. Danh mục sự kiện ở CR-WFD-008 2.5; thêm: đổi cờ, đổi hạn mức, cấp quyền loại bước, override cổng, đọc output nhạy cảm. Không ghi giá trị secret.

### 2.8 Cách ly tenant

Mọi bảng mới có `tenant_id`; Postgres bật `ENABLE` + `FORCE ROW LEVEL SECURITY`, MySQL lọc `tenant_id` ở mọi truy vấn, theo mẫu đã dùng ở các service Go khác. Có test kiểm đọc chéo tenant cho từng bảng mới.

## 3. Tiêu chí chấp nhận

1. Cờ `workflow_engine_v2` tắt: template v2 bị từ chối, template v1 chạy như cũ.
2. Người không có quyền `workflow.step.shell` không lưu được template có bước `shell`.
3. Chạy template bởi người A: mọi kiểm quyền trong bước dùng danh tính A; A không làm được gì qua workflow mà A không làm được trực tiếp (test đối chứng).
4. Webhook tới `169.254.169.254`, `127.0.0.1`, hoặc tên miền giải ra IP riêng bị chặn; thử DNS rebinding không vượt được.
5. Giá trị secret không xuất hiện trong `definition`, `step_executions.output`, sự kiện, log hay `error_message`.
6. Vượt từng hạn mức trong bảng 2.5 trả `WORKFLOW_QUOTA_EXCEEDED` đúng tên.
7. Tác vụ retention xoá đúng dữ liệu quá hạn, không xoá phiên bản template còn được tham chiếu, và không xoá tệp trên dev server.
8. Test Rego (`opa test`) bao phủ mọi hàng ở 2.2; test RLS cho mọi bảng mới.

## 4. Tác động và rủi ro

| Mục | Ghi chú |
|---|---|
| Migration | Tạo `tenant_settings`; thêm cột/bảng quyền loại bước |
| Tương thích | Template hiện có chứa `shell`/`webhook` do người dùng thường tạo có thể bị coi là vi phạm sau khi bật quyền loại bước; cần giai đoạn cảnh báo, không chặn hồi tố (CR-WFD-011) |
| Rủi ro | Che dữ liệu bằng quy tắc không tuyệt đối; cần nói rõ trong tài liệu |
| Rủi ro | Hạn mức mặc định chưa dựa trên số đo thật; có thể quá chặt hoặc quá lỏng |

## 5. Chưa kiểm chứng / câu hỏi mở

- Kiểm quyền hiện có ở `workflow-service` và gateway cho từng kênh (chưa đọc hết, chỉ thấy OPA ở luồng duyệt công bố).
- Nơi lưu "vai trò" của tenant để gán `workflow.*` (auth-service hay tenant-service).
- Có cho phép `http` (không TLS) hay không: mặc định không.
- Số mặc định hạn mức và retention cần chốt khi duyệt (O7).
