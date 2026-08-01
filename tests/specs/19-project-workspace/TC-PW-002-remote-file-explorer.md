# TC-PW-002 — Remote File Explorer

**BL Reference:** BL-PW-02  
**Flow Reference:** docs/flows/logic/project-workspace.md  
**Priority:** P0  
**Type:** Integration  
**Actor:** Developer, Lead (Carlos especially)

---

## TC-PW-002-01: Lazy-load directory — Initial depth 2

**Priority:** P0

### Steps
1. Open project workspace (BL-PW-01)
2. File tree initialized

### Expected Results
- `relay.call('fs.readDir', { path: project.repoPath, depth: 2 })` called during init
- Top 2 levels pre-loaded (not full recursive)
- Third level: not loaded yet (lazy)

### Assertions
```
await rpc.call('workspace.switch', { projectId })
readDirCall = capturedRelayCall('fs.readDir')
assert readDirCall.args.depth === 2
```

---

## TC-PW-002-02: Expand subdirectory — Lazy load depth 1

**Priority:** P0

### Steps
1. User clicks `src/` directory to expand (level 3+)

### Expected Results
- `relay.call('fs.readDir', { path: '/repo/src', depth: 1 })` called
- src/ children loaded
- Each child: `{ name, type: 'file'|'dir', size, modifiedAt }`

---

## TC-PW-002-03: Git status decorations

**Priority:** P0

### Steps
1. `auth.ts` modified (M), `newfile.ts` untracked (?)

### Expected Results
- `auth.ts` shows modified badge
- `newfile.ts` shows untracked badge
- Clean files: no badge

---

## TC-PW-002-04: File viewer — read-only, syntax highlight, max 5MB

**Priority:** P0

### Steps (a): Small TypeScript file
1. Click `auth.ts` (20KB)

### Expected Results
- `relay.call('fs.readFile', { path: '.../auth.ts' })`
- TypeScript syntax highlight, read-only

### Steps (b): Large file > 5MB
1. Click `large.bin` (10MB)

### Expected Results
- Error: "File too large to preview (max 5MB)"
- `fs.readFile` NOT called (size pre-checked)

### Assertions
```
// Large file blocked
mockFileStat('large.bin', { size: 10 * 1024 * 1024 })
result = await rpc.call('fs.openFile', { projectId, path: 'large.bin' }).catch(e => e)
assert result.code === 'FILE_TOO_LARGE'
assert capturedRelayCall('fs.readFile') === undefined
```

---

## TC-PW-002-05: File search — ripgrep content search

**Priority:** P0

### Steps
1. Ctrl+Shift+F in Explorer
2. Search: `"useState"` with glob `*.tsx`

### Expected Results
- `relay.call('fs.search', { pattern: 'useState', path: repoPath, glob: '*.tsx' })`
- Dev Server executes: `rg --json useState --glob '*.tsx'`
- Results: `[{ file, line, context }]` streamed back

### Assertions
```
results = []
subscribeResults(r => results.push(r))
await rpc.call('fs.search', { projectId, pattern: 'useState', glob: '*.tsx' })

searchCall = capturedRelayCall('fs.search')
assert searchCall.args.pattern === 'useState'
assert searchCall.args.glob === '*.tsx'

assert results.length > 0
assert results[0].file !== undefined
assert typeof results[0].line === 'number'
assert results[0].context.includes('useState')
```

---

## TC-PW-002-06: File search — Performance < 2s

**Priority:** P1

### Steps
1. Large repo (1000+ TS files)
2. `fs.search { pattern: 'useEffect', glob: '*.ts' }`

### Expected Results
- First result streams within 500ms
- Total search completes within 2s

### Assertions
```
const start = Date.now()
let firstResultAt = null
subscribeResults(r => { if (!firstResultAt) firstResultAt = Date.now() })
await rpc.call('fs.search', { projectId, pattern: 'useEffect', glob: '*.ts' })
assert firstResultAt - start < 500
assert Date.now() - start < 2000
```

---

## TC-PW-002-07: File search — No results

**Priority:** P1

### Steps
1. `fs.search { pattern: 'xyzNotFoundAnywhere12345' }`

### Expected Results
- Empty results `[]`
- No error, "No results found" message

---

## TC-PW-002-08: Context menu — Copy path, Open in Terminal

**Priority:** P1

### Steps
1. Right-click `src/auth.ts`
2. "Copy path" → clipboard
3. "Open in Terminal" → terminal cd to dir

### Expected Results
- Clipboard: `/srv/projects/vnp-blc/src/auth.ts`
- Terminal: new session with `cd /srv/projects/vnp-blc/src`

---

## TC-PW-002-09: Toggle hidden files (dotfiles)

**Priority:** P1

### Steps
1. Toggle OFF: `.env`, `.gitignore` NOT visible
2. Toggle ON: `.env`, `.gitignore` visible

---

*TC-PW-002 — Orca v5.0 — Updated 2026-08-01*
