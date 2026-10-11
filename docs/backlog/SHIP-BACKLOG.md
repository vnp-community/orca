# SHIP-BACKLOG — việc cần làm để giao Orca cho khách hàng dùng hằng ngày

**Ngày rà soát:** 2026-10-11 · **Nhánh:** `feat/e2e-acceptance-fixes` · **Người rà soát:** Claude (đối chiếu docs, specs, code, báo cáo e2e)

Tài liệu này khác [`README.md`](./README.md) (22 mục `BACKLOG-XXX`: các việc bị chặn bởi quyết định hoặc phụ thuộc chưa xây). Đây là danh sách **toàn bộ việc cần làm để có một hệ thống hoàn chỉnh, dùng được mỗi ngày**: tính theo khoảng cách từ trạng thái hiện tại đến "khách hàng cài đặt, đăng nhập, nhận Request, agent làm xong, review và merge, không cần dev Orca đứng cạnh".

> **Cách đọc mức ưu tiên.** **P0** = chặn giao cho khách (không có thì không ship). **P1** = cần có để khách dùng liên tục sau pilot (làm trong 1 đến 2 sprint sau GA hoặc song song). **P2** = làm được sau GA. Cỡ **S** ≤ 3 ngày, **M** ≈ 1 tuần, **L** ≈ 2 đến 4 tuần, **XL** > 1 tháng. Cỡ là ước lượng thô theo mức độ phức tạp thấy trong code, **chưa phải cam kết**.

---

## 0. Hiện trạng một trang (bằng chứng, không phải kỳ vọng)

| Mảng | Trạng thái thật | Nguồn |
|---|---|---|
| Request → Solution → Plan → Phase → Task (v6) | **Chạy được end-to-end trên stack local** với Claude Code thật qua giao diện web: `run_ui_flow.py --real all` 6/6 xanh; e2e backend `--ai-stub` 114 PASS / 0 FAIL / 2 BLOCKED (MCP tắt); Claude thật 103 PASS / 0 FAIL / 4 BLOCKED | `tests/ORCA-E2E-ACCEPTANCE-REPORT.md` §9–10, `specs/backend-go/bugs/e2e-acceptance/README.md` §5 |
| E2E trên repo thật của người dùng (`deepseek-harness`) | Add Project, từ chối trùng, câu hỏi chỉ-đọc, review Solution, hotfix, Phased change_request có agent viết code + test, một task | commit `3e245c6c2` |
| Backend v6 (spec) | 140 / 199 task đánh dấu xong, **59 còn `[ ]`**; nhiều task `[ ]` là **trạng thái cũ** (vd. Plan/GeneratePlan đã chạy thật nhưng task 012-05 vẫn `[ ]`) → số thật chưa biết | `specs/backend-go/crs/v6` |
| Frontend v6 | 53 / 54 xong, 1 BLOCKED (canvas lens tuỳ chọn) | `specs/frontend/crs/v6` |
| Code Intelligence v7 | agent 144 ✔, frontend 178 ✔, backend 272 ✔ / **25 `[ ]`** (kênh `view/state/subscribe/quality` ở gateway 040-12..30 và tool MCP 041). Chưa có e2e UI thật (`code-intel-web` chưa chạy) | `specs/*/crs/v7` |
| Workflow Engine v2 (CR-WFD-001..011) | **Chỉ là đề xuất** (spec ghi "chưa triển khai"); đã có bộ test nghiệm thu đỏ-trước ở `tests/backend`, `tests/client`, `tests/e2e/workflow-web` (chưa commit). Workflow v1 vẫn chạy (13/13 PASS suite `workflow`) | `specs/backend-go/crs/v7/workflow-engine` |
| Triển khai production | `deploy/prod/docker-compose.yml` **chỉ có `orca-server` (Node)**, không có service backend-go; `deploy/dev` có 48 service. README triển khai **chưa dựng thử từ máy sạch**. Vault dev lưu trong bộ nhớ | `deploy/`, handoff §3, §5 |
| CI | 20+ workflow backend-go + e2e, nhưng `orca-flows-e2e.yml`, `backend-go-request-contracts.yml` **chưa chạy trên runner GitHub**; lint/vitest toàn frontend đỏ do lỗi có sẵn trên HEAD | handoff §5.4 |
| Phân quyền | Mới thử bằng **tài khoản admin**; chưa thử người dùng thường, thành viên project, người duyệt ≠ người tạo | báo cáo e2e §8 |
| Nguồn Request ngoài (Jira, GitHub, webhook, MCP) | **Chưa kiểm thử** (E17, E18 của CR-REQ-025); MCP đang tắt ở dev | báo cáo e2e §8 |
| MySQL | Bộ e2e chỉ chạy Postgres | báo cáo e2e §8 |
| Tài liệu trạng thái | `docs/roadmap/*` đề ngày 09-08 / 09-12, **chưa phản ánh v6, v7, e2e-acceptance** | `docs/roadmap/README.md` |

