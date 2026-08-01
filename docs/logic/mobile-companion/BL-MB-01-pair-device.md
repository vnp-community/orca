# BL-MB-01 — Pair Mobile Device với Desktop

| Thuộc tính | Giá trị |
|-----------|---------|
| **Mã nghiệp vụ** | BL-MB-01 |
| **Tên** | Pair Mobile Device với Desktop App |
| **Nhóm** | Mobile Companion |
| **Actors** | Sam (Mobile-First User) |
| **Ưu tiên** | P0 — Must Have |
| **Tính năng** | F03 Mobile Companion |
| **SRS** | FR-5.1 |

---

## Mô tả nghiệp vụ

Kết nối Orca Mobile App với Orca Desktop App qua QR code — thiết lập kênh truyền thông tin bảo mật E2E giữa hai thiết bị.

---

## Luồng chính

```
1. Desktop: Settings → Mobile → "Show Pairing QR"
2. Desktop generate:
   a. Ephemeral key pair (TweetNaCl)
   b. Server address (local IP:port)
   c. One-time pairing token
   d. Encode thành QR code
3. QR code hiển thị (expire sau 5 phút)
4. Mobile: tap "Pair New Device" → scan QR
5. Mobile decode QR → có server address + token
6. Mobile gửi pairing request tới desktop với token
7. Desktop verify token
8. Key exchange: desktop ↔ mobile exchange public keys
9. Shared secret được derive (TweetNaCl box)
10. Pairing successful → connection established
11. Token bị invalidate (one-time use)
```

---

## Security Model

```
[Desktop]                    [Mobile]
generate keypair (Kd_pub, Kd_priv)
generate token T
                ─── QR (Kd_pub, server, T) ──→
                ←── pairing req (Km_pub, T) ───
verify T
derive shared = box(Kd_priv, Km_pub)
                ─── pairing ack ──────────────→
                derive shared = box(Km_priv, Kd_pub)
[All subsequent messages encrypted with shared secret]
```

---

## Business Rules

| Rule | Mô tả |
|------|-------|
| BR-MB-01 | QR code expire sau 5 phút — ngăn replay attack |
| BR-MB-02 | Token là one-time use — không thể pair lại với cùng QR |
| BR-MB-03 | Chỉ có thể pair tối đa 3 mobile devices với 1 desktop |
| BR-MB-04 | Unpair xóa shared secret — không thể reconnect mà không pair lại |

---

## SLO

| Metric | Target |
|--------|--------|
| Pairing time | < 30 giây |
| QR scan success rate | > 99% |
