# TASK-FE-018: Verify FileViewer Monaco Integration

**Task ID:** TASK-FE-018
**Status:** ✅ COMPLETED — 2026-07-30
**Phase:** 1 — Core Fixes
**Priority:** P1
**Solution Ref:** SOL-FE-V6-007 (Section 2)
**Estimated effort:** 35 minutes
**Dependencies:** TASK-FE-001 (@monaco-editor/react must be installed)

---

## Objective

Read `FileViewer.tsx` (3169 bytes) and verify it uses Monaco Editor (`@monaco-editor/react`) for syntax-highlighted read-only file viewing. If it uses a plain `<pre>` tag instead, replace with Monaco.

---

## Step-by-Step Instructions

### Step 1: Read FileViewer.tsx in full

```
Read file: src/renderer/src/components/workspace/FileViewer.tsx
```

### Step 2: Determine current renderer

Is the file content displayed using:
- `<pre>` or `<code>` tag → needs Monaco replacement
- `Monaco Editor` from `@monaco-editor/react` → verify options and language detection
- Some other editor → document and evaluate

### Step 3: If using Monaco already — verify options

The Monaco Editor must have:
```typescript
options={{
  readOnly: true,           // MUST be true
  minimap: { enabled: false },
  fontSize: 12,
  scrollBeyondLastLine: false,
  wordWrap: 'on',
  lineNumbers: 'on',
}}
theme="vs-dark"
```

If any option is wrong, fix it.

### Step 4: If NOT using Monaco — replace

Replace the content renderer section:

```typescript
// OLD (example):
<pre className="text-xs font-mono p-3 overflow-auto max-h-80">{content}</pre>

// NEW: Add import at top of file
import Editor from '@monaco-editor/react'
// OR (if Editor is not default export):
import { Editor } from '@monaco-editor/react'

// Language detection helper (add near top of file):
const LANGUAGE_MAP: Record<string, string> = {
  ts: 'typescript', tsx: 'typescript',
  js: 'javascript', jsx: 'javascript',
  py: 'python', go: 'go', rs: 'rust',
  css: 'css', scss: 'scss',
  json: 'json', yaml: 'yaml', yml: 'yaml',
  md: 'markdown', html: 'html',
  sh: 'shell', bash: 'shell',
  sql: 'sql',
}
function detectLanguage(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() ?? ''
  return LANGUAGE_MAP[ext] ?? 'plaintext'
}

// In the JSX content section:
const language = detectLanguage(filePath)

{isLoading ? (
  <Skeleton className="h-48 w-full" />
) : (
  <Editor
    value={content}
    language={language}
    options={{
      readOnly: true,
      minimap: { enabled: false },
      fontSize: 12,
      scrollBeyondLastLine: false,
      wordWrap: 'on',
      lineNumbers: 'on',
    }}
    theme="vs-dark"
    height={300}
  />
)}
```

### Step 5: Verify FILE_TOO_LARGE error handling

TDD-FE-17 requires: if `fs.readFile` returns `FILE_TOO_LARGE` error, show a message:

```typescript
}).catch(err => {
  if (err.code === 'FILE_TOO_LARGE') {
    setContent('[File too large to display — max 5MB]')
  } else {
    setContent('[Error loading file]')
  }
}).finally(() => setIsLoading(false))
```

### Step 6: Verify binary file handling

If a binary file is requested, Monaco should show a message instead of garbled content:

```typescript
// After content is loaded, check for binary indicators:
const isBinary = content.includes('\u0000')  // null bytes = binary
if (isBinary) {
  setContent('[Binary file — cannot display]')
}
```

### Step 7: TypeScript check

```bash
cd /Users/binhnt/Work/blockchain/vnp-blc/orca
npx tsc --noEmit 2>&1 | grep -E "FileViewer|Editor" | head -10
```

---

## Acceptance Criteria

- [x] `FileViewer.tsx` uses `Editor` from `@monaco-editor/react`
- [x] `readOnly: true` in editor options
- [x] `detectLanguage()` function maps `.ts/.tsx` → `typescript`
- [x] Loading state shows `Skeleton`
- [x] Error state (FILE_TOO_LARGE, etc.) shows a message
- [x] Monaco `height={300}` or reasonable value set
- [x] `theme="vs-dark"` applied
- [x] No TypeScript errors

---

## Output

Report:
```
FileViewer content renderer: REPLACED
Monaco options readOnly: CORRECT
detectLanguage function: ADDED
FILE_TOO_LARGE handling: ADDED
TypeScript errors: 0
```