**Kết luận ngắn:** lõi sản phẩm (nhận Request → AI phân tích → duyệt → agent thực thi → xem kết quả) đã chứng minh được trên máy dev. Khoảng cách tới khách hàng nằm ở **bốn nhóm**: (1) quyết định sản phẩm và bảo mật chưa chốt, (2) đóng gói và vận hành production chưa có, (3) vòng lặp git hằng ngày còn vài lỗi hợp đồng, (4) chưa kiểm thử nhiều người dùng, nguồn ngoài, MySQL, tải.

---

## 1. Quyết định cần chốt trước (chặn việc phía sau)

Mỗi quyết định ghi **ai quyết**, **hệ quả nếu để trống**. Phần lớn đã nằm trong `BACKLOG-00x` hoặc handoff nhưng chưa ai chốt.

| ID | Quyết định | Chủ | P | Để trống thì |
|---|---|---|---|---|
| SHIP-D-01 | ✅ **ĐÃ CHỐT 2026-10-11 — bề mặt giao khách ở v1 là web app + Dev Server Agent.** Desktop Electron, mobile companion có ưu tiên thấp hơn, làm sau GA (SHIP-CLI-01, 02). Hệ quả: desktop và backend Node/SQLite (BACKLOG-014) không còn là điều kiện ship | Product | ~~P0~~ xong | — |
| SHIP-D-02 | **Chính sách thực thi agent**: task luôn chạy `--dangerously-skip-permissions`, `trustPreset:"full"`, không sandbox (B4). Chọn: chính sách theo project, sandbox riêng (container/VM), hay chấp nhận có điều kiện | Security + Product | P0 | Khách doanh nghiệp từ chối; rủi ro agent ghi/xoá ngoài repo, lộ secret của dev server |
| SHIP-D-03 | **Mô hình triển khai**: SaaS đa tenant của VNPAY, on-prem một tenant, hay cả hai. Quyết định số DB hỗ trợ ở GA (Postgres only hay + MySQL/TiDB) | Product + Ops | P0 | Không biết cách ly tenant cần chứng minh tới mức nào, không biết matrix kiểm thử |
| SHIP-D-04 | **Nguồn AI ở GA**: phiên Claude Code đã đăng nhập trên dev server (đang dùng ở e2e) hay khoá API; nhà cung cấp hỗ trợ chính thức; ai trả tiền, hạn mức | Product | P0 | Chưa có kịch bản thương mại; phụ thuộc vào cơ chế đăng nhập CLI của từng dev server |
| SHIP-D-05 | **Phiên agent: hướng A (PTY, theo TDD C4.9) hay B (giữ `execPrompt`, hiển thị tương đương)** — đang chạy hướng B | Tech lead | P1 | Tài liệu thiết kế và code lệch nhau; ảnh hưởng tương tác thời gian thực (gõ vào phiên agent) |
| SHIP-D-06 | **Phạm vi v6/v7 bản đầu**: OpenSpec engine (026), context-sources (031), impact-risk (030), quality gate chặn cứng — vào GA hay sau | Product | P1 | 40+ task còn `[ ]` không rõ có phải điều kiện ship |
| SHIP-D-07 | **Workflow Engine v2 có vào GA không** (11 CR, 5 wave, ~L đến XL) | Product | P1 | Đang có bộ test đỏ-trước nhưng không có kế hoạch giao |
| SHIP-D-08 | **Telemetry/riêng tư** (BACKLOG-005): có thu thập không, định danh, đồng ý | Product + Pháp lý | P1 | Hiện `telemetry.track` là no-op; không có số liệu sử dụng khi pilot |
| SHIP-D-09 | **Giấy phép và pháp lý**: README ghi MIT; xác nhận mô hình thương mại, quét giấy phép phụ thuộc, điều khoản/DPA | Pháp lý | P0 | Không giao khách được |
| SHIP-D-10 | **Cờ mặc định**: `request_flow_enabled`, `code_intel_enabled`, `quality_gate_enabled`, `REQUEST_APPROVAL_ENABLED` hiện mặc định **tắt**; chốt giá trị cho từng loại môi trường | Product + Ops | P0 | Khách cài xong, không thấy tính năng nào |

---

## 2. Epic OPS — Đóng gói, triển khai, vận hành production

