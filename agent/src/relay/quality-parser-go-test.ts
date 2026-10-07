import { QualityParserInput, QualityParserOutput, RawQualityFinding } from './quality-parser-types'
import fs from 'fs'
import readline from 'readline'
import { toRepoRelative } from './quality-repo-path-mapping'

type GoTestOutput = {
  Time?: string
  Action: string
  Package?: string
  Test?: string
  Output?: string
}

export const goTestParser = {
  key: 'go-test',
  parse: async (input: QualityParserInput): Promise<QualityParserOutput> => {
    return new Promise((resolve) => {
      const findings: RawQualityFinding[] = []
      
      const fileStream = fs.createReadStream(input.stdoutPath, { encoding: 'utf8' })
      fileStream.on('error', (e: any) => {
        resolve({
          findings: [],
          failure: { kind: 'env', envReason: 'STDOUT_UNREADABLE', detail: e.message },
          stats: { scannedFiles: 0 }
        })
      })

      const rl = readline.createInterface({
        input: fileStream,
        crlfDelay: Infinity
      })

      // We maintain output text for tests currently running.
      // testKey = `${Package}/${Test}`
      const testOutputs: Map<string, string[]> = new Map()

      let buildFailed = false
      let panicOccurred = false

      rl.on('line', (line) => {
        if (!line.trim()) return

        let event: GoTestOutput
        try {
          event = JSON.parse(line)
        } catch (e) {
          return // ignore non-json
        }

        const pkg = event.Package || ''
        const test = event.Test || ''
        
        if (event.Action === 'output' && event.Output) {
          if (test) {
            const key = `${pkg}/${test}`
            let outputs = testOutputs.get(key)
            if (!outputs) {
              outputs = []
              testOutputs.set(key, outputs)
            }
            outputs.push(event.Output)
            if (event.Output.includes('panic:')) {
              panicOccurred = true
            }
          } else {
            // Package level output - maybe a build error
            // file.go:12:3: error
            const match = event.Output.match(/^([^:]+\.go):(\d+):(?:(\d+):)?\s+(.*)/)
            if (match) {
              buildFailed = true
              const [_, pathStr, lineStr, colStr, msg] = match
              const { file, outside } = toRepoRelative(pathStr, input.cwd, input.repoRoot, input.platform)
              if (!outside) {
                findings.push({
                  ruleId: 'go-test/build-failed',
                  message: msg.trim(),
                  file,
                  line: parseInt(lineStr, 10) || 1,
                  column: colStr ? parseInt(colStr, 10) : 1,
                  severity: 'error'
                })
              }
            }
          }
        }

        if (event.Action === 'fail') {
          if (test) {
            const key = `${pkg}/${test}`
            const outputs = testOutputs.get(key) || []
            // Extract the first file_test.go:line: reference for position
            let file = ''
            let lineNum = 1
            let colNum = 1
            let message = outputs.join('').trim()
            
            for (const outLine of outputs) {
              const match = outLine.match(/([^:]+_test\.go):(\d+):(.*)/)
              if (match) {
                const mapped = toRepoRelative(match[1], input.cwd, input.repoRoot, input.platform)
                if (!mapped.outside) {
                  file = mapped.file
                  lineNum = parseInt(match[2], 10) || 1
                  break
                }
              }
            }

            // Fallback for file
            if (!file) {
              file = pkg // fallback to package name or something
            }

            findings.push({
              ruleId: panicOccurred ? 'go-test/panic' : 'go-test/test-failed',
              message: message,
              file,
              line: lineNum,
              column: colNum,
              severity: 'error'
            })
            
            // clear output
            testOutputs.delete(key)
          } else {
            // Package failed
            if (event.Output && event.Output.includes('build failed')) {
              buildFailed = true
            }
          }
        }

        if (event.Action === 'pass' || event.Action === 'skip') {
          if (test) {
            const key = `${pkg}/${test}`
            testOutputs.delete(key) // clear output to save memory
          }
        }
      })

      rl.on('close', () => {
        resolve({
          findings,
          failure: null,
          stats: { scannedFiles: 0 } // Not collecting files stats for gotest here unless we count packages
        })
      })
    })
  }
}
