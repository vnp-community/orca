# TASK-TDD-002-T4-03: Đăng ký vnp-qa-platform thành project Orca và bật cờ

**From Solution:** [SOL-TDD-002](../solutions/SOL-TDD-002-tdd-agent-as-knowledge-hub.md) mục 4 (bước 3)
**Checklist:** [T4.3](CHECKLIST-TDD-002-knowledge-hub.md)
**Priority:** P0 · **Area:** orca (stack local)
**Files:** stack local, `orca` CLI, REST `/v1/projects`, `orca engine set` (T2.10)
**Depends on:** T2.10, T4.2
**Status:** `[ ]` TODO

---

## Context

Orca cần một project có repo trỏ tới worktree pilot trên dev server `dev-local-mac`, để task worktree của Orca nằm cạnh worktree pilot và `knowledgeRoot` là chính worktree này. Project/Request của pilot không được lẫn với dữ liệu người dùng; dùng tên có tiền tố nhận diện và dọn ở T4.8.

## Việc cần làm

1. Xác định id `dev-local-mac` (`GET /v1/infra/dev-servers`, host `dev-local-mac`).
2. Tạo project (UI hoặc `POST /v1/projects`) tên `pilot-tdd-<ngày>`; thêm repo `url` = đường dẫn worktree pilot, `dev_server_id` = id trên.
3. `orca engine show --project <id>`; bật `orca engine set --project <id> --engine openspec --solution-agent=true`; xem kết quả preflight, xử lý từng lỗi (`fix_hint`) tới khi đạt.
4. Smoke: `orca request create` một Request thử, `classify`, xác nhận chạy được rồi huỷ Request đó.
5. Ghi id project, id repo, kết quả preflight.

## Tiêu chí hoàn thành

- [ ] Preflight đạt đủ (openspec, tdd-agent, skill, `.tdd-agent`, feature agent).
- [ ] Project chỉ chứa repo pilot; không động tới project khác.

## Kiểm thử

- Smoke bằng CLI.

## Rủi ro và ghi chú

- Nếu preflight đạt giả (stub chưa thay), dừng: T2.3 chưa xong.