**Mục tiêu:** một người ops không biết code Orca có thể dựng, nâng cấp, sao lưu, theo dõi hệ thống.

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-OPS-01 | **Bộ triển khai production cho backend-go** (compose hoặc Helm): ~17 service + Postgres + NATS + Vault + gateway + frontend tĩnh; hiện `deploy/prod` chỉ có `orca-server` | Dựng từ máy sạch bằng một lệnh theo README; smoke test `PLT-*` xanh | L | P0 |
| SHIP-OPS-02 | **Vault chế độ production**: lưu trữ bền, unseal/auto-unseal, sao lưu, chính sách tối thiểu cho từng service (đang dùng dev mode, mất khi khởi động lại) | Khởi động lại Vault không mất secret; diễn tập khôi phục | M | P0 |
| SHIP-OPS-03 | **Quản lý secret và token nội bộ**: sinh, xoay, phân phối (`REQUEST_INTERNAL_CALLER_TOKEN`, `PROJECT_INTERNAL_CALLER_TOKEN`, `AUTH_DIRECTORY_CALLER_TOKEN`, …); token rỗng phải fail | Không còn `ensure-*-env.sh` thủ công; có kiểm tra `/readyz` cho auth/mcp/project | M | P0 |
| SHIP-OPS-04 | **Sao lưu và khôi phục DB** (Postgres từng schema, MySQL nếu hỗ trợ), kèm quy trình migration tiến và lùi, thứ tự nâng cấp service | Diễn tập khôi phục từ bản sao lưu vào môi trường trống, số liệu RPO/RTO ghi lại | M | P0 |
| SHIP-OPS-05 | **NATS JetStream bền**: stream, retention, replay outbox; cảnh báo outbox lag | Mất một node không mất sự kiện; `orca_request_outbox_pending` có cảnh báo | S | P0 |
| SHIP-OPS-06 | **Quan sát**: nạp `request.rules.yaml` vào Prometheus (hiện không có nơi nạp), dashboard, gom log, xuất trace OTel; hiệu chỉnh ngưỡng | Ít nhất 6 cảnh báo bắn thật khi mô phỏng sự cố; dashboard sức khoẻ theo service | M | P0 |
| SHIP-OPS-07 | **CI xanh và chạy trên runner**: chạy `orca-flows-e2e.yml`, `backend-go-request-contracts.yml`; sửa lint/vitest đỏ sẵn trên HEAD; tách `ai-complete-handler.ts`, `agent-capability-report.ts` dưới 300 dòng (không tắt `max-lines`); bỏ `--no-verify` | PR không thể merge khi đỏ; pre-commit chạy không SIGKILL | M | P0 |
| SHIP-OPS-08 | **E2E dùng AI thật trong CI** (nightly): cần tài khoản Claude trên runner hoặc dev server cố định; giữ chế độ `--ai-stub` cho PR | Nightly xanh 7 ngày liên tục | M | P1 |
| SHIP-OPS-09 | **Tải và dung lượng**: hiệu chỉnh giới hạn tốc độ (5 ghi/s, burst 10 mới là đề xuất), thử N người dùng đồng thời, nhiều worktree, 50 đến 500 Request, `usePlanTree` tải cả project (`task.list` không lọc theo Request) | Có số đo, bảng sizing, giới hạn đã chỉnh; không còn `REQUEST_RATE_LIMITED` ở tải bình thường | L | P0 |
| SHIP-OPS-10 | **Diễn tập rollback cờ và nâng cấp** (mục "chưa diễn tập" ở runbook là điều kiện vào Beta) | Biên bản diễn tập: tắt cờ, quay lại bản trước, dữ liệu nguyên vẹn | S | P0 |
| SHIP-OPS-11 | **Cài Dev Server Agent**: gói cài một lệnh cho Linux/macOS (Windows nếu dev server của khách có Windows; Dev Server Agent là thành phần bắt buộc của bản web nên đây là P0), tự nối lại, nâng cấp agent, chạy dưới systemd/launchd, chặn `git push` mặc định có tuỳ chọn | Cài trên máy sạch < 10 phút; khớp onboarding F28 | M | P0 |
| SHIP-OPS-12 | **Phát hành**: versioning thống nhất, image đã ký/đẩy registry, changelog, kênh cập nhật (desktop `release-*`; server image) | Một lần "cut release" tạo đủ artifact | M | P1 |
| SHIP-OPS-13 | **Dữ liệu thử và dọn dẹp môi trường**: xoá ~47 template `e2e-*`, Request mồ côi, 4 project `probe-proj`, 2 dev server `*-stub-*`; công cụ dọn có kiểm soát | Môi trường staging sạch, script idempotent | S | P1 |
| SHIP-OPS-14 | **Chẩn đoán/hỗ trợ**: xuất gói chẩn đoán (phiên bản, cấu hình ẩn secret, log gần nhất, trạng thái service), trang "Về" có version | Nhân viên hỗ trợ lấy được gói trong 1 thao tác | S | P1 |

