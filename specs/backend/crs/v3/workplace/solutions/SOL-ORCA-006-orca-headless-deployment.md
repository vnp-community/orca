> ⚠️ **SUPERSEDED (2026-08-10):** Tài liệu task/solution breakdown này được sinh ra từ các CR-ORCA-00x/CR-TASK-00x đã bị viết lại hoặc retired — dựa trên giả định sai rằng `vnp-planner` bị loại bỏ và `vnp-workplace` tự dựng `planner-service` mới. Kiến trúc đúng: xem [`docs/crs/v3/orca/README.md`](../../../../../docs/crs/v3/orca/README.md). Nội dung bên dưới chỉ còn giá trị tham khảo lịch sử — không dùng để implement.
>
> ---
>
# SOL-ORCA-006 — Orca Headless Deployment

| Field | Value |
|-------|-------|
| **CR ref** | [CR-ORCA-006](../../../../../../docs/crs/v3/orca/CR-ORCA-006-orca-headless-deployment.md) |
| **Title** | Orca Headless Deployment — vận hành server mode |
| **Service** | Orca deploy (bootstrap/systemd — repo `orca`) **+** `planner-service` (`backend/services/planner-service/config/`, `deploy/docker/docker-compose.yml`, `:3013` — repo `vnp-workplace`) |
| **Priority** | P1 |
| **Risk** | high |
| **Status** | 📐 PROPOSED |
| **Phạm vi** | Phần **bootstrap headless của Orca** (`orca serve`, systemd unit, cấu hình env) — **ngoài phạm vi Go monorepo**, Orca team tự vận hành theo checklist §5. Phần **discovery config phía `planner-service`** (biết địa chỉ Orca instance, `deploy/docker/docker-compose.orca.yml` overlay, entry `planner-service` trong `deploy/docker/docker-compose.yml`) — **trong phạm vi `vnp-workplace`**, được đặc tả đầy đủ ở §4. |
| **TDD refs** | `backend/specs/tdd/v1/01-project-structure.md` (deploy/ layout), `backend/specs/tdd/v1/00-go-conventions.md` §7 (config.go/envconfig), `backend/specs/tdd/v3/README.md` §Services & Binaries Mới (port allocation `:3013`) |
| **Depends on** | — (song song, cần có trước khi CR-ORCA-002 chạy integration test thật) |

---

## 1. Tóm tắt vấn đề & mục tiêu

> ⚠️ **Xác thực với Orca thật (2026-08-10):** Orca **đã có sẵn** headless server mode thật (`orca serve`, entry point `backend/src/server/index.ts`) và deploy tooling thật trong chính repo `orca` (`deploy/prod/Dockerfile`, `deploy/prod/docker-compose.yml`, và một `deploy/dev/docker-compose.orca.yml` **Orca team đã tự vận hành** cho hạ tầng `b15.openledger.vn`) — khác với giả định gốc rằng phía tích hợp cần "phát minh" cấu hình headless từ đầu. Nhiều chi tiết trong CR gốc (port `:3000`, cờ `--headless`, `server-config.json`, image `ghcr.io/stablyai/orca:latest`) không khớp thực tế. Đã sửa tại chỗ + bổ sung §10 cuối file. Mục này **thuần Orca-side, không đổi theo re-scope 2026-08-10** dưới đây.

