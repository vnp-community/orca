# Phase 5 — Gateway, MCP, OPA, bảo mật, số liệu, e2e, tài liệu, rollout (CR-OR-030 và kênh)

**Solution:** BE-OR-SOL-004 §2.2–2.6 · **Phụ thuộc:** có thể làm kênh sớm theo từng RPC; phần còn lại sau Phase 4 · **Service:** `api-gateway`, `request-service`, `tests/`

## Gateway, MCP, OPA

- [x] **BE-OR-T501** Kênh `request.review.estimate|generate|get|list|artifact|revisions|regenerate|restore|decide|approve|profile.get|profile.save` + push `request.review.changed|progress`.
  Files: `api-gateway/internal/adapter/wscompat/channels_request_review.go` (+ đăng ký ở Registry). Verify: `go test ./services/api-gateway/internal/adapter/wscompat/...`; phản hồi artifact ≤ 256 KiB.
  Kết quả (2026-10-20): 17 kênh `request.review.{estimate,generate,get,list,runs,artifact,revisions,regenerate,restore,compare,decide,approve,decisions,linkChild,profile.get,profile.save,profile.reset}` trong `wscompat/channels_request_review.go`, đăng ký ở `RegisterRequestUnaryChannels`; bản đồ kênh↔RPC (`TestRequestRPCChannelParity`) và `channel_inventory.json` đã cập nhật. Đẩy sự kiện: `request.event` với `eventType` `review.changed` (3 subject `generated|artifact_regenerated|decided`) và `review.progress` (subject `progress`), chỉ mang id và enum (test: không lọt chữ). Đã thêm đường REST `GET|PUT /v1/projects/{id}/review-drawing-profile`, `POST .../reset` và bản tenant `/v1/review-drawing-profile` cho CLI (test: URL quyết định lớp, người thường bị 403, ẩn danh 401). PASS: `go test -race ./services/api-gateway/...`. **Chênh lệch:** (1) tên đẩy là `request.event`+`eventType` (đúng mẫu CONTRACT mục 7), không phải kênh riêng `request.review.changed`; (2) có thêm `compare|linkChild|decisions|runs|revisions` so với danh sách ban đầu; (3) HEAD của gateway **không biên dịch** (thiếu ký hiệu workflowv1, có sẵn từ trước, không do phần này), nên mọi lần chạy gateway ở máy này dùng `-overlay` bỏ hai tệp `channels_workflow_step_approvals*.go`; (4) giới hạn phản hồi artifact ≤ 256 KiB đảm bảo bởi trần của kho hình và test kích thước ở Phase 3, chưa có test riêng ở tầng kênh; (5) `TestBrowserScreencastChannel_*` chập chờn dưới `-race` khi chạy cả gói, chạy riêng thì PASS (không dùng mã review).

- [x] **BE-OR-T502** Tool MCP `request_review_*` (chỉ đọc cho agent) và OPA `agent_rpcs`; ghi/sinh lại/duyệt **không** cấp cho agent.
  Verify: thử gọi ghi bằng danh tính agent → bị từ chối.
  Kết quả (2026-10-20): 5 tool MCP chỉ đọc `request_review_{get,estimate,list,runs,decisions}` trong `mcpserver/tools/pack_request_flow.go` (24 tool, 18 đọc), 10 kênh ghi/sinh/duyệt/hồ sơ nằm trong `excluded_channels.yaml` có lý do; `tools_list.golden.json` thêm đúng 5 mục (45 dòng, không xoá dòng nào). OPA: `agent_rpcs` chỉ có các RPC đọc; `TestCatalogAgentFlagMatchesRego` (duyệt toàn bộ catalog với Rego thật, vai admin đứng sau agent) và `TestRequestPolicy_AgentNeverReachesForbiddenRPCs` (thêm 9 RPC ghi của review) PASS. `request_test.rego` có thêm 9 ca nhưng **chưa chạy bằng `opa test`** vì máy không có `opa` (bản Go ở trên đánh giá cùng Rego).

