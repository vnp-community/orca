# Mobile

**Route / trigger:** `activeView === 'mobile'`. Opened via the sidebar "Mobile" button (`frontend/src/renderer/src/components/sidebar/SidebarNav.tsx`, `Smartphone` icon, calls `openMobilePage()` and dismisses the onboarding badge). Shown by default; hideable per-user via `settings.showMobileButton` (right-click "Hide" menu, `shouldShowMobileButton`).
**Top-level component:** `MobilePage` (`frontend/src/renderer/src/components/mobile/MobilePage.tsx`), rendering via `MobilePageContent` (`MobilePageContent.tsx`)

## Purpose
Onboarding and device-pairing hub for the "Orca Mobile" companion app: install the mobile app, pair a phone/tablet with this desktop instance over the LAN/Tailscale via a scannable QR code, and manage already-paired devices.

## Layout
`MobilePage` is a state machine over one of three `MobilePageStage`s (`mobile-page-stage.ts`: `'intro' | 'paired' | 'flow'`), rendered inside a two-column hero (`MobilePageContent.tsx`):
```
┌ MobilePageToolbar (close, hide-sidebar-button toggle) ─────────────────┐
├ mp-hero ─────────────────────────────────────────────┬─────────────────┤
│ mp-hero-copy (stage-dependent):                       │ mp-stage:       │
│  intro  → HeroIntro ("Get started" CTA)                │  PhoneCarousel  │
│  paired → HeroPaired (list of PairedDevice, revoke,     │  (illustrative  │
│           "Pair another device")                       │   phone mockup) │
│  flow   → HeroFlow, a 2-step wizard:                    │                 │
│    Step 1: platform toggle (iOS/Android [+ iOS channel: │                 │
│            Preview/Stable]) + install QR + install link │                 │
│    Step 2: pairing QR + network-interface picker +      │                 │
│            "Copy pairing code" + Windows-firewall notice│                 │
└────────────────────────────────────────────────────────┴─────────────────┘
```
Key child components: `MobileHeroIntro.tsx` (`HeroIntro`), `MobileHeroPairedDevices.tsx` (`HeroPaired`, `PairedDevice` type), `MobileHero.tsx` (`HeroFlow`, platform/channel toggles), `NetworkInterfacePicker.tsx`, `WindowsFirewallNotice.tsx`, `PhoneCarousel.tsx`, `MobilePageToolbar.tsx`.

## Data shown
- **Stage**: local `stage: FlowStage | null` (`'intro'|'paired'|'flow'`) and `stepIdx: 0|1`, chosen on mount by checking whether any devices are already paired (`shouldShowPairedAfterDeviceRefresh` from `mobile-page-stage.ts`).
- **Paired devices**: `devices: PairedDevice[]` via `window.api.mobile.listDevices()` — `{ deviceId, name, pairedAt, lastSeenAt }` (type re-exported from `MobileHeroPairedDevices.tsx` / duplicated in `settings/MobilePairedDevicesSection.tsx` for the Settings-pane variant of the same list). Polled while on the pairing step or the paired view via `useMobilePairingDevicePolling`, so a phone that finishes pairing appears without a manual refresh.
- **Pairing QR**: `pairQrDataUrl`/`pairingUrl` via `window.api.mobile.getPairingQR({ address?, rotate? })`; regenerated when the network address changes or the user hits Regenerate.
- **Install QR**: `installQrUrl` via `useMobileInstallQr(stage, platform, iosChannel)` (`use-mobile-install-qr.ts`) — encodes the install link from `getInstallCopy(platform, iosChannel)` (`mobile-platform-copy.ts`): App Store / TestFlight links for iOS (`preview` vs `stable` channel), a GitHub-releases APK URL for Android.
- **Network interfaces**: `networkInterfaces: MobileNetworkInterface[]` via `window.api.mobile.listNetworkInterfaces()`, plus `selectedAddress`/`addressIsManual` (tracks whether the address is OS-enumerated or hand-typed, so refreshes don't clobber a manual Tailscale/static entry — see `selectRefreshedNetworkAddress`).
- **Sidebar toggle**: `settings.showMobileButton`, read/written via `updateSettings`.

## Key interactions
- **Get started** (intro) → enters the 2-step `flow`.
- **Step 1**: switch platform (iOS/Android); for iOS, switch release channel (Preview/TestFlight vs Stable/App Store); "Open App Store/TestFlight" or "Copy install link"; Continue to Step 2.
- **Step 2**: pick/refresh a network interface for the QR to encode; "Copy pairing code" (writes `pairingUrl` to clipboard); "Regenerate code" (rotates the pairing token, `generatePairing(true)`); Back/Continue/Done navigation — Done is only shown once at least one device is paired, jumping to the `paired` stage.
- **Paired view**: revoke a device (`window.api.mobile.revokeDevice({deviceId})`, deduped against double-clicks via `revokingDeviceIds`) — revoking the last device returns to `intro`; "Pair another device" jumps directly to Step 2 (skipping Step 1, since the app is presumably already installed).
- **Toolbar**: close the page (`closeMobilePage`), or hide the Mobile sidebar button going forward (`toggleMobileSidebarButton`, shows a toast pointing to Settings > Mobile to re-enable).

## Notable implementation details / known issues
- Device-count polling logic is deliberately baseline-relative (`deviceCountAtPairStart` vs. `currentDeviceCount`) rather than just "count > 0", so that entering Step 2 with devices already paired doesn't immediately auto-jump to the paired view — only a *new* pairing during this session does.
- Re-entering the flow (`enterFlow`/`pairAnotherDevice`) explicitly clears `pairQrDataUrl`/`pairingUrl` and resets `hasGeneratedRef` so a stale/expired QR is never flashed from a previous session before the new one is minted.
- iOS ships two install tracks (weekly App Store vs. daily TestFlight); Android ships one APK track only — this asymmetry is hardcoded in `mobile-platform-copy.ts` and surfaces as the channel toggle only appearing for `platform === 'ios'`.
- A `PairedDevice` type is defined independently in two places (`MobileHeroPairedDevices.tsx` for this page, `settings/MobilePairedDevicesSection.tsx` for the Settings > Mobile pane's device list) — same shape, not shared, worth consolidating if touched.
