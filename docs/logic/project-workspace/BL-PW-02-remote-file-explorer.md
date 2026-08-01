# BL-PW-02 — Remote File Explorer

| Trường | Giá trị |
|--------|---------|
| **Mã** | BL-PW-02 |
| **Tên** | Remote File Explorer |
| **Domain** | Project Workspace |
| **Actor** | Developer, Lead |
| **Priority** | P0 |

---

## Mô tả

File Explorer duyệt cây thư mục và files trên dev server thông qua relay. Lazy-load directories khi mở rộng. Hiển thị git status decorations (Modified, Added, Deleted, Untracked) inline.

---

## File Tree Loading

```typescript
// Lazy-load: chỉ load depth=1 khi expand, không load toàn bộ cây ngay
async function expandDirectory(path: string): Promise<FileTreeNode[]> {
  const entries = await relay.call('fs.readDir', {
    path,
    depth: 1,
    includeDotFiles: false,  // ẩn .git, .env... (toggle option)
    sortBy: 'name',
    foldersFirst: true
  })
  return entries.map(e => ({
    name: e.name,
    path: e.path,
    isDir: e.isDirectory,
    size: e.sizeBytes,
    gitStatus: gitStatusMap.get(e.path)  // 'M' | 'A' | 'D' | '?' | undefined
  }))
}
```

## Git Status Decorations

```
File tree với git status overlays:
  📁 src/
  ├── 📁 auth/
  │   ├── 📄 auth-manager.ts   [M]  ← modified (yellow dot)
  │   ├── 📄 auth.test.ts      [M]
  │   └── 📄 bcrypt-utils.ts   [A]  ← added (green dot)
  ├── 📄 index.ts
  └── ...

Status colors:
  [M] = modified   → yellow
  [A] = added      → green
  [D] = deleted    → red strikethrough
  [?] = untracked  → grey
  [C] = conflict   → orange
```

---

## File Viewer (read-only)

```typescript
async function openFile(filePath: string): Promise<void> {
  // Size check trước khi load
  const stat = await relay.call('fs.stat', { path: filePath })
  if (stat.sizeBytes > MAX_PREVIEW_SIZE) {  // 5MB
    showWarning("File too large to preview (>5MB). Open in terminal instead.")
    return
  }

  const content = await relay.call('fs.readFile', {
    path: filePath,
    encoding: 'utf-8'
  })

  // Detect language for syntax highlighting
  const lang = detectLanguage(filePath)  // from extension
  openFileTab({ path: filePath, content, lang, readOnly: true })
}
```

## File Search

```typescript
async function searchFiles(query: string): Promise<SearchResult[]> {
  // Filename search
  const byName = await relay.call('fs.glob', {
    pattern: `**/*${query}*`,
    cwd: project.repoPath,
    ignore: ['node_modules/', '.git/', 'dist/'],
    limit: 50
  })

  // Content search (grep)
  const byContent = await relay.call('fs.grep', {
    pattern: query,
    cwd: project.repoPath,
    recursive: true,
    ignore: ['node_modules/', '.git/'],
    limit: 30
  })

  return mergeAndRankResults(byName, byContent)
}
```

## Context Menu Actions

```
Right-click file/folder:
  - 📋 Copy path (relative)
  - 📋 Copy absolute path
  - 💻 Open in Terminal (cd to directory)
  - 🔍 Search in this folder
  - 📂 Reveal in Explorer (focus + expand parents)
  - — (separator) —
  - 🔀 Git: View diff (if modified)
  - 🔀 Git: Stage file
  - 🔀 Git: Discard changes (with confirm)
```

---

## Tiêu chí chấp nhận

- [ ] Lazy-load directories on expand
- [ ] Git status decorations inline (color + badge)
- [ ] File viewer: read content, syntax highlight, max 5MB
- [ ] File search: by name (glob) + by content (grep) via relay
- [ ] Context menu: copy path, open in terminal, git actions
- [ ] Toggle hidden files (.gitignore, .env...)
- [ ] Refresh button + auto-refresh sau agent complete
- [ ] Folder collapse/expand state persistent during workspace session