- [x] **BE-OR-T503** Sự kiện outbox `orca.request.review.generated|artifact_regenerated|decided` (tên chốt theo `outbox_subjects.go`).
  Verify: consumer chịu giao lặp.
  Kết quả (2026-10-20): `orca.request.review.generated|artifact_regenerated|decided` (và `progress` cho dòng tiến trình) trong `domain/outbox_subjects.go`; `usecase/outcome_review_events.go` (`ReviewEvents`, payload chỉ id và enum, nil an toàn) phát khi Generate tạo mới, khi lượt context/vẽ kết thúc, khi quyết định và khi duyệt. 4 test: vòng đời lượt chạy, lượt lỗi chỉ phát tiến trình, quyết định/duyệt không mang ghi chú, nil không làm gì. **Giới hạn:** sự kiện được ghi vào outbox **sau** khi dữ liệu đã lưu (không cùng giao dịch), nên nếu ghi outbox lỗi thì mất một sự kiện (chỉ có log); màn hình tự đọc lại khi mở nên không mất dữ liệu. Mỗi sự kiện chỉ là tín hiệu "đọc lại" nên bên nhận chịu được giao lặp; hiện chưa có consumer nào ngoài gateway.

## Bảo mật

- [x] **BE-OR-T510** Bộ injection (AC, tên tệp, văn bản SAD/KG chứa chỉ thị, khối giả `</untrusted-input>`).
  Verify: kết quả vẫn qua lược đồ, không lộ biến môi trường, không id KG bịa, không công cụ ngoài danh sách.
  Kết quả (2026-10-20): `usecase/outcome_review_security_test.go`: lời nhắc không bị đóng sớm bởi tiêu đề/AC (`</request>`), tên tệp (`</changed_files>`) hay mô tả task (`</plan_targets>`); mọi lượt agent đều `ReadOnlyEnforced`, không `WriteProfile`, không `ProposeChangesets`, có `KnowledgeRoot` và nonce; bằng chứng KG mà agent không khai đã đọc (`SVC-99`) không được giữ. Cộng với `TestContextRun_InjectionInTheRequestChangesNothing` (biến môi trường không lọt vào lời nhắc/review). **Giới hạn:** là test với agent giả; chưa có lượt tấn công thật nào vào Claude.

- [x] **BE-OR-T511** Canary bí mật (theo `code-intel-service/internal/usecase/secret_canary.go`): canary trong AC/tệp/diff/KG giả không xuất hiện ở DB, RPC, log, audit, hình.
  Kết quả (2026-10-20): canary dạng khoá AWS, token GitHub, khối khoá RSA đặt trong tiêu đề, AC, nhãn container, `tech`, nhãn thông điệp, tiêu đề/chi tiết/trước/sau của mục: không xuất hiện ở mô hình review đã lưu, bản ghi context, phản hồi (`Latest`), log và audit; test có kiểm "văn bản cùng trường vẫn được lưu" để không đạt giả. **Giới hạn:** `code-intel-service/.../secret_canary.go` hiện là bản rỗng (`ScanSecretCanary` luôn false) nên dùng bộ quét `RedactRaw` có sẵn của request-service; canary trong SVG chưa có test riêng (kho hình kiểm bằng `domain/outcome_review_artifact_check.go`).

- [x] **BE-OR-T512** Cô lập tenant/Request: khoá cache, dedupe blob, `NOT_FOUND` không phân biệt tồn tại.
  Kết quả (2026-10-20): hợp đồng chạy trên Postgres 16 và MySQL 8 thật (có RLS): review/lượt chạy/quyết định/hồ sơ/hình/ngữ cảnh không đọc chéo tenant; khoá dedupe blob theo `(tenant, digest)`, cùng khoá ở tenant khác là hàng khác; thêm kiểm review của tenant khác trả **cùng loại lỗi, cùng mã, cùng câu chữ (trừ id)** với review không tồn tại. Cả hai dialect PASS (Postgres 74 s, MySQL 127 s cho toàn bộ gói).

- [x] **BE-OR-T513** Manifest skill lệch → `REQUEST_REVIEW_SKILL_MISSING`.
  Kết quả (2026-10-20): đã phủ bằng `TestContextRun_RefusesBeforeSpendingAnything` (skill thiếu → `REQUEST_REVIEW_SKILL_MISSING`, không gọi agent, không có hàng run) và `TestDrawRun_RefusesBeforeSpending` (skill chưa cài, không báo skill, **bản khác bản ghim** → `SKILL_MISSING`). **Giới hạn:** `PinnedDiagramDesignSHA` đang rỗng vì skill chưa được vendor (T301–T304), nên trên thực tế chưa có bản "lệch" nào bị chặn.

