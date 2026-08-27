import { describe, expect, it } from 'vitest'
import { createOsc133CommandFinishedScanner } from '../shared/terminal-osc133-command-finished'

describe('PowerShell OSC 133 bootstrap output vs the shared scanner', () => {
  it("the scanner recognizes the bootstrap prompt function's emitted C/D sequences", () => {
    const started: number[] = []
    const finished: (number | null)[] = []
    const scanner = createOsc133CommandFinishedScanner(
      (exitCode) => finished.push(exitCode),
      () => started.push(started.length)
    )

    // Simulates one PowerShell prompt cycle's emitted bytes: prior command's
    // D (exit code 0) is skipped on the very first prompt (HasSeenPrompt is
    // false), so start the trace mid-session — this is what the bootstrap's
    // `Global:prompt` function and PSConsoleHostReadLine wrapper actually
    // write to the console, per pty-osc133-bootstrap.ts.
    const A = '\x1b]133;A\x07'
    const C = '\x1b]133;C\x07'
    const D = (code: number) => `\x1b]133;D;${code}\x07`

    scanner.scan(A) // prompt starts
    scanner.scan(C) // PSConsoleHostReadLine wrapper: command exec'd
    expect(started).toHaveLength(1)

    scanner.scan(D(0)) // next prompt's Global:prompt emits D for the finished command
    expect(finished).toEqual([0])
  })

  it('handles a non-zero exit code split across two chunks', () => {
    const finished: (number | null)[] = []
    const scanner = createOsc133CommandFinishedScanner((exitCode) => finished.push(exitCode))

    scanner.scan('\x1b]133;D;1')
    scanner.scan('27\x07') // BEL terminator arrives in a later chunk

    expect(finished).toEqual([127])
  })
})
