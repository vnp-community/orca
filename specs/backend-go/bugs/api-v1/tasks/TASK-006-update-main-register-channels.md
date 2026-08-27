# TASK-006: Cập nhật `cmd/server/main.go` — truyền `rateLimiter` vào `RegisterRealChannels`

**From Solution:** SOL-002 (Code Change 3)  
**Priority:** P1  
**Service:** `api-gateway`  
**File:** `services/api-gateway/cmd/server/main.go`  
**Depends on:** TASK-005 (cần `RegisterRealChannels` signature mới)  
**Status:** `[x]` DONE

---

## Context

Sau khi TASK-005 thêm tham số `rateLimits rateLimitReader` vào `RegisterRealChannels`,
call site trong `main.go` sẽ fail build. Task này fix call site đó.

---

## Thay đổi cần thực hiện

**File:** `services/api-gateway/cmd/server/main.go`

Tìm dòng (khoảng line 241):
```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient)
```

Thay bằng:
```go
wscompat.RegisterRealChannels(wsCompatRegistry, annotationClient, taskClient, gitClient, automationClient, infraFleetClient, rateLimiter)
```

> **Lưu ý:** `rateLimiter` đã được khai báo tại line 210:
> ```go
> rateLimiter := usecase.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
> ```
> Không cần tạo biến mới — chỉ truyền `rateLimiter` hiện có vào function call.

---

## Verify sau khi thay đổi

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go build ./...
go vet ./...
```

Expected: toàn bộ service build thành công.

---

## Full integration verify

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca/backend-go/services/api-gateway
go test ./... -count=1 -v 2>&1 | tail -20
```

Expected: tất cả tests pass.