> **Ghi chú Re-scope (2026-08-10):** SOL này trước đây đặc tả phần discovery config cho `temporal-worker`/`signal-svc`/`plan-svc` (`vnp-planner` — Go backend độc lập, **ngoài** repo `vnp-workplace`). Quyết định kiến trúc mới: **`vnp-workplace` thay thế hoàn toàn `vnp-planner`**. Toàn bộ vai trò dispatch + result-callback + discovery-config gộp vào **1 service mới trong `vnp-workplace`: `planner-service` (`:3013`)**, tái dùng `knowledge-service`/`skills-service`/`diagnostics-service`/`notification-service`/`ai-service` đã có sẵn thay vì dựng `context-svc`/`signal-svc`/`assessment-svc` mới. §3 (khảo sát Orca thật — port, route, image, systemd) **giữ nguyên không đổi**, vì là thông tin đã xác thực trực tiếp trên code `orca`, độc lập với kiến trúc phía tích hợp. Bảng ánh xạ đầy đủ: [`docs/crs/v3/orca/README.md#ghi-chú-re-scope-2026-08-10`](../../../../../../docs/crs/v3/orca/README.md#ghi-chú-re-scope-2026-08-10).

Orca là Electron desktop app — để `planner-service` (dispatcher theo SOL-ORCA-002, session monitor theo SOL-ORCA-005, callback receiver theo SOL-ORCA-004) gọi được, Orca phải chạy ở **headless server mode**, persistent, có địa chỉ mạng ổn định. SOL này (1) xác nhận checklist vận hành phía Orca là **điều kiện tiên quyết** (precondition) trước khi bật integration thật của CR-ORCA-002/005, (2) đặc tả phần cấu hình discovery + docker-compose thuộc về `planner-service` (`vnp-workplace`).

## 2. Kiến trúc triển khai (đã sửa theo Orca thật — xem §10; phía tích hợp đã re-scope sang `planner-service`)

```
Server hạ tầng
  orca-node (container hoặc VM riêng)
    orca serve --port 6768 --pairing-address <domain>     # KHÔNG có cờ --headless — "headless" là ngầm định
                                                            # khi chạy qua entry point src/server/index.ts,
                                                            # không phải Electron main (desktop/src/main/index.ts)
      HTTP  :6769  /api/trace-stream, /api/agent-token, /health, /health/ready, /health/metrics, /auth/*, /admin/api/*
                   (KHÔNG có /api/planner-tasks — endpoint đó CHƯA tồn tại, xem SOL-ORCA-001 §9)
      WS    :6768  /  (browser JSON-RPC)  và  /agent  (dev-server relay agent, không phải AI coding agent)
    Worktree do Orca UI/RPC tạo theo yêu cầu người dùng — không có thư mục cố định
    kiểu /workspace/worktrees do Orca tự quản lý theo quy ước planner
           │
           │ HTTP nội bộ mạng (kwp-network / VPN)
           ▼
  vnp-workplace: planner-service (:3013)  [NEW — thay thế plan-svc + temporal-worker của vnp-planner]
    dispatcher       (SOL-ORCA-002) → OrcaURL = http://<orca-host>:6769 (`ORCA_URL`, `config.go` §4.1)
    session monitor   (SOL-ORCA-005, đọc qua diagnostics-service) → poll cùng OrcaURL
    result callback   (SOL-ORCA-004) ← nhận POST /api/v1/orca-callback tại OrcaCallbackURL
                                        (mặc định `http://planner-service:3013/api/v1/orca-callback`
                                        — cơ chế phát callback CHƯA tồn tại phía Orca, xem SOL-ORCA-004 §9)
