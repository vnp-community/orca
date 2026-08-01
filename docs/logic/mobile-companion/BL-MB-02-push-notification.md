# BL-MB-02 — Gửi Push Notification tới Mobile

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-MB-02 |
| **Tên** | Gửi Push Notification khi Agent có Sự kiện |
| **Nhóm** | Mobile Companion |
| **Actors** | Sam (Mobile-First User), Carlos |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F03 Mobile Companion, F11 |
| **SRS** | FR-5.2 |

---

## Mô tả nghiệp vụ

Khi agent hoàn thành, gặp lỗi, hoặc chờ input — desktop tự động gửi push notification về mobile trong < 5 giây.

---

## Sự kiện Trigger Notification

| Event | Notification Text | Priority |
|-------|------------------|---------|
| Agent completed | "✅ {agent} xong: {task-summary}" | High |
| Agent error | "❌ {agent} lỗi: {error-msg}" | High |
| Agent waiting | "⏸ {agent} chờ input bạn" | Medium |
| Rate limit | "⚠️ Rate limit: {provider}. Reset lúc {time}" | Medium |

---

## Luồng chính

```
1. Agent status change detected (BL-AG-05)
2. Check: có mobile paired không?
3. Format notification payload
4. Encrypt với shared secret (từ BL-MB-01)
5. Gửi qua WebSocket tới mobile
6. Nếu mobile online: receive và display
7. Nếu mobile offline: buffer, retry khi reconnect
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-MB-05 | Notification phải được encrypt — không plaintext trên network |
| BR-MB-06 | Delivery trong < 5 giây khi mobile đang connected |
| BR-MB-07 | Buffer tối đa 50 notifications khi mobile offline |
| BR-MB-08 | Notification settings có thể cấu hình per-event-type |

---

## SLO

| Metric | Target |
|--------|--------|
| Notification delivery (online) | < 5 giây |
| Delivery rate | > 99% |
