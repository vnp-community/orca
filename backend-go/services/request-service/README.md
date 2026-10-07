# Request Service

Manages engineering requests and approvals.

## Chạy local

Cần Docker cho cơ sở dữ liệu và NATS. Từ gốc `backend-go`:

```bash
make dev-up
go run ./services/request-service/cmd/server
```

## Biến môi trường

Service sẽ sử dụng config mặc định nếu không khai báo biến:
- `DATABASE_CREDENTIALS_FILE`: Đường dẫn file JSON chứa thông tin nối DB.
- `NATS_URL`: Địa chỉ kết nối NATS (mặc định: `nats://localhost:4222`).
- `TASK_SERVICE_ADDR`: Địa chỉ GRPC của task-service.
- `AI_PROVIDER_SERVICE_ADDR`: Địa chỉ GRPC của ai-provider-service.
- `PROJECT_SERVICE_ADDR`: Địa chỉ GRPC của project-service.
- `REQUEST_FLOW_ENABLED`: Cờ bật/tắt luồng request (mặc định `false`).

## Real vs Stub

| Component | Real | Stub |
| --- | --- | --- |
| `config` | `internal/config` | (none) |
