# Kế hoạch triển khai

Đoạn mô tả tự do ngoài vùng: `- [ ] T9.9 không phải task` và một khối JSON giả:

```json
{"phases":[{"title":"giả"}]}
```

<!-- orca:begin plan request=REQ-142 schema=1 -->
## PH-1 Nền tảng aiclient
> Giao gói `aiclient` độc lập, kiểm thử riêng.
- [ ] T1.1 Dựng client tương thích OpenAI [type=feature] [h=3] [ac=AC-AI-01,AC-AI-02] [labels=area:backend]
  Serves: FR-SYS-07
  Code: create `backend/shared/pkg/aiclient/client.go::Client`
  Verify: AC-AI-01 qua `go test ./aiclient/... -run TestClient`
  Done when: client gọi được nhà cung cấp và trả lời.
- [ ] T1.2 Chặn SSRF theo địa chỉ đã phân giải [type=feature] [h=2.5] [ac=AC-AI-07] [depends=T1.1]
  Serves: NFR-SE-05
  Code: create `backend/shared/pkg/aiclient/transport.go::NewHTTPClient`
  Verify: AC-AI-07 qua `go test ./aiclient/... -run TestSSRF`
  Done when: địa chỉ nội bộ bị chặn.
## PH-2 Dịch vụ hội thoại
> Nối AIService với aiclient.
- [ ] T2.1 Thêm AppendTurn và ListMessages [type=task] [h=4] [ac=AC-AI-01] [depends=T1.2]
  Serves: ENT-17
  Code: modify `backend/services/platform/system-setting/internal/usecase/ai/service.go::AIService`
  Verify: AC-AI-01 qua `go test ./...`
  Done when: lượt hội thoại được lưu và liệt kê.
<!-- orca:end plan -->

Phần cuối tự do.
