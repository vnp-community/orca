# Luồng Dữ liệu — Design & Browser

**Domain:** Design & Browser  
**Nghiệp vụ:** BL-DB-01 → BL-DB-03  
**Kiến trúc tham chiếu:** HLD v1 — Main Process, Embedded Browser (Electron WebContents)

---

## Thành phần tham gia

| Thành phần | Layer | Vai trò |
|------------|-------|---------|
| Renderer (React UI) | UI | Design mode toolbar, annotation overlay |
| Embedded Browser (Electron) | Runtime | BrowserView / WebContentsView hiển thị app |
| Main Process | Business Logic | DesignModeManager, BrowserCapture |
| Daemon / PTY / Agent | External | Nhận context từ UI element |
| CDP (Chrome DevTools Protocol) | Debug Protocol | Element inspection, screenshot |

---

## BL-DB-01 — Capture UI Element

```
Người dùng (Alex/QA)
    │
    ▼
[Renderer] bật "Design Mode" → di chuột trên embedded browser
    │
    ▼
[Main Process — DesignModeManager.enable()]
    ├─ Inject script vào WebContents:
    │   window.addEventListener('mousemove', highlightElement)
    │   window.addEventListener('click', captureElement)
    ├─ Overlay: highlight border cho element đang hover
    │
    ▼
User click element trong embedded browser
    │ WebContents event: did-click
    ▼
[Main Process — BrowserCapture.captureElement()]
    ├─ CDP: Runtime.evaluate → document.elementFromPoint(x, y)
    ├─ CDP: DOM.getOuterHTML(nodeId) → element HTML
    ├─ CDP: CSS.getComputedStyleForNode → computed CSS
    ├─ CDP: Page.captureScreenshot { clip: { x, y, width, height } }
    ├─ Build ElementContext:
    │   { tagName, id, class, outerHTML, computedCSS, screenshotBase64, xpath }
    └─ Store in memory (current capture context)
    │
    ▼
[Renderer] hiển thị captured element info panel (properties, screenshot)

Luồng:
User hover/click → WebContents → Main (CDP queries)
                              → element HTML + CSS + screenshot
                              → Renderer (element panel)
```

---

## BL-DB-02 — Inject UI Context vào Agent

```
Người dùng (Alex/QA) — sau BL-DB-01
    │
    ▼
[Renderer] click "Send to Agent" trong Design panel
    │ contextBridge.invoke('design.injectContext', { sessionId, elementContext })
    ▼
[Main Process — DesignModeManager.injectToAgent()]
    ├─ Format context message:
    │   "UI Element captured:\n"
    │   "  Tag: <div class='login-form'>\n"
    │   "  Computed CSS: { display: flex, gap: 16px... }\n"
    │   "  Screenshot: [attached]\n"
    │   "Task: Fix the alignment issue in this component"
    ├─ Encode screenshot as base64 (nếu agent hỗ trợ image input)
    └─ Daemon.writeToPty(sessionId, contextMessage)  ← Unix Socket
    │
    ▼
[Agent Process] nhận element context → phân tích → generate fix

Luồng:
User → Renderer → IPC → Main (format context)
                       → Unix Socket → Daemon → PTY stdin → Agent
```

---

## BL-DB-03 — Viewport Testing

```
Người dùng (QA/Alex)
    │
    ▼
[Renderer] Design Mode → Viewport selector: Mobile / Tablet / Desktop / Custom
    │ contextBridge.invoke('browser.setViewport', { width, height, devicePixelRatio })
    ▼
[Main Process — BrowserManager.setViewport()]
    ├─ webContents.setSize(width, height)
    ├─ CDP: Emulation.setDeviceMetricsOverride
    │   { width, height, deviceScaleFactor, mobile: true/false }
    ├─ CDP: Emulation.setUserAgentOverride (mobile UA nếu cần)
    └─ emit: viewport:changed { width, height }
    │
    ▼
[Renderer] embedded browser re-renders ở viewport mới
    └─ Screenshot button → BL-DB-01 để capture lại

Automation flow (QA run matrix):
[Main] FOR each viewport in [320, 768, 1280, 1920]:
    browser.setViewport(viewport)
    CDP.captureScreenshot() → save to file
    → compare với baseline

Luồng:
User → Renderer → IPC → Main → CDP (setDeviceMetrics)
                              → WebContents re-render
                              → Renderer (viewport indicator update)
```

---

## Sơ đồ tổng quan — Design & Browser

```
┌─────────────────────┐   IPC   ┌──────────────────────────────────┐
│  Renderer           │◄───────►│  Main Process                    │
│  Design Mode UI     │         │  DesignModeManager               │
│  Element Panel      │         │  BrowserCapture                  │
│  Viewport Selector  │         │  BrowserManager                  │
└─────────────────────┘         └──────────┬───────────────────────┘
                                           │ CDP
                                 ┌─────────▼──────────────────────┐
                                 │  Embedded Browser               │
                                 │  Electron WebContentsView       │
                                 │  (User's web app)               │
                                 │                                 │
                                 │  CDP endpoints:                 │
                                 │  - DOM.getOuterHTML             │
                                 │  - CSS.getComputedStyle         │
                                 │  - Page.captureScreenshot       │
                                 │  - Emulation.setDeviceMetrics   │
                                 └─────────────────────────────────┘
                                           │ context inject
                                 ┌─────────▼──────────────────────┐
                                 │  Daemon → PTY → Agent Process  │
                                 │  (receive UI context)           │
                                 └────────────────────────────────┘
```