```

## 3. Trách nhiệm phía Orca (ngoài phạm vi — tóm tắt điều kiện tiên quyết, GIỮ NGUYÊN không đổi theo re-scope)

Không thiết kế lại — CR-ORCA-006 gốc đã đủ chi tiết (`server-config.json`, systemd unit, checklist). **Xác thực với Orca thật:** `server-config.json` không tìm thấy trong repo `orca`; cấu hình thật dùng biến môi trường (`ORCA_PORT`, `ORCA_HTTP_PORT`, `ORCA_MULTI_USER`, `ORCA_AUTH_MODE`, `ORCA_ADMIN_EMAIL/PASSWORD`, `ORCA_DB_URL`, xem `deploy/prod/.env.example`), không phải file JSON. Systemd unit có thật trong repo (`deploy/agent/orca-agent.service`) là cho **dev-server agent daemon** (`agent.js`, chạy trên máy remote), **không phải** cho tiến trình `orca serve` chính — không tìm thấy systemd unit mẫu riêng cho `orca serve` trong repo; `deploy/dev/specs/01-orca-server-setup.md` hướng dẫn chạy `orca serve` trực tiếp (qua nginx reverse proxy), không qua systemd. Điều kiện **bắt buộc** `planner-service` (vnp-workplace) cần Orca team xác nhận trước khi tích hợp:

1. `curl http://<orca-host>:6769/health/ready` trả `200` liên tục (không chỉ lúc mới khởi động) — port thật `6769` (HTTP), không phải `3000`; route `/health/ready` là route Orca team tự dùng làm Docker healthcheck (`deploy/prod/docker-compose.yml`), khuyến nghị dùng chung route này thay vì `/health` trơn.
2. `ORCA_API_SECRET` (phía `planner-service`, xem `config.go` §4.1) giống hệt giá trị cấu hình `ORCA_PLANNER_API_SECRET` phía Orca — **lưu ý:** biến này chưa tồn tại phía Orca hôm nay, Orca team cần thêm mới (xem SOL-ORCA-001 §9 pt.3).
3. Endpoint `/api/planner-tasks` đã triển khai theo đúng contract SOL-ORCA-001 (không phải version cũ chưa có idempotency §4 của SOL-ORCA-001) — **lưu ý:** endpoint này hiện **hoàn toàn chưa tồn tại**, đây không phải điều kiện "đã triển khai đúng version" mà là "đã triển khai lần đầu", xem SOL-ORCA-001 §9 pt.1.
4. `/workspace` có đủ dung lượng và **không** bị dọn dẹp tự động trong lúc task đang `IN_PROGRESS` (worktree bị xoá giữa chừng sẽ làm hỏng kết quả — liên quan SOL-ORCA-003 §3.4) — **lưu ý:** không có thư mục cố định `/workspace` trong Orca thật; worktree path do Orca UI/RPC quyết định theo cấu hình project, cần xác nhận với Orca team quy ước thư mục thật sẽ dùng.
5. `orca serve` cấu hình `Restart=always` (systemd) — nếu Orca crash giữa lúc `planner-service` đang chờ callback, poll fallback (SOL-ORCA-002 §3.7) chỉ hoạt động lại sau khi Orca lên — cần SLA restart rõ ràng. **Lưu ý:** không có systemd unit mẫu cho `orca serve` trong repo để đối chiếu trực tiếp `RestartSec`; unit thật duy nhất tìm thấy (`deploy/agent/orca-agent.service`, cho dev-server agent) dùng `RestartSec=5s` (không phải `10s`) — không nên giả định con số này áp dụng cho `orca serve` mà cần Orca team xác nhận riêng.

## 4. Phần thuộc phạm vi `planner-service` (vnp-workplace)

### 4.1 `planner-service` — Orca discovery config

`backend/services/planner-service/config/config.go` (CR-ORCA-006 Change 4) đã khai báo đầy đủ field discovery MVP theo convention `envconfig` chung toàn bộ services WKP (`OrcaURL`, `OrcaAPISecret`, `OrcaCallbackURL`, `OrcaCallbackSecret`, `OrcaPollIntervalSecs`, `OrcaHealthCheckIntervalSecs`, `OrcaTaskTimeoutHours`). SOL này mở rộng thêm 1 field **optional**, chuẩn bị chỗ cho multi-instance (§4.4/Change 5), nhưng **MVP chỉ dùng 1 instance** — tránh triển khai `OrcaInstancePool` (CR gốc "future") khi chưa có nhu cầu tải thật:

```go
// backend/services/planner-service/config/config.go — bổ sung so với CR-ORCA-006 Change 4
type Config struct {
	// ... các field đã có (OrcaURL, OrcaAPISecret, OrcaCallbackURL, OrcaCallbackSecret, ...) ...

	// Orca — hỗ trợ multi-instance qua danh sách JSON, MVP dùng phần tử đầu tiên.
	OrcaInstancesJSON string `envconfig:"ORCA_INSTANCES_JSON" default:""` // optional override
}

type OrcaInstanceConfig struct {
	URL                 string   `json:"url"`
	Name                string   `json:"name"`
	APISecret           string   `json:"api_secret"`
	MaxConcurrentTasks  int      `json:"max_concurrent_tasks"`
	PreferredAgentTypes []string `json:"preferred_agent_types"`
}
```

