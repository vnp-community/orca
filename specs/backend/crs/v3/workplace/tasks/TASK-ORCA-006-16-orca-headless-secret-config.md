> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
## Status: 🔲 NOT STARTED

# TASK-ORCA-006-16 — Orca: `ORCA_PLANNER_API_SECRET` + Callback Timeout Config + Headless Verification

**Phase:** 0 — Nền tảng (điều kiện tiên quyết vận hành cho integration thật CR-ORCA-002/005)
**Scope:** 🟠 **Orca TypeScript/config CONTRACT — KHÔNG thực thi trong repo `vnp-workplace`.** Config/env var mới cho repo `orca` (`/opt/repos/orca`): `deploy/prod/.env.example`, đọc biến trong `backend/src/server/planner-task-routes.ts` (TASK-ORCA-001-13) và `PlannerCallbackPublisher` (TASK-ORCA-004-15).
**Source:** [SOL-ORCA-006 §3, §5, §10](../solutions/SOL-ORCA-006-orca-headless-deployment.md#3-trách-nhiệm-phía-orca-ngoài-phạm-vi--tóm-tắt-điều-kiện-tiên-quyết-giữ-nguyên-không-đổi-theo-re-scope)
**Depends On:** — (song song, cần xong trước khi TASK-ORCA-001-13/004-15 có thể chạy integration thật)
**Người thực thi:** Orca team

---

## Vì sao task này tồn tại

`planner-service` (phía `vnp-workplace`) cấu hình `ORCA_API_SECRET` — biến tương ứng phía Orca, `ORCA_PLANNER_API_SECRET`, **chưa tồn tại phía Orca hôm nay** (biến gần nhất, khác mục đích, là `ORCA_AGENT_API_SECRET`). Orca **đã có sẵn** headless server mode thật (`orca serve`, `backend/src/server/index.ts`) và deploy tooling thật (`deploy/prod/Dockerfile`, `deploy/prod/docker-compose.yml`, `deploy/prod/.env.example`) — task này chỉ bổ sung **1 biến môi trường mới** + xác nhận vận hành, KHÔNG "phát minh" lại headless deployment từ đầu.

> **Ghi chú Re-scope (2026-08-10):** Task này thuần Orca-side — nội dung kỹ thuật (biến môi trường, vị trí code, checklist vận hành) **giữ nguyên không đổi**. Điểm duy nhất cập nhật là phía đối tác nhận secret/callback: trước đây `temporal-worker`/`signal-svc`/`plan-svc` (repo `vnp-planner`, ngoài `vnp-workplace`), nay là **`planner-service` (`backend/services/planner-service/`, `:3013`, trong chính monorepo `vnp-workplace`)** — 1 service duy nhất đảm nhiệm cả dispatch, callback receiver, và discovery config. Xem bảng ánh xạ đầy đủ tại [`docs/crs/v3/orca/README.md#ghi-chú-re-scope-2026-08-10`](../../../../../../docs/crs/v3/orca/README.md#ghi-chú-re-scope-2026-08-10).

---

## Bối cảnh quan trọng — sai lệch đã xác nhận so với CR gốc (Orca-side, không đổi theo re-scope)

| Điểm | CR-ORCA-006 gốc giả định | Thật |
|---|---|---|
| Port | `:3000` | HTTP `:6769`, WebSocket `:6768` (`backend/src/server/index.ts:14-15,46-47`) |
| Cờ headless | `orca serve --headless` | Không có cờ `--headless` — `orca serve --port <port> --pairing-address <domain>` (`desktop/src/cli/specs/serve.ts:30`); "headless" là hệ quả của dùng entry point `backend/src/server/index.ts` thay vì Electron main |
| Cấu hình | `server-config.json` | Biến môi trường (`ORCA_PORT`, `ORCA_HTTP_PORT`, `ORCA_MULTI_USER`, `ORCA_AUTH_MODE`, `ORCA_ADMIN_EMAIL/PASSWORD`, `ORCA_DB_URL`, xem `deploy/prod/.env.example`) |
| Healthcheck | `/health` | `/health/ready` (route Docker healthcheck thật dùng, `deploy/prod/docker-compose.yml`) |
| Image | `ghcr.io/stablyai/orca:latest` | `vnpblc/orca-server` (`deploy/prod/Dockerfile`) |

---

## Acceptance Criteria

- [ ] `ORCA_PLANNER_API_SECRET` thêm vào `deploy/prod/.env.example` với comment giải thích mục đích (service-to-service auth cho `/api/planner-tasks*`, khác `ORCA_AGENT_API_SECRET`) — giá trị phải khớp `ORCA_API_SECRET` phía `planner-service` (`backend/services/planner-service/config/config.go`, TASK-ORCA-006-12)
- [ ] `ORCA_PLANNER_CALLBACK_TIMEOUT_MS` thêm vào `deploy/prod/.env.example`, mặc định `10000`, đọc trong `PlannerCallbackPublisher` (TASK-ORCA-004-15)
- [ ] `curl http://<orca-host>:6769/health/ready` trả `200` ổn định (xác nhận vận hành, không phải code)
- [ ] Xác nhận với `vnp-workplace`: task store thật (`orca_tasks`, tuỳ `ORCA_STORAGE_BACKEND`) khôi phục được sau `orca serve` restart — nếu không, `planner-service` sẽ nhận `404` qua poll fallback và coi là lỗi dispatch vĩnh viễn, cần biết trước để set kỳ vọng đúng
- [ ] Xác nhận route mạng: `ORCA_CALLBACK_URL` (trỏ về `planner-service`, mặc định `http://planner-service:3013/api/v1/orca-callback`) reachable từ mạng nơi Orca chạy (test 2 chiều bằng `curl` thủ công trước go-live, đặc biệt nếu Orca chạy trong DMZ/mạng tách biệt)
- [ ] `Restart=always` (hoặc tương đương) cho tiến trình `orca serve` — xác nhận SLA restart rõ ràng với `vnp-workplace` (không có systemd unit mẫu sẵn cho `orca serve` trong repo — unit thật duy nhất tìm thấy là cho dev-server agent daemon, không áp dụng trực tiếp)

---

## Thay đổi cấu hình mẫu

### `deploy/prod/.env.example` [MODIFY — thêm vào cuối, giữ nguyên các biến hiện có]

```bash
# ─── Planner Integration (CR-ORCA-001..006) ──────────────────────────────────
# Service-to-service secret for POST/GET /api/planner-tasks* (TASK-ORCA-001-13).
# DISTINCT from ORCA_AGENT_API_SECRET (used by /api/agent-token and
# /api/trace-stream for a different purpose — dev-server relay agent auth).
# Must match ORCA_API_SECRET on the planner-service side (vnp-workplace,
# backend/services/planner-service/config/config.go).
# Generate with: openssl rand -hex 32
ORCA_PLANNER_API_SECRET=

# Timeout (ms) for PlannerCallbackPublisher.publish() (TASK-ORCA-004-15) before
# giving up on delivering a task result to planner-service's callback_url. No
# retry occurs after this timeout — planner-service's poll fallback covers
# missed callbacks.
ORCA_PLANNER_CALLBACK_TIMEOUT_MS=10000
```

### Đọc biến trong code (tham chiếu — đã dùng trong TASK-ORCA-001-13/004-15)

```ts
// backend/src/server/planner-task-routes.ts — isAuthorized() (TASK-ORCA-001-13)
const apiSecret = process.env['ORCA_PLANNER_API_SECRET']?.trim()

// backend/src/main/task/PlannerCallbackPublisher.ts — constructor default (TASK-ORCA-004-15)
private readonly timeoutMs = Number(process.env['ORCA_PLANNER_CALLBACK_TIMEOUT_MS'] ?? 10000)
```

Không cần code mới nào khác ngoài 2 điểm đọc biến này (đã đặc tả đầy đủ ở TASK-ORCA-001-13 và TASK-ORCA-004-15) — task này chủ yếu là **thêm biến cấu hình + checklist vận hành xác nhận với `vnp-workplace`**, không phải viết logic mới.

---

## Checklist vận hành cần Orca team xác nhận với `vnp-workplace` (2 chiều)

```markdown
- [ ] `ORCA_PLANNER_API_SECRET` được generate (`openssl rand -hex 32`) và chia sẻ
      an toàn cho vnp-workplace (không qua Slack/email plaintext — dùng vault/secret
      manager chung nếu có)
- [ ] Cùng giá trị đó được set ở CẢ 2 phía: Orca (`deploy/prod/.env`, biến
      `ORCA_PLANNER_API_SECRET`) VÀ `planner-service` (`vnp-workplace`, biến
      `ORCA_API_SECRET` — xem TASK-ORCA-006-12, TASK-ORCA-002-06)
- [ ] `curl http://<orca-host>:6769/health/ready` → 200 từ mạng nơi `planner-service`
      chạy (không chỉ từ localhost trên Orca host)
- [ ] `curl` 2 chiều: từ Orca host tới `ORCA_CALLBACK_URL` (`planner-service`,
      `:3013/api/v1/orca-callback`) — xác nhận route mạng mở trước go-live, đặc
      biệt nếu 1 trong 2 phía ở DMZ
- [ ] Xác nhận `ORCA_STORAGE_BACKEND` đang dùng (SQLite/MySQL/PostgreSQL/TiDB)
      và task store khôi phục đúng sau restart — `planner-service` cần biết SLA
      này để set retry/timeout phù hợp cho luồng dispatch (SOL-ORCA-002)
```

---

## Verification

```bash
# Phía Orca team (trên host chạy orca serve):
curl -f http://localhost:6769/health/ready

# Xác nhận secret hoạt động (sau khi TASK-ORCA-001-13 đã merge):
curl -X POST http://localhost:6769/api/planner-tasks \
  -H "Authorization: Bearer $ORCA_PLANNER_API_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"planner_task_id":"smoke-test","title":"t","description":"d","worktree_repo":"git@x","agent_type":"claude","priority":"P1"}'
# Kỳ vọng: 201, KHÔNG 401
```
