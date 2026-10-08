# TC-CLI-003 — Chạy Orca Headless Mode

**BL Reference:** BL-CLI-03  
**Priority:** P1  
**Actor:** DevOps

---

## TC-CLI-003-01: `orca serve` — Start headless server

### Steps
1. `orca serve --port 6769 --multi-user`

### Expected Results
- Server started on port 6769
- HTTP :6769, WS :6768
- Health: `GET /health/ready` → 200

---

## TC-CLI-003-02: Headless mode — No GUI required

### Steps
1. Start on headless Linux (no DISPLAY)
2. Server functions normally

### Expected Results
- Server starts without X11/DISPLAY
- All API endpoints functional