Nếu `ORCA_INSTANCES_JSON` rỗng, dùng trực tiếp `OrcaURL`/`OrcaAPISecret` đơn instance (CR-ORCA-006 Change 4) — giữ đường đơn giản cho MVP, tránh code chết (`OrcaInstancePool.SelectBest`, Change 5) không có test coverage thật.

### 4.2 `deploy/docker/docker-compose.orca.yml` — dev/integration-test overlay trong `vnp-workplace`

> **Xác thực với Orca thật:** repo `orca` **đã có sẵn** một `deploy/dev/docker-compose.orca.yml` mà Orca team tự dùng để vận hành instance thật của họ (2-server: Gateway 172.20.2.16 + Orca server 172.20.2.39, domain `b15.openledger.vn`), cùng `deploy/prod/Dockerfile` build image `vnpblc/orca-server` (KHÔNG phải `ghcr.io/stablyai/orca:latest` như giả định ở bảng rủi ro §7 — không tìm thấy image `ghcr.io/stablyai/orca` nào được publish/tham chiếu trong repo `orca`) và `deploy/prod/docker-compose.yml` + `.env.example` riêng của họ. Vì vậy: (a) `vnp-workplace` **không cần tự build lại** cấu hình chạy Orca từ đầu cho môi trường staging/prod — chỉ cần trỏ `ORCA_URL` tới instance Orca team đã vận hành; (b) file `deploy/docker/docker-compose.orca.yml` phía `vnp-workplace` (dùng cho integration test cục bộ, theo CR-ORCA-006 Change 3) nên tham khảo trực tiếp cấu trúc thật trong repo `orca` (image `vnpblc/orca-server`, ports `6768:6768`/`6769:6769`, healthcheck `wget http://localhost:6769/health/ready`) thay vì đoán tên image/port như bản CR gốc (`ghcr.io/stablyai/orca:latest`, port `3000`). Nội dung dưới đây đã cập nhật cho khớp thực tế Orca, đặt đúng theo path CR-ORCA-006 Change 3 (`deploy/docker/docker-compose.orca.yml`, overlay riêng — không merge vào `deploy/docker/docker-compose.yml` chính vì Orca là Electron/TypeScript app bên ngoài, không phải Go service nội bộ WKP):

```yaml
# deploy/docker/docker-compose.orca.yml — chỉ dùng cho dev/integration test cục bộ.
# staging/prod KHÔNG đóng gói Orca trong compose của vnp-workplace — Orca là hệ thống
# ngoài do team Orca vận hành riêng (deploy/prod/Dockerfile, deploy/prod/docker-compose.yml
# trong chính repo `orca`); chỉ cần trỏ ORCA_URL tới instance thật của họ cho các môi trường đó.
version: '3.9'

services:
  orca:
    image: vnpblc/orca-server:${ORCA_VERSION:-latest}   # KHÔNG phải ghcr.io/stablyai/orca:latest
                                                          # (image không tồn tại — xem §7); pin ${ORCA_VERSION}
                                                          # ngoài môi trường dev
    ports:
      - "6768:6768"   # WebSocket
      - "6769:6769"   # HTTP — /health, /health/ready, /api/trace-stream, /api/agent-token
                       # NOTE: /api/planner-tasks* CHƯA được implement bởi image này cho tới
                       # khi Orca team ship TASK-ORCA-001-13.
    volumes:
      - orca-workspace:/workspace
    environment:
      ORCA_PORT: "6768"
      ORCA_HTTP_PORT: "6769"
      ORCA_MULTI_USER: "false"
      ORCA_AUTH_MODE: "${ORCA_AUTH_MODE:-local}"
      # ORCA_PLANNER_API_SECRET — set khi Orca team thêm biến này (chưa tồn tại hôm nay,
      # xem SOL-ORCA-001 §9 pt.3); phải khớp ORCA_API_SECRET của planner-service (§4.1).
      ORCA_PLANNER_API_SECRET: "${ORCA_API_SECRET:-dev-secret-placeholder}"
    healthcheck:
      test: ["CMD", "wget", "-qO-", "http://localhost:6769/health/ready"]
      interval: 10s
      timeout: 5s
      retries: 6
    restart: unless-stopped
    networks:
      - kwp-network   # cùng network với planner-service và các service WKP khác

volumes:
  orca-workspace:
    driver: local

networks:
  kwp-network:
    external: true
```

