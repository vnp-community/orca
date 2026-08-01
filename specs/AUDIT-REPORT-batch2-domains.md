# AUDIT REPORT — agent-ws, ai-providers, auth, automation, code-review, cli-headless

**Ngày:** 2026-08-01  
**Phạm vi:** 6 domains: agent-ws, ai-providers, auth, automation, code-review, cli-headless  
**Phương pháp:** HLD-to-Code Mapping, grep analysis, source file review

---

## Tóm tắt Executive

| Domain | Bugs phát hiện | Severity cao nhất | Tình trạng |
|--------|---------------|-------------------|------------|
| agent-ws | 4 bugs | 🔴 CRITICAL (topology sai) | Partial — Direct-WS implement, relay-ws doc sai |
| ai-providers | 4 bugs | 🔴 CRITICAL (pending never resolved) | Partial — CRUD OK, flow broken |
| auth | 3 bugs | 🟡 MEDIUM | Partial — login OK, audit/security gaps |
| automation | 3 bugs | 🔴 HIGH | Partial — schedule OK, event/remote broken |
| code-review | 1 bug | 🔴 HIGH | Local flow only, remote path missing |
| cli-headless | 1 bug | 🟡 MEDIUM | Headless mode incomplete |
| **TOTAL** | **16 bugs** | | |

---

## Phát hiện nổi bật

### 1. agent-ws — Topology BL-AWS-01 ngược chiều (CRITICAL)
HLD mô tả Orca chủ động kết nối đến Dev Server (`ws://dev-server:6799`). Thực tế code chỉ implement Dev Server là WS client kết nối vào Orca. Đây là **lỗi documentation**, không phải lỗi code — nhưng gây hiểu lầm nghiêm trọng.

### 2. ai-providers — Account mới không thể dùng trong 15 phút (CRITICAL)
`createAccount()` trả về `status='pending'`. `resolveForProject()` chỉ filter `status='active'`. ProviderHealthChecker cần chạy (15 phút) trước khi status='active'. → Bất kỳ agent nào cố dùng provider account mới đều FAIL.

### 3. ai-providers — Credential không được decrypt trước khi relay (CRITICAL)
Server forward `encryptedBlob` (browser-encrypted với PBKDF2(sessionToken)) sang Dev Server. Dev Server không có session key để decrypt → credential store sai → agent auth fail.

### 4. automation — Event-based triggers hoàn toàn chưa implement (HIGH)
`EventBus`, `AutomationEventHandler`, GitHub webhook handler đều không tồn tại. BL-AT-03 là 0% implemented.

### 5. automation — Remote host scheduling disabled by default (MEDIUM)
`allowRemoteHostScheduling = false` → tất cả remote-server automations bị block. Flag này cần được bật khi có relay connections.

---

## Mapping Bug → Component

### Backend (SRV) Bugs

| Bug ID | Domain | Mô tả ngắn | Severity |
|--------|--------|------------|----------|
| BUG-AWS-001 | agent-ws | BL-AWS-01 topology doc ngược chiều | 🔴 CRITICAL |
| BUG-AWS-002 | agent-ws | Token không SHA256 hash, không persist | 🟡 MEDIUM |
| BUG-AWS-003 | agent-ws | Token TTL max 10min, không có refresh | 🔴 HIGH |
| BUG-AWS-004 | agent-ws | X-Orca-Admin bypass nếu thiếu env var | 🔴 HIGH |
| BUG-AIP-001 | ai-providers | pending status → account không bao giờ resolve | 🔴 CRITICAL |
| BUG-AIP-002 | ai-providers | encryptedBlob không decrypt trước khi relay | 🔴 HIGH |
| BUG-AIP-003 | ai-providers | HealthChecker không emit alert khi status đổi | 🟡 MEDIUM |
| BUG-AIP-004 | ai-providers | _relayPool tham số không dùng | 🟢 LOW |
| BUG-AUTH-001 | auth | Login không ghi audit log | 🟡 MEDIUM |
| BUG-AUTH-002 | auth | Cookie SameSite=Lax thay vì Strict | 🟡 MEDIUM |
| BUG-AUTH-003 | auth | SessionManager thiếu idle timeout | 🟡 MEDIUM |
| BUG-AT-001 | automation | AutomationService phụ thuộc Electron WebContents | 🔴 HIGH |
| BUG-AT-002 | automation | EventBus + event triggers chưa implement | 🔴 HIGH |
| BUG-AT-003 | automation | Remote scheduling disabled by default | 🟡 MEDIUM |
| BUG-CR-001 | code-review | DiffService missing, remote path không có | 🔴 HIGH |
| BUG-CLI-001 | cli-headless | HeadlessDispatcher không được wire vào headless startup | 🟡 MEDIUM |

### Agent (DEV) Bugs

Không có bugs mới từ domains này cụ thể cho Dev Server layer (các bugs relay chính đã được ghi trong agent-orchestration domain).

### Frontend (WEB) Bugs

Không phát hiện bugs frontend mới cụ thể từ 6 domains này. (Phụ thuộc vào AI Provider UI và Code Review UI chưa được audit riêng).

---

## Documents đã cập nhật

| Document | Thay đổi |
|----------|---------|
| `docs/flows/logic/cli-headless.md` | Cập nhật phản ánh Electron architecture, thêm Dev Server path |
| `docs/flows/logic/code-review.md` | Thêm remote worktree dual-path via relay |
| `docs/flows/component-mapping.md` | Thêm BL-AT-01→04 và BL-CR-01→05, cập nhật CLI/Headless note |

---

## Priority Fix Order

1. 🔴 **BUG-AIP-001** + **BUG-AIP-002**: Credential flow broken → không có agent nào hoạt động được với remote AI providers
2. 🔴 **BUG-AWS-004**: Security bypass → cần fix ngay
3. 🔴 **BUG-AT-002**: EventBus implement → BL-AT-03 capability
4. 🔴 **BUG-AT-001**: Wire HeadlessDispatcher → headless mode usable
5. 🟡 Remaining medium bugs

---

## Liên kết tài liệu

- [agent-ws bugs](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/agent-ws/)
- [ai-providers bugs](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/ai-providers/)
- [auth bugs](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/auth/)
- [automation bugs](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/automation/)
- [code-review bugs](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/code-review/)
- [cli-headless bugs](file:///Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend/bugs/cli-headless/)
