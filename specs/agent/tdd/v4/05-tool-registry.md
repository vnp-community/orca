# TDD-AG-05: Tool Registry

**Version:** 4.0  
**Date:** 2026-07-28  
**Source:** `deploy/dev/agent/agent.js`

---

## 1. Tool Registry Structure

```javascript
const toolRegistry = new Map()   // Map<toolName, ToolDefinition>

const ToolDefinition = {
  name:        string,    // MCP tool name (e.g., 'claude_code')
  description: string,
  inputSchema: JSONSchema,
  handler:     async (input) => ToolResult
}
```

---

## 2. Tool Discovery

```javascript
async function discoverTools() {
  const tools = []

  // Check each potential tool by running detection command
  const candidates = [
    { name: 'claude_code', detect: 'claude --version' },
    { name: 'gh',          detect: 'gh --version' },
    { name: 'git',         detect: 'git --version' },
    { name: 'gitnexus',    detect: 'gitnexus --version' },
    { name: 'codegraph',   detect: 'codegraph --version' },
    { name: 'docker',      detect: 'docker --version' },
    // shell, read_file, list_dir always available
  ]

  for (const { name, detect } of candidates) {
    try {
      await exec(detect)   // throws if not found
      tools.push(name)
    } catch {
      console.log(`[Agent] Tool not available: ${name}`)
    }
  }

  // Always-available tools (built-in):
  tools.push('shell', 'read_file', 'list_dir')

  return tools
}
```

---

## 3. Built-in Tools (Always Available)

| Tool | Description | Shell injection safe |
|------|-------------|---------------------|
| `shell` | Execute arbitrary shell command | ✅ spawn(shell: false) |
| `read_file` | Read file contents | ✅ fs.readFile() |
| `list_dir` | List directory contents | ✅ fs.readdir() |

---

## 4. Optional Tools (Require Installation)

| Tool | Command | Description |
|------|---------|-------------|
| `claude_code` | `claude` | Anthropic Claude CLI for code tasks |
| `gh` | `gh` | GitHub CLI |
| `git` | `git` | Git version control |
| `gitnexus` | `gitnexus` | GitNexus code analysis |
| `codegraph` | `codegraph` | Code relationship graph |
| `docker` | `docker` | Docker container management |

---

## 5. Tool Registration

```javascript
function registerTool(registry, toolDef) {
  if (registry.has(toolDef.name)) {
    console.warn(`[Agent] Duplicate tool: ${toolDef.name}`)
    return
  }
  registry.set(toolDef.name, toolDef)
  console.log(`[Agent] Registered tool: ${toolDef.name}`)
}
```

---

## 6. MCP Tool List Response

```javascript
// Response to 'tools/list' request:
{
  tools: Array.from(registry.values()).map(t => ({
    name:        t.name,
    description: t.description,
    inputSchema: t.inputSchema
  }))
}
```

---

## 7. Input Schema Examples

### shell tool

```json
{
  "type": "object",
  "properties": {
    "command": { "type": "string", "description": "Command to execute" },
    "args":    { "type": "array", "items": { "type": "string" } },
    "cwd":     { "type": "string", "description": "Working directory" },
    "timeout": { "type": "number", "description": "Timeout in ms (default: 300000)" }
  },
  "required": ["command"]
}
```

### read_file tool

```json
{
  "type": "object",
  "properties": {
    "path":     { "type": "string" },
    "encoding": { "type": "string", "enum": ["utf8", "base64", "hex"], "default": "utf8" },
    "maxBytes": { "type": "number", "default": 1048576 }
  },
  "required": ["path"]
}
```

### claude_code tool

```json
{
  "type": "object",
  "properties": {
    "prompt":    { "type": "string" },
    "cwd":       { "type": "string" },
    "model":     { "type": "string" },
    "maxTokens": { "type": "number" }
  },
  "required": ["prompt"]
}
```
