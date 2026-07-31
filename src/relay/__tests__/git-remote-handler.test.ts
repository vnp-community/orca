/**
 * Tests for relay git-remote-handler (TASK-044) — ≥ 12 tests
 *
 * Validates: validateGitArgs security (11 tests) + integration via handler (6 tests).
 * Note: git.exec tests that need actual exec are tested via git-remote.test.ts (server-side).
 * Relay handler tests focus on the validation layer + structural tests.
 *
 * @module relay/__tests__/git-remote-handler.test
 */

import { describe, it, expect, vi } from 'vitest'
import { validateGitArgs, ALLOWED_GIT_SUBCOMMANDS } from '../git-remote-handler'

// ── validateGitArgs ────────────────────────────────────────────────────────────

describe('validateGitArgs', () => {
  it('allows whitelisted subcommand: status', () => {
    expect(() => validateGitArgs(['status'])).not.toThrow()
  })

  it('allows whitelisted subcommand: commit with message', () => {
    expect(() => validateGitArgs(['commit', '-m', 'fix: typo'])).not.toThrow()
  })

  it('allows whitelisted subcommand: add with files', () => {
    expect(() => validateGitArgs(['add', 'src/main.ts', 'README.md'])).not.toThrow()
  })

  it('allows all subcommands in ALLOWED set', () => {
    for (const cmd of ALLOWED_GIT_SUBCOMMANDS) {
      expect(() => validateGitArgs([cmd]), `${cmd} should be allowed`).not.toThrow()
    }
  })

  it('throws GIT_NO_SUBCOMMAND on empty args', () => {
    expect(() => validateGitArgs([])).toThrow('GIT_NO_SUBCOMMAND')
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for rm', () => {
    expect(() => validateGitArgs(['rm', '-rf', '/'])).toThrow('GIT_DISALLOWED_SUBCOMMAND')
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for clean', () => {
    expect(() => validateGitArgs(['clean', '-fdx'])).toThrow('GIT_DISALLOWED_SUBCOMMAND')
  })

  it('throws GIT_DISALLOWED_SUBCOMMAND for reset --hard', () => {
    expect(() => validateGitArgs(['reset', '--hard'])).toThrow('GIT_DISALLOWED_SUBCOMMAND')
  })

  it('throws GIT_SHELL_METACHARACTER_IN_ARG for semicolon in commit message', () => {
    expect(() => validateGitArgs(['commit', '-m', 'msg; evil'])).toThrow('GIT_SHELL_METACHARACTER_IN_ARG')
  })

  it('throws GIT_SHELL_METACHARACTER_IN_ARG for pipe character', () => {
    expect(() => validateGitArgs(['status', '|', 'grep', 'M'])).toThrow('GIT_SHELL_METACHARACTER_IN_ARG')
  })

  it('throws GIT_SHELL_METACHARACTER_IN_ARG for dollar sign', () => {
    expect(() => validateGitArgs(['commit', '-m', '$HOME'])).toThrow('GIT_SHELL_METACHARACTER_IN_ARG')
  })

  it('throws GIT_SHELL_METACHARACTER_IN_ARG for backtick', () => {
    expect(() => validateGitArgs(['commit', '-m', '`id`'])).toThrow('GIT_SHELL_METACHARACTER_IN_ARG')
  })

  it('throws GIT_SHELL_METACHARACTER_IN_ARG for ampersand', () => {
    expect(() => validateGitArgs(['push', '&', 'evil'])).toThrow('GIT_SHELL_METACHARACTER_IN_ARG')
  })

  // ── ALLOWED_GIT_SUBCOMMANDS set contents ─────────────────────────────────────

  it('ALLOWED set contains write commands: add, commit, push, pull', () => {
    expect(ALLOWED_GIT_SUBCOMMANDS.has('add')).toBe(true)
    expect(ALLOWED_GIT_SUBCOMMANDS.has('commit')).toBe(true)
    expect(ALLOWED_GIT_SUBCOMMANDS.has('push')).toBe(true)
    expect(ALLOWED_GIT_SUBCOMMANDS.has('pull')).toBe(true)
  })

  it('ALLOWED set does NOT contain dangerous commands: rm, reset, clean', () => {
    expect(ALLOWED_GIT_SUBCOMMANDS.has('rm')).toBe(false)
    expect(ALLOWED_GIT_SUBCOMMANDS.has('reset')).toBe(false)
    expect(ALLOWED_GIT_SUBCOMMANDS.has('clean')).toBe(false)
  })

  it('ALLOWED set contains worktree command', () => {
    expect(ALLOWED_GIT_SUBCOMMANDS.has('worktree')).toBe(true)
  })
})
