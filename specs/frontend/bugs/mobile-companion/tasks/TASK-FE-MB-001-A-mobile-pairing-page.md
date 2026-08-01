# TASK-FE-MB-001-A: `MobilePairingPage` — QR code + pair code flow (BL-MB-01)

**Domain:** mobile-companion  
**Solution Ref:** SOL-FE-MB-001 §Component 1  
**Bug:** BUG-FE-MB-001  
**Priority:** 🔴 P0  
**Estimated:** 60 phút  
**Status:** ✅ DONE — Implemented

---

## Mục tiêu

Tạo `MobilePairingPage` — trang chính để pair mobile device qua QR code hoặc pair code.

---

## Files cần tạo

- **TẠO MỚI:** `src/renderer/src/components/mobile/MobilePairingPage.tsx`

---

## Các bước thực thi

Tạo file với nội dung từ SOL-FE-MB-001 §Component 1:

### Layout (2 tabs: QR Code | Manual Code)

```
[Mobile App ─ Pair Your Phone]

  Tab: [📷 Scan QR] [🔢 Enter Code]

  QR Tab:
  ┌─────────────────────┐
  │  [QR CODE 200x200]  │  ← <QrCode value={pairUrl} size={200} />
  │                     │
  │  Expires in: 05:42  │
  └─────────────────────┘
  [Refresh QR] button

  Code Tab:
  ┌──────────────────────┐
  │  PAIR CODE: ABC-123  │
  │  [Copy]              │
  └──────────────────────┘
  [Generate New Code]
```

### Logic

```typescript
// State
const [pairCode, setPairCode] = useState<string | null>(null)
const [pairUrl, setPairUrl] = useState<string | null>(null)
const [expiresAt, setExpiresAt] = useState<number | null>(null)
const [activeTab, setActiveTab] = useState<'qr' | 'code'>('qr')

// Generate pair session
async function generatePairSession() {
  const result = await window.api.mobile.createPairSession()
  setPairCode(result.pairCode)
  setPairUrl(result.pairUrl)
  setExpiresAt(Date.now() + result.expiresInSeconds * 1000)
}

useEffect(() => { generatePairSession() }, [])
```

### Countdown timer display

```typescript
// Hook tính remaining time
function useCountdown(expiresAt: number | null) {
  const [remaining, setRemaining] = useState(0)
  useEffect(() => {
    if (!expiresAt) return
    const tick = () => setRemaining(Math.max(0, Math.floor((expiresAt - Date.now()) / 1000)))
    tick()
    const id = setInterval(tick, 1000)
    return () => clearInterval(id)
  }, [expiresAt])
  return remaining
}
// Display: mm:ss format
const mm = String(Math.floor(remaining / 60)).padStart(2, '0')
const ss = String(remaining % 60).padStart(2, '0')
```

### Dependencies cần kiểm tra

```bash
# QR code library:
grep "qrcode\|qr-code\|react-qr" package.json
# Nếu không có:
# npm install qrcode.react
```

---

## Verify

```bash
grep -n "MobilePairingPage\|createPairSession\|QrCode\|qrcode" \
  src/renderer/src/components/mobile/MobilePairingPage.tsx
```

## Depends on
Không có

## Blocking
TASK-FE-MB-001-C (MobileCompanionPage assembly)
