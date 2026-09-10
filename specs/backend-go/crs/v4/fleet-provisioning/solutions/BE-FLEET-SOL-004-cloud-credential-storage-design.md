# BE-FLEET-SOL-004: Cloud provider credential storage — design proposal for TASK-BE-FLEET-009's security review

> **🟡 DESIGN DRAFTED (2026-09-09) — chưa được security review.** Đây là
> tài liệu thiết kế TASK-BE-FLEET-009 yêu cầu ("(1) 1 tài liệu thiết kế đề
> xuất, KHÔNG phải code"). Không có dòng code Go/TS nào được viết ở đây
> hay ở bất kỳ đâu khác cho hạng mục này. Tài liệu này trả lời đầy đủ 6
> câu hỏi của task, có câu trả lời được người giao việc chốt trực tiếp
> (câu 1), có câu trả lời được đề xuất dựa trên code thật đã có sẵn trong
> repo (câu 2-5), và 1 câu (câu 6) **cố ý để ngỏ** — không tự chỉ định
> người/role chịu trách nhiệm review.

**CR:** CR-FLEET-002 | **Task:** [TASK-BE-FLEET-009](../tasks/TASK-BE-FLEET-009-cloud-credential-storage-design-security-review.md)

---

## Tóm tắt đề xuất (đọc trước khi đi vào chi tiết)

**Không xây mới.** `credential-broker-service` đã có sẵn 1 hệ thống lưu
trữ credential tổng quát, tenant-scoped, mã hoá tại Vault, có audit log
bắt buộc đồng bộ, đã dùng thật cho 5 category khác (SCM OAuth, issue
tracker OAuth, AI provider key, SSH, service secret — xem
`proto/orca/credentialbroker/v1/credentialbroker.proto:79-84`). Đề xuất:
thêm 1 category mới `CREDENTIAL_CATEGORY_CLOUD_PROVIDER`, tái sử dụng
100% hạ tầng ghi/đọc/xoay-vòng/thu-hồi/audit đã có — không viết 1 usecase
lưu trữ mới nào trong `infra-fleet-service`.

Điều này trả lời trực tiếp phần "có tái dùng cơ chế encryption-at-rest đã
có sẵn cho secret khác trong hệ thống không" của câu hỏi 1 — **có, và cơ
chế đó không chỉ tồn tại mà đã production-tested cho use case gần như
giống hệt** (OAuth token/API key của bên thứ 3, lưu dài hạn, cần
audit/revoke).

## Câu 1 — Nơi lưu trữ

**Đã chốt bởi người giao việc (2026-09-09): Vault KV engine, dùng chung 1
Vault container/instance đã có sẵn trong hệ thống.**

Khớp hoàn toàn với hạ tầng đã có: `common/secrets.Client` (dùng bởi
`credential-broker-service` qua `internal/adapter/vault.SecretStore`) đã
có sẵn `KVWrite`/`KVRead`/`KVDestroyMetadata`
(`common/secrets/vault.go:100,111,281`), dùng đúng 1 Vault instance mà
`credential-broker-service` đã dial. Không dựng Vault instance riêng,
không mount KV engine mới thủ công ngoài quy trình — dùng đúng
`kvMount` mà 5 category hiện có đang dùng
(`internal/usecase/write_credential.go:77`, `resolve_credential.go:109`).

Ràng buộc #1 của task ("không tái dùng `domain.SshTarget.VaultSSHRole`")
vẫn được tôn trọng: `VaultSSHRole` là 1 khái niệm hoàn toàn khác (Vault SSH
secrets engine role, ký cert ngắn hạn qua `SSHSignPublicKey`) — category
mới này dùng KV engine + Transit encrypt, cùng cơ chế 5 category kia đang
dùng, không đụng SSH secrets engine.

## Câu 2 — Vòng đời credential

**Đề xuất: tái sử dụng nguyên trạng vòng đời đã có của
`credential-broker-service`, không tự thiết kế mới.**

- **Rotation**: `RotateCredential` usecase đã tồn tại
  (`internal/usecase/rotate_credential.go`) — ghi version mới, không xoá
  version cũ (Vault KV v2 tự giữ lịch sử).
- **Ai được cấp quyền tạo/xem/xoá**: `WriteCredentialInput.RequestingService`
  được gRPC layer resolve từ mTLS/JWT identity, **không tin từ request
  body** (`write_credential.go`'s doc comment, khớp nguyên tắc
  `ResolveCredentialInput`'s comment "never trusted from the request
  body"). `infra-fleet-service` sẽ là 1 `RequestingService` mới được cấp
  quyền gọi `WriteCredential`/`ResolveCredential`/`RevokeCredentialByOwner`
  cho category `CLOUD_PROVIDER` — cơ chế cấp quyền theo dịch vụ (không
  phải theo user) đã có tiền lệ (category `SERVICE_SECRET` dùng cho
  `notification-service` gọi credential-broker, xem F11's
  `TASK-BE-NOTIF-010`).
- **Audit log**: `AuditRepository` + `AccessAuditEntry`
  (`internal/domain/access_audit_entry.go`) đã ghi **bắt buộc, đồng bộ,
  TRƯỚC khi trả giá trị về caller** — đúng yêu cầu §8/§9 của
  `credential-broker-service.md` mà `ResolveCredential.Execute`'s doc
  comment trích dẫn. Đây chính xác là loại audit log câu hỏi 2 yêu cầu
  ("audit log riêng cho mỗi lần credential được đọc ra để dùng") — không
  cần xây lại, chỉ cần category mới ghi vào cùng bảng.
- **Revoke**: `RevokeCredential`/`RevokeCredentialByOwner` đã tồn tại.

## Câu 3 — Phạm vi (scope)

**Đề xuất: `OwnerID` = `fleet_definition_id`.**

`CredentialMetadata.OwnerID` (`domain/credential_metadata.go:117`) đã là
`string` tự do — proto's comment ghi rõ "user id or service name" nhưng
implementation không ràng buộc semantics, category khác đã dùng nó làm
"tên provider" (vd. `owner_id = "github"` cho `scm-integration-service`).
Với cloud credential, `domain.FleetDefinition` (CR-FLEET-003) đã có
`ID`/`TenantID`/`Provision *ProvisionConfig` 1:1
(`internal/domain/fleet_definition.go:37-46`) — dùng `OwnerID =
fleet_definition_id` cho phép:

- Mỗi `FleetDefinition` có credential riêng (không bắt buộc dùng chung 1
  credential cho mọi definition trong tenant — linh hoạt hơn).
- Tận dụng `ListCredentialsByCategory` sẵn có
  (`credentialbroker.proto:73-74`, "which owner_ids have a credential in
  this category for this tenant") để UI liệt kê "definition nào đã có
  credential" mà không cần thêm RPC mới.
- `RevokeCredentialByOwner` sẵn có xử lý đúng trường hợp xoá 1
  `FleetDefinition` → thu hồi credential đi kèm trong 1 lệnh gọi, không
  cần logic dọn dẹp riêng ở `infra-fleet-service`.

**Đánh đổi đã cân nhắc**: 1 credential per-tenant (dùng chung mọi
definition) đơn giản hơn nhưng không cho phép mỗi definition dùng 1 cloud
account/role khác nhau — MVP hiện tại (`DefaultVaultSSHRole` cũng là 1
giá trị per-definition, không per-tenant, xem `ProvisionConfig`'s comment)
đã có tiền lệ chọn per-definition, nên đề xuất theo cùng hướng cho nhất
quán.

## Câu 4 — Truyền credential tới agent

**Đề xuất: tái dùng đúng pattern "sent BY VALUE, exactly once" đã có,
không cần kênh mới.**

`internal/usecase/ports.go:369-407` đã đặc tả rõ pattern này cho 2 trường
hợp tương tự (`DialHiddenSshTarget`'s `target` credential fields,
`ReadCredentialFile`'s response bytes): giá trị secret gửi qua **đúng
kênh RPC agent<->Orca đã có** (cùng cơ chế `vm.exec` dùng), không persist
phía agent, không log (`agent-ephemeral-vm-handler.ts`'s "never log
contentPEM" — cần áp dụng comment tương đương phía agent cho
credential cloud provider).

Áp dụng cho `terraform apply` (TASK-BE-FLEET-006/007): `infra-fleet-service`
gọi `credential-broker-service.ResolveCredential` lấy giá trị decrypt tại
runtime (không cache), truyền vào agent RPC `terraform.apply`
(TASK-BE-FLEET-007) như 1 tham số bổ sung (map biến môi trường), agent set
biến môi trường tiến trình con `terraform apply` rồi **không giữ lại**
sau khi tiến trình kết thúc — cùng nguyên tắc "process env var, process
lifetime only" các credential khác trong hệ thống đã tuân theo.

**Xác nhận pattern áp dụng được nguyên trạng**: có — không cần điều
chỉnh, vì bản chất truyền tải giống hệt (1 secret string, gửi 1 lần, dùng
ngay, không persist).

## Câu 5 — Threat model tối thiểu

| # | Kịch bản | Biện pháp đã có sẵn hoặc đề xuất |
|---|---|---|
| (a) | Credential bị lộ qua log agent | Áp dụng cùng convention "never log" đã ghi cho `contentPEM` — cần review code thật của agent's `terraform.apply` handler khi implement, xác nhận không `slog`/print biến môi trường chứa secret. |
| (b) | Credential bị đọc bởi 1 tenant khác do lỗi tenant-scoping | `CredentialMetadata.TenantID` + `ResolveCredential` filter theo tenant đã có sẵn (dùng chung cơ chế 5 category kia đã production-tested) — rủi ro không cao hơn category hiện có. |
| (c) | Credential còn sống sau khi `terraform apply` xong (biến môi trường tiến trình con còn tồn tại) | Biến môi trường chỉ tồn tại trong vòng đời tiến trình con `terraform apply` do agent spawn — kết thúc tiến trình giải phóng theo OS. Rủi ro còn lại: agent process tự nó (process cha) có thể giữ giá trị trong memory lâu hơn cần thiết nếu không zero-hoá sau khi dùng — cần review code thật khi implement (đề xuất: không lưu secret vào biến Go sống lâu hơn phạm vi hàm gọi `exec.Command`). |
| (d) | Credential bị commit nhầm vào `varsFile`/`workingDir` nếu là thư mục git-tracked | `ProvisionConfig.WorkingDir`/`VarsFile` (đã có) không được chứa secret trực tiếp — secret chỉ đi qua biến môi trường (câu 4), không qua file. Cần 1 test/lint xác nhận `varsFile` sinh ra không nội suy giá trị credential vào nội dung file khi implement. |
| (e) *(bổ sung, phát hiện khi viết tài liệu này)* | Credential rò rỉ qua `AccessAuditEntry`'s `target`/log fields nếu vô tình ghi giá trị thay vì chỉ `credential_id` | `Append`'s chữ ký hiện tại (`auditclient.Client.Append`) chỉ nhận string field rời (`action`, `target`...), không có tham số "value" — thiết kế hiện tại đã tự nhiên ngăn việc này, nhưng cần code review xác nhận không ai truyền nhầm giá trị secret vào `target`. |

## Câu 6 — Ai review

**Cố ý để ngỏ.** Không có quy trình security-review chính thức nào được
xác nhận tồn tại trong repo ở thời điểm viết tài liệu này (không tìm thấy
`SECURITY.md`/`docs/security-review-process.md` hay tương đương qua khảo
sát). Người giao việc/Architecture Owner cần xác định: (a) ai/role chịu
trách nhiệm ký duyệt tài liệu này, (b) hình thức sign-off (comment PR,
1 dòng ghi trong chính file này, hay quy trình khác).

## Kết luận — điều kiện để mở TASK-BE-FLEET-01X (code)

Tài liệu này trả lời đủ 5/6 câu hỏi bằng cách tái sử dụng hạ tầng đã có
(không phát minh cơ chế mới) — giảm đáng kể bề mặt rủi ro so với đánh giá
"Cao" ban đầu của CR-FLEET-002, vì đây không còn là "bề mặt bảo mật hoàn
toàn mới" mà là "mở rộng 1 category cho hệ thống credential đã
production-tested". Câu 6 vẫn cần người có thẩm quyền trả lời trước khi
bất kỳ task code nào (`TASK-BE-FLEET-01X`, chưa tồn tại) được tạo — task
code đó khi viết cần trích dẫn đúng file này, không thiết kế lại từ đầu.

## Liên quan

- [TASK-BE-FLEET-009](../tasks/TASK-BE-FLEET-009-cloud-credential-storage-design-security-review.md)
- `backend-go/services/credential-broker-service/` — toàn bộ hệ thống được đề xuất tái sử dụng
- `backend-go/proto/orca/credentialbroker/v1/credentialbroker.proto` — `CredentialCategory` enum (thêm giá trị mới ở đây khi implement)
- `backend-go/services/infra-fleet-service/internal/usecase/ports.go:369-407` — pattern "sent BY VALUE, exactly once" tái dùng cho câu 4
- `backend-go/services/infra-fleet-service/internal/domain/fleet_definition.go` — `ProvisionConfig`, cơ sở cho câu 3