- [x] **BE-OR-T514** Ma trận trạng thái × pha (trước `awaiting_plan_approval` → `NOT_READY`; `executing` → chỉ `design`; xong → `delivered` mới, `design` `superseded`, tái dùng `design.*`/hình).
  Verify: đếm lời gọi `kg_*` và lượt vẽ.
  Kết quả (2026-10-20): `TestReviewStateByPhaseMatrix` chạy cả 12 trạng thái Request: 9 trạng thái chưa có Plan → `REQUEST_REVIEW_NOT_READY`, không gọi agent, không có run, không có review; `awaiting_plan_approval` và `executing` → `design`; `completed` → `delivered`. `..._DeliveredRetiresDesignWithoutAskingTheGraph`: pha delivered thay review design (1 delivered, 0 design còn sống, 1 superseded) và lượt agent thứ hai **không** nhận `KnowledgeRoot` (0 lời gọi `kg_*`).

## Số liệu và log

- [x] **BE-OR-T520** Chỉ số (SOL-004 §2.4) gồm `review_design_reviewed_ratio`; log `audit=true`.
  Verify: `review_cache_hit_ratio` tính được cho Request pilot. Chưa đặt ngưỡng số.
  Kết quả (2026-10-20): port `ReviewObserver` + 10 chuỗi Prometheus mới (`request_review_context_lookups_total`, `_run_duration_seconds`, `_tokens_total`, `_cost_estimate_error_ratio`, `_diagrams_total`, `_draw_failures_total`, `_regenerations_total`, `_time_to_first_facts_seconds`, `_agent_items_total`, `_design_reviewed_ratio`) cùng 3 chuỗi hình đã có; nhãn thuộc tập đóng (test: giá trị lạ → `other`, chữ bí mật không thành nhãn). Log `audit=true` kèm `inputs_digest`, không nội dung, cho sinh, sinh lại, khôi phục, quyết định, duyệt, context. Sai số chi phí = |thực − dự đoán| / dự đoán, dự đoán là chi phí của lượt thành công gần nhất cùng loại của review đó. **Chưa làm / chưa đo:** `review_cache_hit_ratio` tính được từ các chuỗi lookups nhưng **chưa có số liệu Request pilot**; chưa đặt ngưỡng nào; `review_tokens` chỉ có khi agent báo; hạn mức chưa nhập vào `ai_budget.go`.

## e2e

- [~] **BE-OR-T530** `tests/stages/e2e/orca-flows/suite_outcome_review.py` đăng ký ở `run_acceptance.py`: Estimate → Generate → lần hai trúng cache → sinh lại → quyết định → duyệt; cuối lượt "open Requests … 0 orphaned".
  Verify: `python3 tests/stages/e2e/orca-flows/run_acceptance.py outcome-review`.
  Kết quả (2026-10-20): **viết nhưng chưa chạy được.** `tests/stages/e2e/orca-flows/suite_outcome_review.py` (ORV-01..09 miễn phí với `--ai-stub`, ORV-20 tốn tiền, chỉ khi `ORCA_E2E_REVIEW_REAL=1`) đã đăng ký ở `run_acceptance.py` (`--list` thấy `outcome_review`), có mục trong `README.md`, `py_compile` PASS. Không chạy được vì stack local đang chạy là bản cũ (không có `request.review.*`) và gateway ở HEAD không biên dịch nên không dựng lại được; suite tự báo BLOCKED khi kênh chưa có. Hệ quả: ORV-01..09 **chưa được chứng minh trên stack thật** và "0 orphaned" cuối lượt chưa kiểm.

- [~] **BE-OR-T531** Lượt thật (tốn tiền): chỉ khi người dùng đồng ý; chạy `workspace_probe.py` trước; tuân thủ quiet-checkout 90 giây; đặt đúng engine dự án cho fixture; ghi chi phí và thời gian.
  Kết quả (2026-10-20): **chưa làm**, đúng điều kiện của task: tốn tiền, cần người dùng đồng ý. `ORV-20` đã viết sẵn nhưng chưa chạy; chưa đo chi phí/thời gian nào.

## Tài liệu và cấu hình