---

## 3. Epic SEC — Bảo mật và tuân thủ

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-SEC-01 | **Cài đặt chính sách/sandbox thực thi agent** theo quyết định D-02 (thay `trustPreset:"full"` cố định): theo project, danh sách công cụ cho phép, giới hạn đường dẫn, mạng | Agent không ghi ra ngoài worktree; có test; chính sách hiển thị trên UI | L | P0 |
| SHIP-SEC-02 | **Lưu cloud credential** (BACKLOG-019): tìm người review, sign-off, rồi cài. Cần cho Fleet Terraform | Có chữ ký review; credential không vào log/git; xoá sau `apply` | M | P1 |
| SHIP-SEC-03 | **Danh tính xuyên service**: `task-service` còn tin metadata `x-user-id`; chuyển sang danh tính ký (ADR-015) hoặc kiểm ở biên; mọi service dùng `grpcmw.NewClient` | Gọi giả mạo `x-user-id` bị từ chối; `check-grpc-dials.sh` xanh | L | P0 |
| SHIP-SEC-04 | **RLS thật ở mọi service có `tenant_id`** (task-service chưa đặt `app.tenant_id`); hoàn tất 035-02 (actor_type) | Test đọc chéo tenant cho mọi bảng, cả Postgres và MySQL | L | P0 |
| SHIP-SEC-05 | **Che secret và chuỗi cung ứng** (TASK-REQ-035-09): redaction ở log/output agent, quét phụ thuộc, SBOM | Quét có báo cáo; không lộ secret trong e2e `live-secret-leak-scan` | M | P0 |
| SHIP-SEC-06 | **Vòng đời grant**: grant `user` do request-service tự cấp dưới danh nghĩa chủ cây không tự thu hồi khi Request xong; audit ghi sai `granted_by` | Request xong → grant tạm hết hiệu lực; audit ghi đúng chủ thể | M | P0 |
| SHIP-SEC-07 | **Bộ test phân quyền nhiều người dùng**: người dùng thường, thành viên project, người duyệt ≠ người tạo, tự duyệt, khác tenant | ≥ 30 ca tự động, cả đường REST và WS | M | P0 |
| SHIP-SEC-08 | **Rà soát bảo mật bên thứ ba/pentest**: OWASP, session/cookie, CSRF, WS đã xác thực, rate limit đăng nhập, header, SSO OIDC/GitHub e2e (SAML nếu khách yêu cầu: CR-RBAC-007) | Báo cáo, mọi lỗi High/Critical đã sửa | L | P0 |
| SHIP-SEC-09 | **Audit log đủ và giữ lại**: phủ mọi hành động quản trị và duyệt, thời hạn lưu, xuất | Một câu hỏi "ai làm gì lúc nào" trả lời được từ audit | M | P1 |
| SHIP-SEC-10 | **Share link của task**: `GenerateShareLink`/`GetTaskByShareToken` chưa expose, chờ review (BACKLOG-025) | Review xong, hoặc gỡ code | S | P1 |
| SHIP-SEC-11 | **Quyền cấp Request** (CR-035 §2.3 chưa duyệt; một số task dùng quy tắc tạm `reporter_id`) | Ma trận quyền được duyệt và cài | M | P0 |
| SHIP-SEC-12 | **Quyền riêng tư và xoá/xuất dữ liệu** (`EraseRequest`, `ExportRequest`, lưu giữ) | Có quy trình đáp ứng yêu cầu xoá/xuất của khách | M | P1 |

---

