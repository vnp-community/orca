# TASK-BE-FLEET-009: Thiết kế credential storage cho cloud provider — CẦN SECURITY REVIEW TRƯỚC KHI CODE

**Solution:** BE-FLEET-SOL-002 §4 | **CR:** CR-FLEET-002
**Service:** `infra-fleet-service` (dự kiến — chưa chốt)
**Depends on:** Không
**Status:** 🟡 DESIGN DRAFTED (2026-09-09) — chờ security review. Câu hỏi #1 ("Nơi lưu trữ") đã được người
giao việc chốt trực tiếp; 5/6 câu còn lại nay có đề xuất cụ thể trong
[BE-FLEET-SOL-004](../solutions/BE-FLEET-SOL-004-cloud-credential-storage-design.md) — dựa trên tái sử dụng
100% hạ tầng `credential-broker-service` đã có sẵn (không phát minh cơ chế mới). **Vẫn KHÔNG đủ điều kiện để
tạo task code tiếp theo** — tài liệu thiết kế chỉ là đề xuất, câu 6 (ai chịu trách nhiệm review) cố ý để ngỏ,
và chưa có ai thực hiện sign-off thật.

### Quyết định đã chốt (2026-09-09)

> Trả lời câu hỏi #1 ("Nơi lưu trữ") ở mục "Câu hỏi cần trả lời" bên dưới: **Vault KV engine, dùng chung 1
> Vault container/instance đã có sẵn trong hệ thống** (không dựng Vault instance riêng, không dùng bảng
> Postgres + tự viết encryption-at-rest). Điều này phù hợp với ràng buộc #1 đã biết (không tái dùng
> `domain.SshTarget.VaultSSHRole` — vẫn cần 1 KV path/mount riêng cho cloud credential, khác path SSH secrets
> engine hiện có, dùng chung instance Vault không có nghĩa dùng chung secrets engine/mount).

### Tài liệu thiết kế đầy đủ đã soạn (2026-09-09)

> [**BE-FLEET-SOL-004**](../solutions/BE-FLEET-SOL-004-cloud-credential-storage-design.md) trả lời đủ 6 câu
> hỏi của mục "Câu hỏi cần trả lời" bên dưới — 5/6 câu có đề xuất cụ thể (dựa trên tái sử dụng hạ tầng
> `credential-broker-service` đã có sẵn, không phát minh cơ chế mới), câu 6 (ai review) cố ý để ngỏ. Đây là
> **đề xuất chờ security review**, không phải quyết định đã chốt như câu 1 — task vẫn giữ trạng thái dưới
> đây cho tới khi có sign-off thật.

---

## ⚠️ Đây là task THIẾT KẾ + SECURITY REVIEW, không phải task implement trực tiếp

CR-FLEET-002 tự đánh giá rủi ro của hạng mục này là **"Cao"** — bề mặt bảo mật hoàn toàn mới so với mô hình
hiện tại của `backend-go` (chỉ cầm Vault SSH cert ngắn hạn qua Vault SSH secrets engine, không bao giờ cầm
raw credential dài hạn của bất kỳ hệ thống thứ 3 nào). **Không AI hay dev nào được tự ý chọn 1 schema lưu
trữ và code thẳng** — task này chỉ sản xuất ra: (1) 1 tài liệu thiết kế đề xuất (không phải code), (2) yêu
cầu security review rõ ràng trước khi bất kỳ task code nào (nếu có, đánh số tiếp theo sau task này, chưa tồn
tại trong bộ task hiện tại) được phép bắt đầu.

## Mục tiêu

Sản xuất 1 tài liệu thiết kế (`docs/` hoặc bổ sung vào chính CR-FLEET-002, tuỳ team quyết định nơi lưu) trả
lời đầy đủ các câu hỏi dưới đây — KHÔNG viết code Go/TS nào trong phạm vi task này.

## Ràng buộc đã biết (từ CR-FLEET-002 §4, BE-FLEET-SOL-002 §0/§4 — không được vi phạm bởi bất kỳ đề xuất nào)

1. **Không tái dùng `domain.SshTarget.VaultSSHRole`** — loại secret khác hẳn (Vault SSH secrets engine role
   vs. cloud provider IAM key/service-account JSON).
