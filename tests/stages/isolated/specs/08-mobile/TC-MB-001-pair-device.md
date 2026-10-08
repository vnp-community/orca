# TC-MB-001 — Pair Mobile Device

**BL Reference:** BL-MB-01  
**Flow Reference:** docs/flows/logic/mobile-companion.md  
**Priority:** P0  
**Type:** Integration + Security  
**Actor:** Sam

---

## TC-MB-001-01: Pair device — QR code flow

**Priority:** P0

### Steps
1. Desktop app generate QR code (contains: WS endpoint + one-time pairing token)
2. Mobile app scan QR code
3. Mobile app connect WebSocket với pairing token
4. ECDH key exchange (TweetNaCl box)
5. Pairing confirmed

### Expected Results
- QR code displayed trong < 2s
- Mobile connect trong < 30s
- E2E encryption established: shared secret derived
- DB: pairing record stored
- Event: `mobile:paired { deviceId, deviceName }`

### Assertions
```
qrData = await ipc.invoke('mobile.generateQR')
assert qrData.token !== undefined
assert qrData.wsEndpoint !== undefined

// Simulate mobile scan + connect
mobileClient = simulateMobileScan(qrData)
assert mobileClient.connected === true
assert mobileClient.encryptionEstablished === true

event = await events.next('mobile:paired')
assert event.deviceId !== undefined
```

---

## TC-MB-001-02: Pairing token one-time use

**Priority:** P0  
**Security:** Token không thể dùng lại

### Steps
1. Generate QR code với token
2. Mobile scan và pair thành công
3. Thử pair lại với cùng token

### Expected Results
- Lần 2: Token rejected, connection closed

---

## TC-MB-001-03: Pairing timeout

**Priority:** P1

### Steps
1. Generate QR
2. Không scan trong 5 phút (timeout)
3. Thử dùng token sau timeout

### Expected Results
- Token expired, connection rejected

---

## TC-MB-001-04: TweetNaCl encryption — E2E verification

**Priority:** P0  
**Security:** CRITICAL

### Steps
1. Pair device
2. Desktop send message tới mobile
3. Intercept WS message
4. Verify ciphertext (not plaintext)

### Expected Results
- WS payload là ciphertext (không đọc được)
- Mobile decrypt thành công với shared secret
- Mỗi message có unique nonce

### Assertions
```
// Intercept WS
interceptedMessage = captureWsMessage(pairedConnection)
assert isNotPlaintext(interceptedMessage) // cannot see agent output in plaintext
assert interceptedMessage.nonce !== lastNonce // unique nonce per message

// Mobile decrypt
decrypted = tweetnacl.box.open(interceptedMessage.ciphertext, interceptedMessage.nonce, ...)
assert decrypted !== null // decrypt success
```

---

## TC-MB-001-05: Unpair device

**Priority:** P1

### Steps
1. Device paired
2. `mobile.unpair { deviceId }`
3. Mobile thử gửi message

### Expected Results
- Pairing record deleted
- Mobile WS connection closed
- Mobile message rejected

---

## TC-MB-001-06: Performance — Pair trong < 30s

**Priority:** P0  
**Type:** Performance

### Steps
1. Measure: generateQR → `mobile:paired` event

### Expected Results
- Duration < 30,000ms