## 4. Epic CORE — Hoàn thiện luồng Request hằng ngày

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-CORE-01 | **Đối soát trạng thái 59 task v6 `[ ]` và 25 task v7 `[ ]` với code**; đóng task đã làm, ghi rõ phần còn lại. Nhiều task Plan/Phase đã chạy thật nhưng `[ ]` | Số thật được ghi lại ở mỗi README; không còn trạng thái cũ | S | P0 |
| SHIP-CORE-02 | **Tiếp nhận từ nguồn ngoài**: Jira (đồng bộ hai chiều), GitHub issue/PR, webhook, MCP (`MCP_ENABLED`), có idempotency; kịch bản E17/E18 | e2e có Jira/GitHub giả lập và, nếu có, thật; không Request trùng | L | P0 |
| SHIP-CORE-03 | **Vòng đời Clarification** hoàn chỉnh (028-06 resume consumer, hết hạn, nhắc; 028-07 Decision record); gợi ý định dạng trường `time_box_hours` | Request chờ trả lời sống qua khởi động lại; hết hạn xử lý đúng | M | P0 |
| SHIP-CORE-04 | **Chính sách theo loại** (014-03..06): `pre_deploy` cho hotfix/security, performance, refactor, ops runbook; e2e riêng cho hotfix/security/performance | Mỗi loại có đường đi và cổng đúng bảng 3.4 | M | P0 |
| SHIP-CORE-05 | **Hợp đồng thực thi** (029-04..08): TaskSpec v2, ReadinessGate, xác minh kết quả, phân loại lỗi | Task chỉ chạy khi sẵn sàng; lỗi phân loại rõ (do agent / môi trường / người) | L | P0 |
| SHIP-CORE-06 | **Quản trị AI** (034): ngân sách theo tenant/project, phiên bản prompt, kiểm chứng căn cứ, egress, chế độ bóng | Vượt ngân sách thì dừng có thông báo; có số chi phí theo Request | L | P0 |
| SHIP-CORE-07 | **Cổng duyệt**: nối `ApprovalGates.CheckPlan` (`plan_assumption`), quản trị chính sách người duyệt, thông báo (app, email/push) tới người duyệt, hết hạn/gia hạn | Người duyệt nhận thông báo và duyệt được từ hộp duyệt | M | P0 |
| SHIP-CORE-08 | **Thực thi bền vững**: agent chết giữa chừng, dev server rớt, huỷ task đang chạy, khôi phục lease, chạy lại từ bước lỗi, nhiều phase song song | e2e có chaos: kill agent/restart service mà không mất hoặc lặp tác dụng phụ | L | P0 |
| SHIP-CORE-09 | **Kiểm tra tồn tại project** (D9 còn dư: `project.get` trả `Internal` thay `NotFound`, cần RPC existence ở project-service) và thông báo lỗi người dùng cho mọi mã `REQUEST_*`/`APPROVAL_*` | Không còn lỗi `Internal` cho đầu vào người dùng sai | S | P1 |
| SHIP-CORE-10 | **Sửa test chập chờn** `RQL-13`/`RQL-19` (`REQUEST_VERSION_CONFLICT`, nghi hai lượt phân loại chồng nhau) và điều kiện đua mất chunk đầu `· Session started` | 20 lượt liên tiếp xanh | S | P1 |
| SHIP-CORE-11 | **Phân tích tác động và rủi ro** (030), **nguồn ngữ cảnh** (031), **OpenSpec** (026) — chỉ nếu D-06 đưa vào GA | Theo từng CR | XL | P2 |

---

## 5. Epic GIT — Vòng lặp code hằng ngày (diff, commit, PR, worktree)

Đây là bề mặt mà người dùng chạm mỗi giờ. Handoff ghi các lỗi hợp đồng đã biết, **chưa sửa**.

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-GIT-01 | **Tạo PR/MR hỏng** (G5b): frontend gửi `branch/base/repo`, gateway đọc `headBranch/baseBranch` → `SCM_EMPTY_BRANCH` | Tạo PR từ worktree thật trên GitHub và GitLab qua UI | S | P0 |
| SHIP-GIT-02 | **Git panel sai kiểu dữ liệu** (G5d): `git.upstreamStatus/diff/compare/history` trả snake_case, frontend cần camelCase | Test hợp đồng cho mọi kênh `git.*`; UI hiện đúng | M | P0 |
| SHIP-GIT-03 | **Diff rỗng và commit thiếu SHA** (G5e): `RelayExecutor.GetDiff` rỗng, `Commit` không trả SHA | Xem diff của agent, commit, thấy SHA trên UI (có test trên dev server thật) | M | P0 |
| SHIP-GIT-04 | **Nhập repo lồng nhau**: handler `fs.scanNestedRepos` ở agent, `project.importNested` thêm owner cho người nhập, kênh `orcaProfiles.findProjectProfiles` | Nhập thư mục chứa nhiều repo thành công | M | P1 |
| SHIP-GIT-05 | **Dữ liệu repo cũ không gắn dev server** (BACKLOG-012): chạy rebind trên dữ liệu thật, bỏ hẳn dead code multi-host (TODO §3) | Không còn `GITGATEWAY_REPO_HOST_UNBOUND` trên staging | S | P0 |
| SHIP-GIT-06 | **Vòng đời worktree cho web**: `WorktreeCleanupService` (BACKLOG-023) là mã của desktop nên không dùng được cho web; cần chính sách dọn worktree xong việc ở backend-go, kiểm tra an toàn trước xoá, worktree mồ côi, hỗ trợ SSH | Chính sách dọn được chốt, cài, có cờ | M | P1 |
| SHIP-GIT-07 | **Tương thích Git** (baseline 2.25) và provider GitLab/Gitea/Bitbucket/Azure cho PR, không chỉ GitHub | Test với binary Git 2.25 và provider thứ hai | M | P1 |

