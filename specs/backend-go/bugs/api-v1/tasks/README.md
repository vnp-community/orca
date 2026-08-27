# Tasks — API v1 Bug Fixes

Mỗi task là một đơn vị thực thi độc lập cho AI agent, với đủ context và code chính xác
để thực hiện mà không cần đọc thêm tài liệu.

---

## Thứ tự thực thi bắt buộc

```
TASK-001 ──────────────────────────────────────────────────────── P0 (FIRST)
  │
  ├── TASK-002 (tests cho TASK-001)
  │
  ├── TASK-003 ──► TASK-004 (RateLimiter accessors + tests)
  │
  ├── TASK-003 ──► TASK-005 ──► TASK-006 ──► TASK-007
  │                (channels)   (main.go)    (tests)
  │
  └── TASK-001 ──► TASK-008 ──► TASK-009 (rpcTimeout + tests)
                      │
                      └──► TASK-010 (preflight tests)
                                │
                                └──► TASK-011 (full regression verify)
```

**TASK-001 phải chạy trước tất cả.** Các tasks còn lại có thể chạy song song
theo nhóm độc lập:
- **Nhóm A:** TASK-003 → TASK-004 (không phụ thuộc gì ngoài TASK-003)
- **Nhóm B:** TASK-003 → TASK-005 → TASK-006 → TASK-007
- **Nhóm C:** TASK-001 → TASK-008 → TASK-009 → TASK-010

---

## Task Index

| Task | Solution | Mô tả | Priority | File thay đổi |
|------|----------|-------|----------|---------------|
| [TASK-001](./TASK-001-fix-handler-context-separation.md) | SOL-001 | Tách `dispatchCtx`/`writeCtx` + thêm `writeTimeout` const + contention logging | **P0** | `wscompat/handler.go` |
| [TASK-002](./TASK-002-test-handler-context-separation.md) | SOL-001 | 3 tests cho handler context separation | P0 | `wscompat/handler_test.go` |
| [TASK-003](./TASK-003-add-ratelimiter-accessors.md) | SOL-002 | Thêm `RPS()` và `Burst()` methods vào `RateLimiter` | P1 | `usecase/rate_limit.go` |
| [TASK-004](./TASK-004-test-ratelimiter-accessors.md) | SOL-002 | 3 tests cho `RateLimiter` accessors | P1 | `usecase/rate_limit_test.go` |
| [TASK-005](./TASK-005-register-missing-channels.md) | SOL-002 | Đăng ký `crashReports.getLatestPending` + `rateLimits.get`, cập nhật `RegisterRealChannels` | P1 | `wscompat/channels.go` |
| [TASK-006](./TASK-006-update-main-register-channels.md) | SOL-002 | Truyền `rateLimiter` vào `RegisterRealChannels` trong main.go | P1 | `cmd/server/main.go` |
| [TASK-007](./TASK-007-test-missing-channels.md) | SOL-002 | 4 tests cho `crashReports` và `rateLimits` channels | P1 | `wscompat/channels_test.go` |
| [TASK-008](./TASK-008-add-rpc-timeout-channels.md) | SOL-003 | Thêm `rpcTimeout=8s` const + bọc gRPC calls trong 3 handlers | P1 | `wscompat/channels.go` |
| [TASK-009](./TASK-009-test-rpc-timeout.md) | SOL-003 | 4 tests cho `rpcTimeout` và fail-fast behavior | P1 | `wscompat/channels_test.go` |
| [TASK-010](./TASK-010-test-preflight-check.md) | SOL-004 | 2 tests cho `preflight.check` + cập nhật comment | P2 | `wscompat/channels.go` + `channels_test.go` |
| [TASK-011](./TASK-011-full-regression-verify.md) | All | Full regression verify — build + vet + test + race | P2 | *(chỉ chạy lệnh)* |

---

## Tổng quan files bị thay đổi

| File | Tasks | Loại thay đổi |
|------|-------|---------------|
| `internal/adapter/wscompat/handler.go` | 001 | `writeTimeout` const; `dispatchCtx`/`writeCtx` tách rời; `lockStart` contention log |
| `internal/adapter/wscompat/handler_test.go` | 002 | 3 tests mới |
| `internal/usecase/rate_limit.go` | 003 | `RPS()`, `Burst()` methods |
| `internal/usecase/rate_limit_test.go` | 004 | 3 tests mới |
| `internal/adapter/wscompat/channels.go` | 005, 008, 010 | `rpcTimeout` const; `registerCrashReportChannels`; `rateLimitInfo`/`rateLimitReader`/`registerRateLimitChannels`; per-RPC deadline trong 3 handlers; comment update |
| `cmd/server/main.go` | 006 | 1 dòng thay đổi (thêm `rateLimiter` arg) |
| `internal/adapter/wscompat/channels_test.go` | 007, 009, 010 | 10 tests mới |

---

## Quick start cho AI agent

Thực thi theo thứ tự:

```
1. Đọc và thực thi TASK-001
2. Thực thi TASK-002, TASK-003 (song song)
3. Thực thi TASK-004, TASK-005 (song song, sau TASK-003)
4. Thực thi TASK-006, TASK-007 (song song, sau TASK-005)
5. Thực thi TASK-008 (sau TASK-001)
6. Thực thi TASK-009, TASK-010 (song song, sau TASK-008)
7. Thực thi TASK-011 (cuối cùng)
```