- [x] **BE-OR-T540** `docs/guides/request/outcome-review-setup.md`: cài hai skill, Python cho `self_check.py`, biến môi trường, bật theo project, kiểm bằng `orca engine show --check`, lỗi `REQUEST_REVIEW_*`, xoá artifact, cập nhật skill ghim.
  Kết quả (2026-10-20): `docs/guides/request/outcome-review-setup.md` (đã nối vào `docs/guides/request/README.md`): hai skill, Python, biến môi trường, bật theo project bằng `orca engine set --outcome-review`, kiểm bằng `orca engine show --check`, bảng lỗi `REQUEST_REVIEW_*`, xoá nội dung, cập nhật skill ghim, các giai đoạn, số liệu. Nói thẳng rằng `diagram-design` chưa được vendor.

- [x] **BE-OR-T541** `deploy/dev/.env.example`, `docker-compose.yml` (mặc định `REQUEST_DEFAULT_OUTCOME_REVIEW=off`), mục mới trong `deploy/dev/RELEASE-*.md`.
  Kết quả (2026-10-20): `deploy/dev/.env.example` và `deploy/dev/docker-compose.yml` thêm 10 biến (`REQUEST_DEFAULT_OUTCOME_REVIEW=off` mặc định, kill switch, draw flag và 7 giới hạn tuỳ chọn); `docker compose config` hợp lệ; `deploy/dev/RELEASE-2026-10-DEPLOY.md` có mục 9. `deploy/prod` chưa sửa (không có khối `REQUEST_DEFAULT_*` ở đó).

## Rollout

- [~] **BE-OR-T550** Giai đoạn 0 (pilot `vnp-qa-platform`, `manual`) → 1 (chỉ chữ, `REQUEST_REVIEW_DRAW_ENABLED=false`) → 2 (có hình) → 3 (hồ sơ vẽ, thêm project). Mỗi bước ghi điều kiện qua.
  Kết quả (2026-10-20): biến `REQUEST_REVIEW_DRAW_ENABLED` được thêm và có hiệu lực (`OutcomeReviewConfig.DrawDisabled`: lượt vẽ trả `skipped=draw_disabled`, không có hàng run, không gọi agent; ước tính tái tạo không tính hình); test đọc biến (cả giá trị sai → giữ bật + cảnh báo). Điều kiện qua từng giai đoạn ghi ở hướng dẫn mục 7. **Chưa chạy giai đoạn nào** (0–3) trên pilot `vnp-qa-platform`.

- [x] **BE-OR-T551** Kiểm công tắc `REQUEST_REVIEW_KILL_SWITCH`: ẩn tab, chặn RPC, không xoá dữ liệu; bật lại không mất review.
  Kết quả (2026-10-20): công tắc `REQUEST_REVIEW_KILL_SWITCH` chặn **mọi** RPC review kể cả đọc (`outcomeReviewWired`; test duyệt cả 17 RPC → `FailedPrecondition` + `REQUEST_REVIEW_DISABLED`), ẩn tab (`Effective` → off). `TestReviewKillSwitch_KeepsDataAndRestoresItUnchanged`: review có từ trước giữ nguyên id, version, mô hình sau khi bật rồi tắt công tắc, và lời gọi bị từ chối không tốn lượt agent.

## Hoàn tất

- [~] **BE-OR-T599** `go build` + `go test` + tích hợp hai dialect + `detect_changes({scope:"compare", base_ref:"main"})` (hoặc ghi rõ nếu công cụ không có); chỉ commit khi người dùng yêu cầu.
  Kết quả (2026-10-20): `go build` request-service, CLI, prompts, gateway (overlay) sạch; `go vet` sạch; `go test -race` PASS toàn bộ request-service (15 gói) và prompts (21 gói); `make check-prompts` PASS; tích hợp **Postgres 16 và MySQL 8 thật** PASS toàn bộ gói adapter (74 s và 127 s); gateway `-race` PASS trừ 3 test screencast chập chờn (chạy riêng PASS, không dùng mã review). **Không có:** `detect_changes` của GitNexus (công cụ không có trong phiên này, nên không đối chiếu được với `main`); `opa test` (không có `opa`); gateway chỉ kiểm qua `-overlay` vì HEAD không biên dịch; không có lượt chạy stack thật. Phase 1–4 đã commit; Phase 5 và các phần sau chưa commit.