---

## 6. Epic UX — Giao diện và trải nghiệm

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-UX-01 | **Lưu lịch sử hoạt động agent ở backend** (hiện mất sau tải lại) và phát lại | Tải lại trang vẫn thấy dòng `▸ Read/Edit/Bash` đã qua | M | P0 |
| SHIP-UX-02 | **Màn hình quản trị cho luồng Request**: bật cờ tenant (hiện chỉ qua script Python), chính sách người duyệt, hạn mức, AI, dev server | Admin không cần dòng lệnh | M | P0 |
| SHIP-UX-03 | **Hoàn thiện UI còn dư**: `CoordinatorRunPanel` (BACKLOG-017), `TaskGrantModal` list/revoke (025), thư viện workflow tìm kiếm (026), bảng markdown (`remark-gfm`), Radix `Description` ở Add Project | Không còn "coming soon" ở luồng chính | M | P1 |
| SHIP-UX-04 | **Hiệu năng canvas/đồ thị** với hàng trăm node, chế độ sáng/tối, màn hẹp/di động (chưa kiểm chứng) | Ngân sách hiệu năng đạt ở 500 node | M | P1 |
| SHIP-UX-05 | **Trợ năng (a11y) và i18n**: bản dịch es/ja/ko/zh cần người bản ngữ rà; `verify:localization-*` chạy được | Chạy kiểm tra a11y tự động, không lỗi nghiêm trọng | M | P1 |
| SHIP-UX-06 | **Onboarding người dùng mới cho luồng Request**: tour, trạng thái rỗng, lỗi dễ hiểu, hướng dẫn kết nối dev server và đăng nhập AI | Người mới tạo Request đầu trong ≤ 10 phút không cần hướng dẫn | M | P0 |
| SHIP-UX-07 | **Cập nhật thời gian thực đầy đủ**: sự kiện `solution.superseded`/`clarification.answered`, bỏ polling dư | Không cần F5 ở các luồng chính | M | P1 |

---

## 7. Epic REV — Xem code và kiểm soát chất lượng (v7)

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-REV-01 | **Đối soát và hoàn tất kênh gateway còn `[ ]`**: `view` (040-12..15), `state` (040-16..19), `subscribe` (040-20..22), `quality` (040-23..30), tool MCP (041-01..06). README ghi "đã hoàn thành" nhưng 25 task còn `[ ]` | Mỗi task đóng với bằng chứng | M | P1 |
| SHIP-REV-02 | **E2E UI thật cho code-intel**: suite `code-intel-web` chưa chạy trên stack thật; kiểm với index lớn (1 đến 4 GB mỗi worktree) | Xanh trên repo ≥ 100 nghìn dòng | M | P1 |
| SHIP-REV-03 | **Chiến lược index theo worktree**: overlay theo diff (CR-CV-080) và dung lượng đĩa; chính sách làm mới | Không đầy đĩa dev server khi 10 worktree | M | P1 |
| SHIP-REV-04 | **Cổng chất lượng ở chế độ chỉ báo → có đường chặn** (D-06, O9); hiệu chỉnh quy tắc, giảm báo động giả | Tỷ lệ báo động giả chấp nhận được theo đo thực tế | L | P2 |

---

## 8. Epic WF — Workflow và Automation

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-WF-01 | **Automation `connectionId`** (BACKLOG-024, **High**): mọi action trừ `create_pr` hỏng khi gửi vì không resolve `connectionId` | `run_agent/run_script/send_notification/commit_push` chạy qua UI trên dev server thật | M | P0 nếu bán Automation, còn lại P1 |
| SHIP-WF-02 | **Trigger "PR merged" → automation** (BACKLOG-020): cơ chế webhook đã có (`ReceiveWebhook`), chưa nối vào `HandleExternalTrigger`; thiết kế `trigger_depth` | Merge PR ngoài Orca kích hoạt automation đúng một lần, không vòng | M | P1 |
| SHIP-WF-03 | **Tín hiệu "agent xong"** (BACKLOG-022): quyết định dùng PTY-exit/RPC-response hay lọc `AgentDetector` | Có quyết định và cài | S | P1 |
| SHIP-WF-04 | **Workflow Engine v2 W0 đến W2** (định nghĩa có phiên bản, input/output có kiểu, retry/timeout, tạm dừng/huỷ thật, sự kiện trực tiếp) — chỉ nếu D-07 đưa vào GA; gộp `BACKLOG-021` | Cổng nghiệm thu `ORCA_WORKFLOW_WAVE=2` xanh | XL | P1 |
| SHIP-WF-05 | **Workflow Engine v2 W3 đến W5** (rẽ nhánh, cổng người, trigger, nâng cấp v1→v2) | Cổng W5 xanh | XL | P2 |
| SHIP-WF-06 | **Quyết định/dọn nhỏ**: `approval` step (BACKLOG-027), dead code `worktree-list-groups.ts` và `resolvePortableEphemeralVmProjectId` (TODO §3) | Đã chốt, đã dọn | S | P2 |

