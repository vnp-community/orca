# Settings

**Route / trigger:** `activeView === 'settings'` in the Zustand app store (`frontend/src/renderer/src/store/`). Reached via `openSettingsPage()` / `setActiveView('settings')` — clicked from the sidebar's "Settings" nav button, the Cmd+J command palette ("Open Settings" or a specific section via `openSettingsTarget()`), or deep-linked from many in-app affordances (notification permission cards, browser toolbar menu, link-routing dialog, onboarding, source-control AI action dialogs, etc. — see `Settings`'s 26+ call sites).
**Top-level component:** `Settings` — `frontend/src/renderer/src/components/settings/Settings.tsx:271` (~1720 lines), lazy-loaded from `App.tsx` (`const Settings = lazy(() => import('./components/settings/Settings'))`) and rendered when `activeView === 'settings'` (`App.tsx:2453`).

## Purpose
The single settings surface for the whole app — every user-configurable behavior (agents, git, terminal, appearance, shortcuts, remote hosts, privacy, experimental flags, and per-repository overrides) lives here. It also doubles as an in-app search index: the same section/entry metadata backs the Cmd+J palette's settings search.

## Layout
Two-column shell: a fixed-width left navigation sidebar and a scrollable content column that stacks every section (only the active one is actually visible/mounted).

```
┌─────────────────────┬───────────────────────────────────────────┐
│ SettingsSidebar      │  scrollable content column                │
│ - Back button        │  (max-w-4xl, centered)                    │
│ - Search input       │                                            │
│ - Grouped nav list:  │  <ActiveSettingsSectionProvider>           │
│   AI Capabilities    │    <SettingsSection id="agents">…</>       │
│   Set Up             │    <SettingsSection id="accounts">…</>     │
│   Workflows          │    <SettingsSection id="orchestration">…</>│
│   Interface          │    … one <SettingsSection> per nav entry   │
│   Remote Hosts       │    (only the section matching              │
│   Privacy & Security │     activeSectionId renders its Pane;      │
│   Advanced           │     during search, every matching section  │
│   Experimental       │     is listed in the sidebar but only the  │
│   Repositories (1/repo) │  selected match's Pane content mounts)  │
└─────────────────────┴───────────────────────────────────────────┘
```