Chạy kèm stack chính: `docker compose -f deploy/docker/docker-compose.yml -f deploy/docker/docker-compose.orca.yml --profile phase2 up -d`.

### 4.3 Health-gate cho CI/CD integration test

Trước khi chạy test tích hợp CR-ORCA-002/005 nhắm vào Orca thật (không mock), pipeline CI cần **health-gate**:

```bash
# deploy/docker/scripts/wait-for-orca.sh [NEW]
# Port 6769 (HTTP thật, không phải 3000) + /health/ready (route healthcheck thật Orca team dùng,
# xem deploy/prod/docker-compose.yml trong repo orca)
timeout 60 bash -c 'until curl -sf http://orca:6769/health/ready; do sleep 2; done' \
  || { echo "Orca not healthy after 60s"; exit 1; }
```

### 4.4 Multi-Orca load balancing — hoãn lại, không thiết kế trong SOL này

CR-ORCA-006 Change 5 đề xuất `OrcaInstancePool.SelectBest`. **Không đưa vào phạm vi triển khai hiện tại**: chưa có bằng chứng tải (1 Orca instance đủ cho MVP), thêm abstraction pool nhiều instance trước khi cần sẽ tăng bề mặt test mà không có giá trị đo được. Ghi nhận làm **backlog item** khi `planner-service` báo lỗi dispatch dạng `ErrUnavailable` với tần suất cao trong metrics. Khi cần multi-instance thật (P2), `OrcaInstancePool` sẽ đọc danh sách instance từ bảng Postgres (tương tự cách các service WKP khác lưu config động), có thể kèm Admin UI, thay vì tiếp tục mở rộng envconfig bằng danh sách phân tách JSON.

## 5. Deployment Checklist (thừa hưởng từ CR gốc, bổ sung phần `planner-service`)

```markdown
### Trước khi tích hợp (Orca team xác nhận)
- [ ] `curl http://orca:6769/health/ready` → 200 (ổn định, không chỉ lúc khởi động) — port 6769, route /health/ready (đã sửa từ :3000/health)
- [ ] `/api/planner-tasks` được XÂY MỚI đúng SOL-ORCA-001 (bao gồm idempotency §4) — endpoint này chưa tồn tại, không phải "triển khai đúng version", xem SOL-ORCA-001 §9
- [ ] `ORCA_PLANNER_API_SECRET`/`ORCA_API_SECRET` đã chia sẻ an toàn (không qua Slack/email plaintext) — biến mới hoàn toàn, chưa cấu hình ở đâu phía Orca hôm nay

### `planner-service` config (vnp-workplace)
- [ ] `ORCA_URL`/`ORCA_API_SECRET`/`ORCA_CALLBACK_URL`/`ORCA_CALLBACK_SECRET` set đúng theo `config.go` (§4.1, CR-ORCA-006 Change 4)
- [ ] `ORCA_API_SECRET` khớp `ORCA_PLANNER_API_SECRET` trên Orca server
- [ ] `ORCA_CALLBACK_URL` (`http://planner-service:3013/api/v1/orca-callback` mặc định) truy cập được từ phía Orca server (CR-ORCA-004)
- [ ] `planner-service` xuất hiện trong `docker compose ps` khi chạy `--profile phase2` (`deploy/docker/docker-compose.yml`)
- [ ] `deploy/docker/docker-compose.orca.yml` chạy được cho integration test cục bộ (§4.2)
- [ ] Health-gate script (§4.3) tích hợp vào CI trước khi chạy integration test CR-ORCA-002/005

