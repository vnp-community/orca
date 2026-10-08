# TC-DB-003 — Viewport Testing

**BL Reference:** BL-DB-03  
**Priority:** P1  
**Actor:** QA, Alex

---

## TC-DB-003-01: Switch viewport preset — Mobile

### Steps
1. `design.setViewport { preset: 'mobile-375' }`

### Expected Results
- Browser viewport: width=375, height=667
- Device scale factor: 2 (retina)

---

## TC-DB-003-02: Custom viewport

### Steps
1. `design.setViewport { width: 1920, height: 1080 }`

### Expected Results
- Viewport set to 1920×1080

---

## TC-DB-003-03: Viewport presets available

| Preset | Width | Height |
|--------|-------|--------|
| mobile-375 | 375 | 667 |
| tablet-768 | 768 | 1024 |
| desktop-1280 | 1280 | 800 |
| 4k | 3840 | 2160 |

