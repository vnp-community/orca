# TDD-AG-06: Tool Handlers

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/agent.js`

---

## 1. Tool Handler Contract

```javascript
async function toolHandler(input, { onChunk }) {
  // input: validated against inputSchema
  // onChunk: callback for streaming output { text: string }

  // Return:
  return {
    content: [{ type: 'text', text: 'result' }]
    // OR: isError: true, content: [{ type: 'text', text: 'error' }]
  }
}
```

---

## 2. shell Handler

```javascript
async function shellHandler(input, { onChunk }) {
  const { command, args = [], cwd = WORK_DIR, timeout = 300_000 } = input

  // SECURITY: shell: false — no shell injection
  const child = spawn(command, args, {
    cwd,
    shell: false,   // CRITICAL
    env:   process.env
  })

  // Stream stdout/stderr
  let output = ''
  child.stdout.on('data', (chunk) => {
    const text = chunk.toString()
    output += text
    onChunk({ text })
  })
  child.stderr.on('data', (chunk) => {
    const text = chunk.toString()
    output += text
    onChunk({ text })
  })

  await new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      child.kill()
      reject(new Error(`Command timed out after ${timeout}ms`))
    }, timeout)

    child.on('close', (code) => {
      clearTimeout(timer)
      if (code === 0) resolve()
      else reject(new Error(`Exit code: ${code}`))
    })
  })

  return { content: [{ type: 'text', text: output }] }
}
```

---

## 3. read_file Handler

```javascript
async function readFileHandler(input) {
  const { path: filePath, encoding = 'utf8', maxBytes = 1_048_576 } = input

  // Resolve relative to WORK_DIR
  const resolved = resolve(WORK_DIR, filePath)

  // Security: ensure path doesn't escape WORK_DIR
  if (!resolved.startsWith(WORK_DIR)) {
    return { isError: true, content: [{ type: 'text', text: 'Access denied: path outside work dir' }] }
  }

  const stat = await fs.stat(resolved)
  if (stat.size > maxBytes) {
    return { isError: true, content: [{ type: 'text', text: `File too large: ${stat.size} bytes` }] }
  }

  const content = await fs.readFile(resolved, encoding)
  return { content: [{ type: 'text', text: content }] }
}
```

---

## 4. list_dir Handler

```javascript
async function listDirHandler(input) {
  const { path: dirPath = '.' } = input
  const resolved = resolve(WORK_DIR, dirPath)

  const entries = await fs.readdir(resolved, { withFileTypes: true })
  const result = entries.map(e => ({
    name:  e.name,
    type:  e.isDirectory() ? 'directory' : 'file',
    size:  e.isFile() ? statSync(resolve(resolved, e.name)).size : null
  }))

  return { content: [{ type: 'text', text: JSON.stringify(result, null, 2) }] }
}
```

---

## 5. claude_code Handler

```javascript
async function claudeCodeHandler(input, { onChunk }) {
  const { prompt, cwd = WORK_DIR, model, maxTokens } = input

  const args = ['--print']  // non-interactive mode
  if (model)     args.push('--model', model)
  if (maxTokens) args.push('--max-tokens', String(maxTokens))

  const child = spawn('claude', args, {
    cwd,
    shell: false,
    env:   process.env,
    stdin: 'pipe'
  })

  child.stdin.write(prompt)
  child.stdin.end()

  let output = ''
  child.stdout.on('data', (chunk) => {
    const text = chunk.toString()
    output += text
    onChunk({ text })
  })

  await waitForClose(child)
  return { content: [{ type: 'text', text: output }] }
}
```

---

## 6. git Handler

```javascript
// git commands are WHITELISTED — no arbitrary git command
const ALLOWED_GIT_COMMANDS = [
  'status', 'log', 'diff', 'branch', 'add', 'commit',
  'push', 'pull', 'fetch', 'checkout', 'stash', 'show'
]

async function gitHandler(input, { onChunk }) {
  const { subcommand, args = [], cwd = WORK_DIR } = input

  if (!ALLOWED_GIT_COMMANDS.includes(subcommand)) {
    return { isError: true, content: [{ type: 'text', text: `git ${subcommand} not allowed` }] }
  }

  return shellHandler({ command: 'git', args: [subcommand, ...args], cwd }, { onChunk })
}
```

---

## 7. gh Handler (GitHub CLI)

```javascript
async function ghHandler(input, { onChunk }) {
  const { args = [], cwd = WORK_DIR } = input
  // gh CLI: auth token from GH_TOKEN env var (injected by SessionManager)
  return shellHandler({ command: 'gh', args, cwd }, { onChunk })
}
```

---

## 8. docker Handler

```javascript
const ALLOWED_DOCKER_COMMANDS = ['ps', 'images', 'logs', 'stats', 'inspect', 'exec']

async function dockerHandler(input, { onChunk }) {
  const { subcommand, args = [] } = input

  if (!ALLOWED_DOCKER_COMMANDS.includes(subcommand)) {
    return { isError: true, content: [{ type: 'text', text: `docker ${subcommand} not allowed` }] }
  }

  return shellHandler({ command: 'docker', args: [subcommand, ...args] }, { onChunk })
}
```

---

## 9. gitnexus & codegraph Handlers

```javascript
// gitnexus và codegraph: pass-through wrappers
async function gitnexusHandler(input, { onChunk }) {
  const { args = [] } = input
  return shellHandler({ command: 'gitnexus', args }, { onChunk })
}

async function codegraphHandler(input, { onChunk }) {
  const { query, args = [] } = input
  return shellHandler({ command: 'codegraph', args: ['explore', query, ...args] }, { onChunk })
}
```

---

## 10. Tool Timeout

| Tool | Default Timeout |
|------|----------------|
| shell | 5 minutes |
| read_file | 30 seconds |
| list_dir | 30 seconds |
| claude_code | 10 minutes |
| git | 5 minutes |
| gh | 5 minutes |
| docker | 2 minutes |
