# TASK-BIGFILE-054 — Investigate: Terminal/PTY/Agent-status core (còn lại sau 23 task)

**Loại:** Investigate · **Effort:** L · **Phụ thuộc:** 8, 9, 35, 41
(field-span methodology giống hệt TASK-BIGFILE-035/041)
**Status:** ✅ (sinh task 055+)
**Solution:** `../solutions/SOLUTION-FE-BIGFILE-002-orca-runtime.md`

## Bối cảnh

Sau 23 task (036–053), `orca-runtime.ts` còn **10,760 dòng** (giảm 59.8% từ
26,730). Phần còn lại (dòng ~2,097–10,037, ~8,000 dòng, ~180 method) là
đúng như comment grandfathered ở đầu file mô tả: *"OrcaRuntimeService still
owns the mutable live graph, PTY handles, waiters, mobile floor/layout
state, and managed-worktree reconciliation... the remaining split points
need state-owner extraction before enforcing max-lines."*

Người dùng đã chấp nhận rủi ro cao để khảo sát tiếp vùng này (khác PTY-
lifecycle-core ở TASK-049, vùng này LỚN hơn và LÕI hơn nhiều — waiters,
handle issuance, headless terminal buffer, agent-status OSC parsing, title
tracking — hầu hết đều dùng `this.graph` trực tiếp).

## Phương pháp

Giống TASK-035/041: đo **field-span** (dòng nhỏ nhất/lớn nhất mỗi private
field được `this.fieldName` tham chiếu) thay vì đọc tuyến tính. Field có
span hẹp (không lan ra `pruneDisconnectedPtyTranscript`/
`pruneDisconnectedPtyRecords`, 2 method dọn dẹp chung đọc/xoá gần như MỌI
field per-PTY khi ngắt kết nối) → domain cô lập, an toàn để Move. Field có
span rộng (hầu hết do bị 2 method dọn dẹp trên chạm vào) → lõi thật, KHÔNG
tách được bằng Move.

## Kết quả: field-span đo được (28 field cốt lõi)

| Field | Span (dòng) | Kết luận |
|---|---|---|
| `graph` (RuntimeGraphStore) | toàn bộ file | LÕI — đã tách state riêng ở TASK-041, không tách thêm được |
| `headlessTerminals` | 1306–3761 (~2,455, nhưng dùng thật chỉ 2097–3761) | LÕI — headless terminal buffer dùng bởi cả `createTerminal`/`seedHeadlessTerminal` |
| `agentStatusOscProcessorsByPtyId`, `oscTitleScanTailByPtyId`, `osc7ScanTailByPtyId`, `terminalCwdByPtyId`, `terminalFileUriHostnameByPtyId`, `terminalSpawnCommandsByPtyId`, `ptyOutputSequenceById`, `recentPtyPathCandidatesById`, `latestAgentStatusByPaneKey`, `recentPtyOutputById` | 4,800–6,600 dòng (lan tới `pruneDisconnectedPtyTranscript`/`Records` ở dòng ~8793–8804) | LÕI — cùng bị 2 method dọn dẹp chung chạm tới, không tách rời được mà không refactor lại chính 2 method đó (rủi ro cao, ngoài phạm vi Move thuần) |
| `dataListeners` | 2400–3113 (713) | trung bình — gắn liền `subscribeToTerminalData`/dữ liệu PTY, không cô lập |
| `remoteTerminalViewSubscriberCounts` | 3135–4478 (1,343) | trung bình — bị `onPtyExit` dọn dẹp, không cô lập tuyệt đối nhưng cụm method riêng vẫn liền mạch (xem 061 dưới) |
| **`ptyTitleTrackersByPtyId`** | 2608–2929 (**321**) | ✅ AN TOÀN — cụm `getTrackedRawTitleForPty`/`makeMobileTitleGateKey`/`getOrCreatePtyTitleTrackerEntry`/`applyTrackedPtyTitle`/`disposePtyTitleTracker` (2697–2932) |
| **`waitBlockedCheckStateByPtyId`** | 2415–2485 (**70**) | ✅ AN TOÀN — `scheduleWaitBlockedCheck`/`runWaitBlockedCheck`/`clearWaitBlockedCheckState` (2414–2488) |
| **`ptyForegroundAgentRefreshes`, `ptyDelayedForegroundSnapshotTitleObservations`** | 5309–5373 (**64**) | ✅ AN TOÀN — `refreshPtyForegroundAgent`/`getPendingForegroundAgentRefreshForTitle`/`delayPtyBackedMobileSnapshotForForegroundAgent`/`refreshPtyForegroundAgentFromController`/`loadPtyForegroundAgentFromController` (5301–5412) |
| **`messageWaitersByHandle`** | 9349–9428 (**79**) | ✅ AN TOÀN — `deliverPendingMessagesForHandle`/`notifyMessageArrived`/`waitForMessage`/`resolveMessageWaiter`/`removeMessageWaiter` (9333–9432) |
| **`subscriptionCleanups`, `subscriptionsByConnection`, `subscriptionConnectionByEntry`** | 3872–3919 (**~50**) | ✅ AN TOÀN — `registerSubscriptionCleanup`/`cleanupSubscription`/`cleanupSubscriptionsByPrefix`/`cleanupSubscriptionsForConnection` (3863–3933) |
| **`notificationListeners`** | 3935–3974 (**~40**) | ✅ AN TOÀN — `onNotificationDispatched`/`getMobileNotificationListenerCount`/`dispatchMobileNotification`/`dismissMobileNotification` (3934–3974), cùng cụm với `subscription*` ở trên (đứng liền kề, gộp chung 1 file hợp lý) |
| **`fitOverrideListeners`** | 3171–3180 (**9**) | ✅ AN TOÀN nhưng quá nhỏ đứng riêng — `subscribeToFitOverrideChanges`/`notifyFitOverrideListeners` (3163–3188), gộp cùng cụm subscription/notification ở trên |
| **`mobileDictation`** | 1241, 4126–4295 (**~300 dòng method, field cô lập hoàn toàn**) | ✅ AN TOÀN, đã xác minh kỹ (xem TASK-BIGFILE-055) — `listMobileSpeechModels`…`cancelMobileDictationForClient` (3993–4295), chỉ phụ thuộc `this.store` |
| **`accountServices`** | 1237, 3975–4354 | ✅ AN TOÀN, nhỏ (59 dòng: `requireAccountServices`…`onAccountsChanged`) — có thể gộp cùng `commitMessageAgentEnv` (setter liền kề) thành 1 file "account-services" nhỏ |
| **`activeBrowserScreencastsByConnection`/`ByPage`** | 99 / 5,999 (`ByPage` bị `onPtyExit`/wiring chạm rải rác) | KHÔNG an toàn tách rời — gắn liền method đơn lẻ khổng lồ `browserScreencast` (10237–10649, 412 dòng, 1 method) |