- **Left — `SettingsSidebar`** (`components/settings/SettingsSidebar.tsx`). Renders a back button (`onBack` → `closeSettingsPageWithPromptGuard`, which guards on unsaved Git AI Author prompt drafts), a search input (`searchQuery`/`onSearchChange`, bound to `settingsSearchInputQuery`/`setSettingsSearchQuery`), and the grouped nav list built from `generalGroups` (`NavGroup[]`, one per `SETTINGS_NAV_GROUPS` entry: `capabilities`, `setup`, `workflows`, `interface`, `remote`, `security`, `advanced`, `experimental`) plus a synthetic `repositories` group (`repoSections`, one row per repo, badge-colored, with remote/upstream icons). Clicking a row calls `onSelectSection(sectionId, modifiers)` → `scrollToSection`, which sets `activeSectionId` and scroll-jumps the content column.
- **Right — content column.** A single scrollable `<div>` (`contentScrollRef`) containing one `SettingsSection` (`components/settings/SettingsSection.tsx`) per nav entry, all declared unconditionally in JSX but each individually deciding whether to render via `ActiveSettingsSectionContext` (`activeSectionId`) and search-query matching (`matchesSettingsSearch` against that section's `searchEntries`). Sections not currently selected (and not query-matching) return `null` from `SettingsSection` — they don't even mount their inner Pane.
- Each section wraps one "Pane" component, e.g. `AgentsPane`, `AccountsPane`, `OrchestrationPane`, `ComputerUsePane`, `VoicePane`, `SettingsSetupGuidePane`, `GeneralPane`, `IntegrationsPane`, `MobileSettingsPane`, `GitPane` (+ nested `CommitMessageAiPane`, `GitProviderApiBudgetPane`), `TasksPane`, `TerminalPane`, `QuickCommandsPane`, `BrowserPane`, `MobileEmulatorSettingsPane`, `FloatingWorkspacePane`, `AppearancePane`, `InputPane`, `NotificationsPane`, `ShortcutsPane`, `StatsPane` (`components/stats/StatsPane.tsx`), `SshPane`, `RuntimeEnvironmentsPane` (servers), `DevServerPane`, `DeveloperPermissionsPane`, `PrivacyPane`, `AdvancedPane`, `DevToolsPane` (dev builds only), `ExperimentalPane`, and one `RepositoryPane` per repo — all under `frontend/src/renderer/src/components/settings/`.
- Section *panes* are lazily mounted, not lazily code-split: `mountedSectionIds`/`neededSectionIds` (derived in `settings-load-performance.ts`'s `deriveNeededSectionIds`/`getInitialMountedSectionIds`) track which section IDs have ever been visited; only `general` is eager-mounted on first open, others mount their Pane on first visit and then stay mounted (`isSectionMounted(sectionId)` gate around each Pane) so revisiting a section doesn't refetch/rebuild it, while sections never opened stay entirely unmounted.
- Loading state: while `settings` is still `null` (not yet fetched), the whole page renders a centered "Loading settings..." placeholder instead of the sidebar/content shell.
- Escape key closes the page (guarded — see Notable implementation details); `settings.search` keybinding focuses the search input.

## Data shown
- **Everything reads/writes through `settings: GlobalSettings | null`** — `useAppStore((s) => s.settings)` — and `updateSettings: (patch: Partial<GlobalSettings>) => Promise<void>` — `useAppStore((s) => s.updateSettings)`. `GlobalSettings` is defined in `frontend/src/shared/types.ts`; `fetchSettings()`/`fetchKeybindings()` hydrate it and `keybindings` on mount.
- **Nav section metadata** comes from `useSettingsNavigationMetadata()` (`frontend/src/renderer/src/hooks/useSettingsNavigationMetadata.ts:568`), a hook shared with the Cmd+J palette (`WorktreeJumpPalette.tsx`) so search results and sidebar visibility never drift apart. It calls `buildSettingsNavigationMetadata({ isMac, isWindows, isWindowsTerminalHost, isWebClient, isDev, repos })` (same file, line 110) which returns the ordered `SettingsNavSection[]` — id, title, description, icon, `searchEntries`, `group`, optional `badge`. Desktop-only sections (`computer-use`, `voice`, `mobile`, `browser`, `mobile-emulator`, `notifications`, `ssh`, `developer-permissions`, `advanced`, `dev`) are omitted entirely on the web client (`isWebClientLocation()`); `dev` is further gated to `import.meta.env.DEV`. One extra section per repo (`repo-${repo.id}`) is appended from `s.repos`.
- **Search** (`settingsSearchQuery`/`settingsSearchInputQuery`, `setSettingsSearchQuery`) ranks sections via `rankSettingsSearchItems` (`components/settings/settings-search.ts`), scoring each section's `searchEntries` (title/description/keyword hits) — used both to filter the sidebar's visible sections/groups and to gate which `SettingsSection` is query-matched.
- **Per-section supplementary state**: `keybindings` (shortcuts pane), `repos`/`projects`/`projectHostSetups` (`RepositoryPane`, general/git panes), `modelStates`/`refreshModelStates` (Voice pane's speech-model readiness), `windowsTerminalCapabilities` (`useWindowsTerminalCapabilities`, gates WSL-related rows), `orchestrationSkill`/`computerUseSkill` (`useInstalledAgentSkill`, drives the "install status" badge — `checking`/`installed`/`install` — shown next to the Orchestration and Computer Use nav rows), `repoHooksMap` (per-repo Orca hooks install state, fetched via `checkRuntimeHooks`).
- **Git AI Author drafts** (commit-message/branch-name AI prompts) are tracked as local unsaved-changes state (`hasUnsavedCommitPromptChanges`, `hasUnsavedBranchPromptChanges`) rather than in the store directly, written through `writeSourceControlAiSettings` which patches `settings.sourceControlAi`.

### Section id → what it controls

| Section id | Group | Controls |
|---|---|---|
| `agents` | capabilities | Default AI agent, custom commands, agent runtime config |
| `accounts` | capabilities | AI provider account switching/usage (Claude, Codex, Gemini, OpenCode Go, MiniMax, Grok) |
| `orchestration` | capabilities | Multi-agent orchestration skill install/config |
| `computer-use` | capabilities (desktop only) | Computer-use skill (agents controlling other apps) |
| `voice` | capabilities (desktop only) | Local speech-to-text dictation models |
| `setup-guide` | setup | Onboarding checklist |
| `general` | setup | Workspace defaults, app setup, maintenance (eager-mounted) |
| `integrations` | setup | GitHub/GitLab/Linear/source-hosting connections |
| `mobile` | setup (desktop only) | Orca Mobile pairing |
| `git` | workflows | Branch naming, base refs, attribution, Git AI Author (nested `CommitMessageAiPane`/`GitProviderApiBudgetPane`) |
| `tasks` | workflows | Which task providers show on the Tasks page/sidebar |
| `terminal` | workflows | Shell, renderer, sessions, terminal behavior |
| `quick-commands` | workflows | Saved terminal commands (global/per-project) |
| `browser` | workflows (desktop only) | In-app browser home page, link routing, cookies |
| `mobile-emulator` | workflows (desktop only) | Mobile emulator support |
| `floating-workspace` | workflows | Global floating terminal/browser/markdown tabs |
| `appearance` | interface | Theme, zoom, app/terminal appearance, sidebars, status bar |
| `input` | interface | Selection/editing behavior |
| `notifications` | interface (desktop only) | Native desktop notifications |
| `shortcuts` | interface | Keyboard shortcuts |
| `stats` | interface | Orca + Claude/Codex/OpenCode token analytics, Grok usage |
| `ssh` | remote (desktop only) | SSH host connections |
| `servers` | remote | Remote Orca server pairing (badge: Beta) |
| `dev-servers` | remote | Remote dev-machine connections for agent execution |
| `developer-permissions` | security (desktop, macOS only) | macOS privacy access for terminal-launched tools |
| `privacy` | security | Telemetry/anonymous usage data |
| `advanced` | advanced (desktop only) | Low-level compatibility/troubleshooting settings |
| `dev` | advanced (dev builds only) | Internal dev-only UI-state tools |
| `experimental` | experimental | In-progress features, gated toggles |
| `repo-{repoId}` | repositories | Per-repository overrides (one section per repo) |

## Key interactions
- Click a sidebar row / use Cmd+J → jump to a section (`scrollToSection` sets `activeSectionId`, scrolls the content column, and lazily mounts that section's Pane the first time).
- Type in the search box → filters both the sidebar (grouped, ranked matches) and which section is considered "matched" in the content column; clearing search restores the full grouped list. Search state resets automatically on unmount (`setSettingsRootNode`'s cleanup clears `settingsSearchQuery`).
- Deep link via `settingsNavigationTarget` (`openSettingsTarget()`) — used by other parts of the app to jump straight to a pane/subsection (e.g. a specific Appearance accordion) and optionally trigger an intent like `add-quick-command`.
- Shift-click the Experimental sidebar entry → unlocks a hidden experimental group for the rest of the session (`hiddenExperimentalUnlocked`, intentionally not persisted).
- Press Escape → closes Settings, except a first Escape while the Shortcuts pane is focused shows a "press ESC again" confirm toast (`SHORTCUTS_ESCAPE_CONFIRM_TOAST_ID`) to avoid accidentally discarding an in-progress shortcut capture; Escape inside any input/textarea/contenteditable is swallowed by the field itself.
- Close via back button or window close → `closeSettingsPageWithPromptGuard` prompts to discard unsaved Git AI Author (commit/branch prompt) changes before leaving; the same guard is wired into `registerWindowCloseGuard` so quitting the app or closing the window prompts too.
- Import terminal themes/fonts (Ghostty import via `useGhosttyImport`, Warp theme import via `useWarpThemeImport`) surfaced as header actions inside the Appearance/Terminal sections.

## Notable implementation details / known issues
- Section content is lazily mounted (not lazily code-split) per `settings-load-performance.ts`: `general` is the only section mounted eagerly on open; every other Pane mounts on first visit and then stays mounted for the rest of the Settings session (`mountedSectionIds` only grows). This trades a larger in-memory tree for instant revisits.
- `useSettingsNavigationMetadata()` is intentionally free of any Settings-pane UI imports — it's a pure metadata builder shared with the Cmd+J palette, guarded by a comment warning not to let the two surfaces' available sections drift apart (see `docs/reference/cmd-j-settings-actions-plan.md`).
- `useSettingsNavigationMetadata` explicitly depends on `i18n.language` (`activeLocale`) in its memo — without it, a language switch alone would leave Settings/Cmd+J showing stale-language section titles until Settings remounts.
- Escape-to-close conflicts with any open dialog/menu/listbox are avoided via a DOM query (`hasVisibleOverlay`) checking for visible `[role="dialog"]`/`[role="listbox"]`/`[role="menu"]` elements before treating Escape as "close Settings".