---

## 9. Epic CLI — Các client ngoài web (ưu tiên thấp hơn web + Dev Server Agent, theo D-01)

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-CLI-01 | **Desktop Electron (sau GA)**: smoke test 3 OS, quyết định chuyển sang backend-go hay giữ Node/SQLite, rà soát ~68 bug audit cũ (BACKLOG-014). Trước đó chỉ cần bảo đảm bản web không phụ thuộc mã desktop | Matrix cài đặt 3 OS; không còn finding High chưa xác minh | XL | P2 |
| SHIP-CLI-02 | **Mobile companion (sau GA)**: route `POST /api/mobile/push-subscribe` + CR mobile SSO (BACKLOG-018) | Thiết bị thật nhận push | L | P2 |
| SHIP-CLI-03 | **CLI headless**: tài liệu và kiểm thử e2e với token (nền đã có ở CR-CLI-001..003) | Chạy Request/task từ CI bằng token | S | P2 |
| SHIP-CLI-04 | **Ephemeral VM** (BACKLOG-001, 002, 028): chốt phạm vi; nếu ngoài v1 thì ẩn khỏi UI | Không có đường UI dẫn tới lỗi `INFRA_EPHEMERAL_VM_SSH_UNSUPPORTED` | S (ẩn) / XL (xây) | P2 |

---

## 10. Epic DOC — Tài liệu, hỗ trợ, thương mại

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-DOC-01 | **Hướng dẫn người dùng theo vai trò**: quickstart 15 phút; "một ngày làm việc với Orca" cho dev, tech lead, QA, DevOps; xử lý sự cố thường gặp | Người ngoài làm theo được không cần hỏi | M | P0 |
| SHIP-DOC-02 | **Hướng dẫn cài đặt và quản trị**: yêu cầu hệ thống/mạng (agent chủ động nối ra), SSO, Jira, sizing, sao lưu, nâng cấp | Ops bên khách tự cài được | M | P0 |
| SHIP-DOC-03 | **Cập nhật tài liệu trạng thái**: `docs/roadmap/*` và `feature-completion-matrix.md` chưa có v6, v7, e2e-acceptance; thêm F43+ cho Request flow, Code Intelligence, Workflow Engine v2 vào `docs/features/` | Registry khớp thực tế | S | P1 |
| SHIP-DOC-04 | **Runbook đầy đủ**: sự cố, sao lưu/khôi phục, xoay token, mất Vault, đầy đĩa index | Mỗi runbook có người diễn tập | M | P0 |
| SHIP-DOC-05 | **Chính sách hỗ trợ**: kênh, SLA, quy trình báo lỗi, phát hành vá | Có tài liệu và người trực | S | P1 |
| SHIP-DOC-06 | **Bản tiếng Anh** của tài liệu người dùng/quản trị nếu khách quốc tế (hiện hầu hết tiếng Việt) | Theo D-03 | M | P2 |

---

## 11. Epic GA — Cổng phát hành

| ID | Việc | Nghiệm thu | Cỡ | P |
|---|---|---|---|---|
| SHIP-GA-01 | **Dogfooding nội bộ**: đội Orca dùng chính Orca cho công việc thật ≥ 2 tuần trên staging production-like, có số đo (tỷ lệ Request hoàn thành, thời gian tới PR, lỗi/ngày) | Báo cáo dogfood, backlog lỗi mới được phân loại | M | P0 |
| SHIP-GA-02 | **Pilot** với 2 đến 3 khách/đội: onboarding có người đi cùng, thu phản hồi | Báo cáo pilot, quyết định go/no-go | L | P0 |
| SHIP-GA-03 | **Tiêu chí nghiệm thu cuối**: `run_acceptance.py --strict` **0 FAIL, 0 BLOCKED** với AI thật, MCP bật, Postgres và MySQL (nếu D-03), đa người dùng, Jira/GitHub/webhook; spec thật `run_ui_flow.py --real all` xanh; Electron nếu có | Biên bản nghiệm thu ký | M | P0 |
| SHIP-GA-04 | **Review bảo mật cuối và phê duyệt pháp lý** | Ký duyệt | S | P0 |
| SHIP-GA-05 | **Kế hoạch triển khai khách đầu tiên và diễn tập rollback** | Có lịch, có người trực, có đường lùi | S | P0 |

