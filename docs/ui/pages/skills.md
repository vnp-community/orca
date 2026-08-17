# Skills

**Route / trigger:** `activeView === 'skills'`. The store exposes `openSkillsPage()` (`frontend/src/renderer/src/store/slices/ui.ts`) and `App.tsx` renders `<SkillsPage />` when `activeView === 'skills'`, but no sidebar button, menu item, or command-palette entry currently calls `openSkillsPage()` anywhere in the frontend (confirmed by repo-wide search). `resolveZoomTarget` (`frontend/src/renderer/src/hooks/resolve-zoom-target.ts`) does recognize `'skills'` as a valid `activeView` for zoom-target routing, so the view is fully wired end-to-end — it simply has no discoverable entry point in the current build. Treat this as a page that exists but is not yet exposed in the UI (possibly awaiting a launcher or gated by an unreleased flag).
**Top-level component:** `SkillsPage` (`frontend/src/renderer/src/components/skills/SkillsPage.tsx`)

## Purpose
Catalog of AI agent "skills" (Claude/Codex/Agent-Skills format instruction packages) discovered on the local filesystem — home directory, per-repo, bundled with Orca, and plugin-provided — so a user can see what's installed/available and where each skill file lives.

## Layout
```
┌ header: Back  [Skills] [Beta]   "N skills from M sources" ─────────────────────┐
├ toolbar: [Search skills...]  [Provider ▾]  [Source ▾]  [Refresh]               │
│          chips: Home n · Repository n · Bundled n · Plugin n                    │
├──────────────────────────────────────────────────────────────────────────────┤
│ scrollable list (max-w-5xl) of SkillCard, or an EmptyState (loading/no results) │
└──────────────────────────────────────────────────────────────────────────────┘
```
- Header: Back button, `BookOpen` icon, "Skills"/"Beta" title, and a live count ("N skill(s) from M source(s)").
- Toolbar: a search `Input`, two `Select`s (provider: `codex|claude|agent-skills`; source kind: `home|repo|bundled|plugin`), and a Refresh button (spinning `RefreshCw` while loading). Below that, a row of small count chips per source kind.
- Body: `visibleSkills.map(skill => <SkillCard skill={skill} />)` — each card shows name, `Local`/`Available` badge (`skill.installed`), source-kind badge, description (or "No description found."), the resolved `skillFilePath` (monospace, truncated with tooltip), provider badges, source label, file count, and a relative "updated" timestamp. A "Reveal file" icon button opens the skill's file in the OS file manager (`window.api.shell.openInFileManager`). If nothing matches, `EmptyState` shows a scanning spinner, "No matches" (filters too narrow), or "No local skills found".

## Data shown
- **Discovery result**: `result: SkillDiscoveryResult | null`, loaded via `window.api.skills.discover()` (see `frontend/src/main/skills/discovery.ts` for the scan implementation). Shape (`frontend/src/shared/skills.ts`):
  - `skills: DiscoveredSkill[]` — `{ id, name, description, providers: SkillProvider[] ('codex'|'claude'|'agent-skills'), sourceKind: SkillSourceKind ('home'|'repo'|'bundled'|'plugin'), sourceLabel, rootPath, directoryPath, skillFilePath, installed, fileCount, updatedAt }`
  - `sources: SkillDiscoverySource[]` — `{ id, label, path, sourceKind, providers, exists, skippedReason? ('missing'|'remote-repo') }` — used to compute `activeSourceCount` (sources that actually exist on disk).
  - `scannedAt: number`
- **Filtering**: local `filters: SkillsFilterState` (`{ query, sourceKind: SkillSourceKind|'all', provider: SkillProvider|'all' }`) applied via `filterSkills()` (`frontend/src/renderer/src/components/skills/skills-filter.ts`), which matches query text against name/description/sourceLabel/directoryPath/providers (case-insensitive substring) and enforces a max query byte length (`isSkillsFilterQueryTooLarge`, 2KB) to avoid pathological clipboard pastes. `countSkillsBySource()` computes the per-source chip counts.
- Only one store binding is used: `closeSkillsPage` (`useAppStore`) — the rest of the page's state (`result`, `loading`, `filters`) is local `useState`, refetched on mount and on manual Refresh.

## Key interactions
- Type in the search box to filter by name/description/path/provider.
- Pick a provider or source-kind filter from the two selects.
- Click Refresh to re-run `window.api.skills.discover()` (shows a toast on failure: "Could not scan local skills").
- Click "Reveal file" on a card to open that skill's file in the system file manager; shows a toast if that fails.
- Escape (when no dialog/menu is open and focus isn't in an editable field) calls `closeSkillsPage()`.

## Notable implementation details / known issues
- **No current UI entry point.** `openSkillsPage` is defined and `App.tsx` wires the view, but nothing in the frontend calls it — this is worth flagging to whoever picks up Skills next, since testing it today requires dispatching `openSkillsPage()` manually (e.g. via devtools/store) rather than clicking through the app.
- Discovery is filesystem-scan based and runs fresh each time the page mounts or Refresh is pressed — there's no caching/store slice for skills (unlike most other pages, state lives entirely in the component).
- `installed` distinguishes skills actually present locally ("Local" badge) from ones merely discovered as "Available" (e.g. bundled-but-not-copied); the exact installed/available semantics live in `frontend/src/main/skills/discovery.ts`, not explored further here.