2. Secret phải đi từ nơi lưu trữ → agent (qua kênh agent<->Orca đã có) → biến môi trường tiến trình
   `terraform apply` chạy trên control host — **không được log/persist ở agent** (mirror nguyên tắc đã áp
   dụng cho `ReadCredentialFile`'s "never log contentPEM", `internal/usecase/ports.go:389`).
3. Cần threat-model riêng, không chỉ "thêm 1 field config".

## Câu hỏi cần trả lời trong tài liệu thiết kế

1. **Nơi lưu trữ**: Vault KV engine riêng (khác SSH secrets engine hiện có) hay 1 bảng Postgres mới với
   encryption-at-rest? Nếu chọn Postgres, cơ chế mã hoá nào — có tái dùng cơ chế encryption-at-rest đã có
   sẵn cho secret khác trong hệ thống không (xác nhận có/không tồn tại cơ chế đó trước khi đề xuất tự viết
   mới)?
2. **Vòng đời credential**: rotation policy, ai được cấp quyền tạo/xem/xoá, có audit log riêng cho mỗi lần
   credential được đọc ra để dùng (mirror `docs/crs/v4/team-rbac/CR-RBAC-005`'s audit log outcome nếu áp
   dụng được) không?
3. **Phạm vi (scope)**: credential scoped theo `tenant_id` hay theo `fleet_definition_id` (nếu
   [CR-FLEET-003](../../../../../../docs/crs/v4/fleet-provisioning/CR-FLEET-003-fleet-definition-persistence-yaml-export-deploy.md)
   đã tồn tại — 1 `FleetDefinition` có thể có `ProvisionConfig` riêng, có cần credential riêng theo từng
   definition không, hay 1 credential per-tenant dùng chung cho mọi definition)?
4. **Truyền credential tới agent**: dùng đúng kênh RPC agent<->Orca hiện có (như `vm.exec` dùng) theo
   "gửi giá trị đúng 1 lần, không persist agent-side" — tương tự cách `EphemeralVmSshProvisioner`'s target
   credential fields được gửi (`internal/usecase/ports.go:369-372`, "sent BY VALUE, exactly once"). Xác nhận
   pattern này áp dụng được nguyên trạng cho cloud credential hay cần điều chỉnh.
5. **Threat model tối thiểu**: liệt kê ít nhất — (a) credential bị lộ qua log agent, (b) credential bị đọc
   bởi 1 tenant khác do lỗi tenant-scoping, (c) credential còn sống sau khi `terraform apply` xong (biến môi
   trường tiến trình con còn tồn tại), (d) credential bị commit nhầm vào `varsFile`/`workingDir` nếu đó là 1
   thư mục git-tracked trên control host.
6. **Ai review**: xác định người/role chịu trách nhiệm security review tài liệu này trước khi cho phép code
   — ghi rõ tên/role thật nếu team đã có quy trình security review, hoặc ghi "cần xác định" nếu chưa có.

## Sau khi tài liệu thiết kế được duyệt

Chỉ khi tài liệu ở trên được security review chấp thuận, tạo 1 (hoặc nhiều) task code mới, đánh số tiếp theo
trong bộ này (`TASK-BE-FLEET-01X`), viết đúng theo khuôn các task khác (Files cần sửa, code Go cụ thể, test
case, verify) — task code đó phải trích dẫn tài liệu thiết kế đã duyệt, không tự thiết kế lại từ đầu.

## Verify

Không có lệnh build/test — deliverable là 1 tài liệu. "Verify" cho task này là: tài liệu tồn tại, trả lời đủ
6 câu hỏi ở trên, và có xác nhận (bằng bất kỳ hình thức nào team dùng — comment PR, sign-off ghi trong tài
liệu) rằng người/role chịu trách nhiệm bảo mật đã đọc và không phản đối hướng đề xuất.

## gitnexus

Không áp dụng — task này không sửa code, không có symbol nào để `impact()`.

## Blocking

TASK-BE-FLEET-006/007/008 (orchestration Terraform) **có thể** hoàn thành và test được ở mức unit mà không
cần credential thật (dùng fake `TerraformRunner`/mock `runRecipeCommand`) — nhưng **không được coi là sẵn
sàng chạy thật (production) cho tới khi task này hoàn tất** và credential storage thật được implement theo
tài liệu đã duyệt. Ghi rõ giới hạn này trong bất kỳ demo/rollout plan nào dùng TASK-BE-FLEET-006/007/008
trước khi task này xong.