---

## 12. Lộ trình đề xuất (phụ thuộc, không phải cam kết ngày)

| Giai đoạn | Nội dung | Điều kiện ra |
|---|---|---|
| **R0 — Chốt** (1 đến 2 tuần) | D-02 đến D-04, D-09, D-10 (D-01 đã chốt); SHIP-CORE-01; SHIP-OPS-07 (CI xanh) | Quyết định ghi vào ADR; CI xanh |
| **R1 — Staging production-like** (4 đến 6 tuần) | OPS-01..06, 10, 11; SEC-01, 03..07, 11; CORE-02..08; GIT-01..03, 05; UX-01, 02, 06; DOC-01, 02, 04 | Staging dựng từ máy sạch; e2e đa người dùng xanh; không lỗi Git panel |
| **R2 — Dogfood + Pilot** (4 tuần) | GA-01, 02; OPS-08, 09, 13, 14; SEC-08, 09; UX-03..05, 07; WF-01..03; REV-01..03 | Báo cáo pilot go; 0 lỗi P0 mở |
| **R3 — GA** | GA-03..05; DOC-05; SEC-12 | Biên bản nghiệm thu ký |
| **R4 — Sau GA** | **Desktop (CLI-01) và mobile (CLI-02)**; WF-04, 05; REV-04; CORE-11; CLI-03, 04; DOC-06 | Theo nhu cầu khách |

Điểm hay chạm giữa các nhóm cần xếp thứ tự: **SHIP-SEC-03** (danh tính) và **SHIP-SEC-04** (RLS) đổi hành vi nhiều service nên làm trước **SHIP-OPS-09** (đo tải) để số đo không bị làm lại; **SHIP-CORE-08** (thực thi bền vững) nên xong trước dogfood.

---

## 13. Ánh xạ 22 mục `BACKLOG-XXX` cũ sang backlog ship

| BACKLOG | Trạng thái 2026-10-11 | Vào backlog ship |
|---|---|---|
| 001 | Open | SHIP-CLI-04 |
| 002 | Unblocked, cần xác minh | SHIP-CLI-04 |
| 004 | Open (agent chưa có handler) | không chặn GA; P2 |
| 005 | Open (chờ quyết định) | SHIP-D-08 |
| 009 | **Resolved (code)** | SHIP-CORE-08 (e2e circuit breaker) |
| 012 | **Resolved (code)**, còn dữ liệu cũ | SHIP-GIT-05 |
| 013 | Unblocked | SHIP-UX-03 |
| 014 | Open, **hạ ưu tiên** (D-01: desktop sau GA) | SHIP-CLI-01 (P2) |
| 015 | **Resolved** (còn `removeEdge`) | — |
| 017 | Narrowed (chỉ còn wiring + UI) | SHIP-UX-03 |
| 018 | Open, **hạ ưu tiên** (mobile sau GA) | SHIP-CLI-02 (P2) |
| 019 | Open (chờ sign-off) | SHIP-SEC-02 |
| 020 | Narrowed (đã có webhook) | SHIP-WF-02 |
| 021 | Partial | SHIP-WF-04 |
| 022 | Open (chờ quyết định) | SHIP-WF-03 |
| 023 | Open (mã desktop; web cần cơ chế riêng) | SHIP-GIT-06 |
| 024 | Open (**High**) | SHIP-WF-01 |
| 025 | Narrowed | SHIP-UX-03, SHIP-SEC-10 |
| 026 | Partial | SHIP-UX-03 |
| 027 | Open (quyết định) | SHIP-WF-06 |
| 028 | Open (quyết định) | SHIP-CLI-04 |
| 029 | Open (quyết định) | không chặn GA |

---

## 14. Giới hạn của lần rà soát này

- Trạng thái "Resolved" ở trên được suy ra từ **sự tồn tại của code** (symbol, route, wiring trong `main.go`) tra bằng CodeGraph và đọc file; **chưa chạy test hay e2e** cho từng mục. Cần xác nhận hành vi trước khi xoá file khỏi thư mục này (theo quy ước README).
- Số task v6/v7 lấy từ dấu `Status:` trong file task; nhiều dấu đã cũ so với code, nên "59 còn lại" là **cận trên**.
- Không truy cập môi trường production thật (`b15.openledger.vn`); mọi nhận định về prod dựa trên file trong repo và ghi chú handoff.
- Cỡ công việc là ước lượng thô theo đọc code, chưa qua planning với đội.
- `gitnexus` không có trong phiên này, nên chưa có phân tích blast radius theo quy ước AGENTS.md; không có thay đổi mã nào được thực hiện, chỉ sửa tài liệu.