## Method đơn lẻ khổng lồ: `browserScreencast` (10237–10649, 412 dòng)

KHÔNG phải cụm nhiều method — 1 method duy nhất. Không phù hợp pattern
Move (composition tách nhiều method liên quan) đã dùng xuyên suốt 23 task
trước. Cần 1 task **Investigate riêng** (đọc kỹ, tách thành các hàm private
nội bộ nhỏ hơn trước, rồi mới cân nhắc Move) — không sinh task Move ở đây.

## Task Move được sinh ra (an toàn, ranh giới rõ)

| # | Domain | Method | Dòng gốc | Field |
|---|---|---|---|---|
| 055 | mobile-dictation | 11 method | 3993–4295 (~303) | `mobileDictation` |
| 056 | account-services | 9 method | 3975–4354 (~380, nhiều dòng trắng/comment) | `accountServices`, `commitMessageAgentEnv` |
| 057 | pty-title-tracker | 5 method | 2697–2932 (~235) | `ptyTitleTrackersByPtyId` |
| 058 | connection-subscription-notify | 8 method (`subscription*` + `notification*` + `fitOverride*`) | 3863–3974, 3163–3188 (không liền mạch — 2 đoạn) | `subscriptionCleanups`, `subscriptionsByConnection`, `subscriptionConnectionByEntry`, `notificationListeners`, `fitOverrideListeners` |
| 059 | pty-wait-blocked-check | 3 method | 2414–2488 (~74) | `waitBlockedCheckStateByPtyId` |
| 060 | pty-foreground-agent-refresh | 5 method | 5301–5412 (~110) | `ptyForegroundAgentRefreshes`, `ptyDelayedForegroundSnapshotTitleObservations` |
| 061 | terminal-message-waiter | 5 method | 9333–9432 (~110) | `messageWaitersByHandle` |
| 062 | remote-terminal-view-subscriber | 3 method | 3122–3188 (~66, cạnh fitOverride — cân nhắc gộp 058) | `remoteTerminalViewSubscriberCounts` (bị `onPtyExit` chạm — cần forwarding cẩn thận) |

**Không sinh task cho**: `graph`, `headlessTerminals`, cụm 10 field OSC/
transcript-per-PTY (agentStatusOscProcessorsByPtyId và 9 field liền kề) —
đây là lõi thật, cần thiết kế state-owner riêng (như `RuntimeGraphStore` ở
TASK-041) trước khi Move được, ngoài phạm vi Investigate này.

## Rủi ro chung cho nhóm task 055–062

Tất cả vẫn nằm trong vùng "PTY-lifecycle core" theo nghĩa rộng (cùng file,
cùng class, zero test coverage) nhưng **field/method của từng cụm đã xác
nhận cô lập** qua field-span — rủi ro thực tế thấp hơn nhiều so với
`createTerminal`/waiter chính/handle issuance (những thứ KHÔNG nằm trong
danh sách trên). Áp dụng đúng quy trình đã dùng cho 23 task trước: kiểm tra
"private method gọi từ nơi khác" bằng `tsc` lặp, không chỉ dựa vào phân
tích tĩnh.
