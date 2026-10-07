import { EventEmitter } from 'node:events'

export interface RecordedSpawnCall {
  method: 'spawn' | 'execFile' | 'execFileSync' | 'exec' | 'spawnSync'
  file: string
  argv?: string[]
  options?: Record<string, any>
}

export interface SpawnRecorderOptions {
  stdout?: string | Buffer
  stderr?: string | Buffer
  exitCode?: number
}

export interface SpawnRecorder {
  calls: RecordedSpawnCall[]
  reset: () => void
  createFakeChild: () => any
}

export function createFakeChild(opts: SpawnRecorderOptions = {}): any {
  const child = Object.assign(new EventEmitter(), {
    stdout: new EventEmitter(),
    stderr: new EventEmitter(),
    stdin: { write: () => true, end: () => {} },
    pid: 99999,
    kill: () => true
  })

  setTimeout(() => {
    if (opts.stdout) {
      child.stdout.emit('data', Buffer.from(opts.stdout))
    }
    if (opts.stderr) {
      child.stderr.emit('data', Buffer.from(opts.stderr))
    }
    child.emit('close', opts.exitCode ?? 0)
    child.emit('exit', opts.exitCode ?? 0)
  }, 2)

  return child
}

export function installSpawnRecorder(childProcessModule: any, opts: SpawnRecorderOptions = {}): SpawnRecorder {
  const calls: RecordedSpawnCall[] = []

  const record = (method: RecordedSpawnCall['method'], file: string, arg2?: any, arg3?: any) => {
    let argv: string[] | undefined
    let options: Record<string, any> | undefined

    if (Array.isArray(arg2)) {
      argv = arg2
      options = typeof arg3 === 'object' && arg3 !== null ? arg3 : undefined
    } else if (typeof arg2 === 'object' && arg2 !== null) {
      options = arg2
    }

    calls.push({
      method,
      file,
      argv,
      options
    })
  }

  if (childProcessModule) {
    if (typeof childProcessModule.spawn === 'function') {
      childProcessModule.spawn = (file: string, arg2?: any, arg3?: any) => {
        record('spawn', file, arg2, arg3)
        return createFakeChild(opts)
      }
    }
    if (typeof childProcessModule.execFile === 'function') {
      childProcessModule.execFile = (file: string, arg2?: any, arg3?: any, cb?: any) => {
        record('execFile', file, arg2, arg3)
        const callback = typeof arg2 === 'function' ? arg2 : typeof arg3 === 'function' ? arg3 : cb
        if (typeof callback === 'function') {
          setTimeout(() => callback(null, opts.stdout ?? '', opts.stderr ?? ''), 2)
        }
        return createFakeChild(opts)
      }
    }
    if (typeof childProcessModule.execFileSync === 'function') {
      childProcessModule.execFileSync = (file: string, arg2?: any, arg3?: any) => {
        record('execFileSync', file, arg2, arg3)
        return Buffer.from(opts.stdout ?? '')
      }
    }
    if (typeof childProcessModule.spawnSync === 'function') {
      childProcessModule.spawnSync = (file: string, arg2?: any, arg3?: any) => {
        record('spawnSync', file, arg2, arg3)
        return {
          status: opts.exitCode ?? 0,
          stdout: Buffer.from(opts.stdout ?? ''),
          stderr: Buffer.from(opts.stderr ?? ''),
          pid: 99999
        }
      }
    }
    if (typeof childProcessModule.exec === 'function') {
      childProcessModule.exec = (file: string, arg2?: any, arg3?: any) => {
        record('exec', file, arg2, arg3)
        return createFakeChild(opts)
      }
    }
  }

  return {
    calls,
    reset: () => {
      calls.length = 0
    },
    createFakeChild: () => createFakeChild(opts)
  }
}
