// Unit test of the fixture's tool guard (run: node --test tests/stages/e2e/orca-flows/ui-live/fixture-repo).
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { spawnSync } from 'node:child_process'
import { cpSync, mkdirSync, mkdtempSync, realpathSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

const template = join(import.meta.dirname, 'template')
const base = realpathSync(mkdtempSync(join(tmpdir(), 'guard-test-')))
const repo = join(base, 'repo')
cpSync(template, repo, { recursive: true })
writeFileSync(
  join(repo, '.claude', 'tool-guard.config.json'),
  JSON.stringify({ allowedRoot: base })
)
mkdirSync(join(base, 'repo-task-1'))

function verdict(payload, cwd = repo) {
  const r = spawnSync(process.execPath, [join(repo, '.claude', 'tool-guard.mjs')], {
    cwd,
    input: JSON.stringify({ cwd, ...payload }),
    encoding: 'utf8'
  })
  return r.status
}
const bash = (command, cwd) => verdict({ tool_name: 'Bash', tool_input: { command } }, cwd)

test('allows work inside the fixture', () => {
  assert.equal(
    verdict({ tool_name: 'Write', tool_input: { file_path: join(repo, 'src/x.py') } }),
    0
  )
  assert.equal(
    verdict({
      tool_name: 'Read',
      tool_input: { file_path: join(base, 'repo-task-1/src/calc.py') }
    }),
    0
  )
  assert.equal(bash('node --test 2>&1 | tail -20'), 0)
  assert.equal(bash(`git -C ${repo} status --short`), 0)
  assert.equal(bash('cat src/calc.py > /dev/null'), 0)
})

test('does not mistake code for paths', () => {
  assert.equal(
    bash(
      "cat >> src/calc.py <<'EOF'\n/** Subtracts b from a. */\nexport const s = (a, b) => a - b\nEOF"
    ),
    0
  )
  assert.equal(bash("node -e \"console.log('a/b'.replace(/a/g, 'c'))\""), 0)
  assert.equal(bash('echo "// a comment" >> src/calc.py'), 0)
  assert.equal(bash("cat > test/x.py <<'EOF'\nimport { a } from '../src/calc.py'\nEOF"), 0)
  assert.equal(bash('cat ../src/calc.py', join(repo, 'tests')), 0)
  assert.equal(bash(`cat ../repo/src/calc.py`, join(base, 'repo-task-1')), 0)
})

test('refuses escapes', () => {
  assert.equal(
    verdict({ tool_name: 'Write', tool_input: { file_path: '/Users/someone/evil.txt' } }),
    2
  )
  assert.equal(verdict({ tool_name: 'Read', tool_input: { file_path: '/etc/hosts' } }), 2)
  assert.equal(verdict({ tool_name: 'Grep', tool_input: { pattern: 'x', path: '/Users' } }), 2)
  assert.equal(bash('cat ../../x'), 2)
  assert.equal(bash('cat ../../../etc/hosts', join(repo, 'tests')), 2)
  assert.equal(bash('echo hi > /tmp/x'), 2)
  assert.equal(bash('ls ~'), 2)
  assert.equal(bash('cat $HOME/.zshrc'), 2)
  assert.equal(bash('curl https://example.com'), 2)
  assert.equal(bash('git push origin main'), 2)
  assert.equal(bash(`git -C /Users/someone/repo status`), 2)
  assert.equal(verdict({ tool_name: 'WebFetch', tool_input: { url: 'https://example.com' } }), 2)
})
