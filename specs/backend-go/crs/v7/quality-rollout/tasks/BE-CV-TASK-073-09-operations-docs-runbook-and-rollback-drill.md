# BE-CV-TASK-073-09: Tài liệu vận hành, runbook, kế hoạch rollout và diễn tập quay lui

**From Solution:** BE-CV-SOL-073
**Priority:** P1
**Service:** `docs`, `code-intel-service`
**File:** `docs/guides/code-intel/README.md`, `install-gitnexus-codegraph-on-dev-server.md`, `enable-code-intel-for-tenant.md`, `using-review-view.md`, `runbook-code-intel.md`, `code-intel-performance-and-metrics.md` (task 071-08), `code-intel-threat-model.md` (task 072-09) (mới), `backend-go/services/code-intel-service/README.md`, `backend-go/README.md`
**Depends on:** BE-CV-TASK-073-01..04, BE-CV-SOL-071, 072, `AG-CV-SOL-073-agent-kill-switch`, `FE-CV-SOL-073`
**Status:** `[x] DONE`

---

## Context

- Nội dung nguồn: CR-073 §2.6 (5 giai đoạn), §2.7 (quay lui), §2.8 (tài liệu), §2.9 (runbook). Viết tiếng Việt; đường dẫn `docs/guides/` cần xác minh khi tạo (README v7 §8 điểm 19 ghi `guides/` ở một số tài liệu).
- Cài công cụ: `npm i -g gitnexus` (AGENTS.md); phiên bản hỗ trợ GitNexus 1.6.9, CodeGraph 1.4.1; **không** chạy `codegraph install`; `PATH` của agent từ `deploy/agent/orca-agent.service`; `gitnexus analyze --index-only` bắt buộc; hai loại token (agent và MCP PAT).
- Mọi con số (3 ngày, 2 tuần, 5 đêm, ngưỡng lỗi < 5 %) là giả định (O-15).

## Việc cần làm

1. Viết các tệp ở trên; mỗi phần chưa kiểm chứng (cách cài CodeGraph không tương tác, thời gian dựng chỉ mục Orca, khôi phục WAL) ghi rõ.
2. `runbook-code-intel.md`: bảng sự cố của CR-073 §2.9 dùng **mã lỗi và metric của hợp đồng** (`CODEINTEL_TOOL_UNAVAILABLE`, `…_INDEX_MISSING`, `…_REPO_NOT_REGISTERED`, `…_DISABLED`, `orca_codeintel_*`); thêm hàng "worktree bẩn sau reindex" và "cờ tắt nhưng job còn chạy".
3. `enable-code-intel-for-tenant.md`: biến `CODEINTEL_*`, `SetSettings`, `CODE_INTEL_SERVICE_ADDR=""`, `ORCA_CODEINTEL_DISABLED=1`, cache 5 s, lưu ý `down` migration không dùng khi còn dữ liệu.
4. Diễn tập quay lui ở dev (tắt cờ, cắt gateway, bật lại): ghi kết quả vào runbook.
5. Cập nhật `README` service và `backend-go/README.md` (hàng service, RPC nào thật). Cột "Trạng thái" của CR v7 khi triển khai (người điều phối).

## Kiểm thử

- Rà soát chéo: mỗi mã lỗi trong runbook tồn tại trong ui-api §2.3; mỗi metric tồn tại ở SOL-071; mỗi lệnh có nguồn (đã chạy hoặc "chưa kiểm chứng").
- Chạy diễn tập một lần; không có test tự động.

## Tiêu chí hoàn thành

- [x] Đủ 7 tệp; ghi rõ `--index-only` và `PATH` systemd.
- [x] Diễn tập quay lui ghi nhận.

## Rủi ro và lưu ý

- Tài liệu lạc hậu khi hợp đồng đổi; liên kết tới mục hợp đồng thay vì chép bảng.
- SSH/remote: runbook ghi rõ độ trễ 50–200 ms/hop và `relay-ssh` ngoài phạm vi v7 (O-5).