### Sau khi lên production
- [ ] systemd service của Orca auto-start khi reboot
- [ ] Log rotation cấu hình (`/var/log/orca/`)
- [ ] Health check alert cho cả `orca:6769/health/ready` và `planner-service:3013/health`
- [ ] Disk space alert cho worktree storage (worktrees có thể rất lớn)
- [ ] Dashboard `orca.instance.offline`/`orca.session.timeout` (SOL-ORCA-005, qua `diagnostics-service`) route tới kênh cảnh báo vận hành
- [ ] Theo dõi tần suất `ErrUnavailable` từ `orcaclient` — ngưỡng cân nhắc multi-instance (§4.4)
```

## 6. Tích hợp với các CR khác

- **CR-ORCA-001**: `ORCA_URL` phải trỏ đúng instance đã triển khai theo CR này.
- **CR-ORCA-002/005**: đọc `Config`/`OrcaInstanceConfig` (§4.1) từ đây; SOL-ORCA-005 tiêu thụ dữ liệu qua `diagnostics-service` (tái dùng, thay `signal-svc`).
- **CR-ORCA-004**: `ORCA_CALLBACK_URL` phải reachable từ mạng nơi Orca chạy — nếu Orca ở mạng tách biệt (DMZ), cần xác nhận route mạng trước, không chỉ cấu hình DNS.

## 7. Rủi ro & giảm thiểu

| Rủi ro | Mức độ | Giảm thiểu |
|---|---|---|
| Orca restart mất hết task đang `IN_PROGRESS` (không persist qua restart) | Cao | Xác nhận với Orca team: task store thật (`orca_tasks` — SQLite/MySQL/PostgreSQL/TiDB tuỳ `ORCA_STORAGE_BACKEND`, xem `docs/hld/backend-server-architecture.md` §6.5 trong repo orca, KHÔNG phải "planner_tasks store" nội bộ nào — bảng đó chưa tồn tại, xem SOL-ORCA-001 §9) phải khôi phục được sau restart; nếu không, `planner-service` cần phát hiện qua poll fallback trả `404` và coi là lỗi dispatch |
| Callback URL không reachable từ mạng Orca (firewall/DMZ) | Cao | Test thủ công `curl` 2 chiều trước go-live (checklist §5); nếu không thể mở route, chấp nhận 100% chạy theo poll fallback (SOL-ORCA-002 §3.7) thay vì callback |
| 1 instance Orca là điểm lỗi đơn (SPOF) cho toàn bộ dispatch | Trung bình | Theo dõi `ErrUnavailable` (§4.4); có kế hoạch multi-instance khi cần, không triển khai sớm |
| Compose `deploy/docker/docker-compose.orca.yml` dùng image không pin version → breaking change bất ngờ trong dev | Trung bình | **Sửa theo Orca thật:** image thật do Orca team tự build là `vnpblc/orca-server` (`deploy/prod/Dockerfile` trong repo orca), không phải `ghcr.io/stablyai/orca` (không tìm thấy image này trong repo orca — có thể là tên đã lỗi thời hoặc nhầm lẫn). Pin tag cụ thể (`${ORCA_VERSION}`), cập nhật có chủ đích, không dùng `:latest` ngoài môi trường dev |

## 8. Ước tính công việc

| Component | Task | Giờ |
|---|---|---|
| Orca (ngoài phạm vi) | Bootstrap config, systemd, headless verification | 5h (theo CR gốc) |
| `planner-service` | `deploy/docker/docker-compose.orca.yml` (dev/integration test) | 2h |
| `planner-service` | `OrcaInstanceConfig` (đơn giản hoá, không pool) trong `config.go` | 1h |
| `planner-service` | Health-gate script + CI wiring | 2h |
| `planner-service` | Docker Compose service entry chính (`deploy/docker/docker-compose.yml`, `:3013`) — theo CR-ORCA-006 Change 4 | 1h |
| Ops | Checklist + docs vận hành | 1h |
| **Tổng phía `vnp-workplace`** | | **7h** |

## 9. Dependencies

Không phụ thuộc CR nào để bắt đầu (song song từ đầu). Là điều kiện tiên quyết vận hành cho việc chạy integration test thật của CR-ORCA-002 và CR-ORCA-005 (mock được dùng cho unit/workflow test không cần chờ CR này).

---

## 10. Xác thực với Orca thật (GIỮ NGUYÊN — khảo sát ngày 2026-08-10, không đổi theo re-scope)

Đã đối chiếu với code & deploy config thật tại `/opt/repos/orca`. Đây là SOL có nhiều sai lệch định lượng nhất so với CR gốc (port, tên image, đường dẫn cấu hình) dù ý tưởng kiến trúc tổng thể (Orca chạy headless, phía tích hợp trỏ tới qua `ORCA_URL`) là đúng hướng. Toàn bộ mục này thuần Orca-side, **không phụ thuộc** vào quyết định re-scope `vnp-planner` → `vnp-workplace`/`planner-service`:

1. **Port sai xuyên suốt: `:3000` → phải là `:6769` (HTTP) / `:6768` (WebSocket).** Bằng chứng: `backend/src/server/index.ts:14-15,46-47` (`ORCA_PORT` mặc định 6768, `ORCA_HTTP_PORT` mặc định `ORCA_PORT+1`=6769); `deploy/prod/Dockerfile` `EXPOSE 6768`/`EXPOSE 6769`; `deploy/prod/docker-compose.yml` port mapping `6768:6768`/`6769:6769`. Đã sửa tại §2, §3, §4.3, §5.
2. **Không có cờ `--headless`** — CLI thật là `orca serve --port <port> --pairing-address <domain>` (`desktop/src/cli/specs/serve.ts:30`); "headless" là hệ quả của việc dùng entry point `backend/src/server/index.ts` (Node adapter) thay vì Electron main process (`desktop/src/main/index.ts`), không phải một cờ CLI. Đã sửa tại §2.
3. **`server-config.json` không tồn tại** — cấu hình thật dùng biến môi trường (`ORCA_PORT`, `ORCA_HTTP_PORT`, `ORCA_MULTI_USER`, `ORCA_AUTH_MODE`, `ORCA_ADMIN_EMAIL/PASSWORD`, `ORCA_DB_URL`/`ORCA_DB_*`, `ORCA_FLEET_METRICS_ENABLED`, xem `deploy/prod/.env.example`). Đã ghi chú tại §3.
4. **Systemd unit thật (`deploy/agent/orca-agent.service`) là cho dev-server agent daemon (`agent.js`), không phải cho `orca serve`** — không có unit mẫu cho tiến trình Orca server chính trong repo; `RestartSec=5s` (không phải `10s` như giả định gốc) chỉ áp dụng cho agent daemon, không nên suy ra cho `orca serve`. Đã ghi chú tại §3 điểm 5.
5. **Orca team đã có sẵn deploy tooling thật** (`deploy/prod/Dockerfile`, `deploy/prod/docker-compose.yml`, `deploy/dev/docker-compose.orca.yml` cho hạ tầng `b15.openledger.vn` của chính họ) — build image `vnpblc/orca-server`, KHÔNG phải `ghcr.io/stablyai/orca:latest` như bảng rủi ro gốc giả định (image này không xuất hiện ở đâu trong repo `orca`). `planner-service` nên tham khảo trực tiếp các file này thay vì tự đoán cấu hình. Đã sửa tại §4.2, §7.
6. **Healthcheck route thật Orca team dùng là `/health/ready`**, không phải `/health` trơn (`deploy/prod/docker-compose.yml` healthcheck: `wget -qO- http://localhost:6769/health/ready`). Đã sửa tại §3 điểm 1, §4.3, §5.
7. **`ORCA_PLANNER_API_SECRET` và toàn bộ endpoint `/api/planner-tasks` vẫn chưa tồn tại** (nhắc lại từ SOL-ORCA-001 §9) — checklist §5 đã cập nhật để phản ánh đây là công việc "xây mới", không phải "xác nhận cấu hình có sẵn".
8. **`/workspace/worktrees` không phải quy ước thư mục có thật** — worktree path do Orca UI/RPC quyết định (thường theo cấu hình project/dev-server), không có đường dẫn cố định toàn cục nào được xác nhận trong code. Đã ghi chú tại §2, §3 điểm 4.
