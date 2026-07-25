// ─── repo-remote.test.ts ────────────────────────────────────────────────────
// Unit tests for remote repository IPC handlers:
// repo.listRemoteDirectory, repo.addRemote, repo.cloneRemote, repo.scanRemote
// (TASK-027)

import { describe, it, expect, vi, beforeEach } from 'vitest'

// ── Hoisted mocks ─────────────────────────────────────────────────────────────

const { ipcHandleMock, ipcRemoveHandlerMock } = vi.hoisted(() => ({
  ipcHandleMock: vi.fn(),
  ipcRemoveHandlerMock: vi.fn()
}))

vi.mock('electron', () => ({
  ipcMain: {
    handle: ipcHandleMock,
    removeHandler: ipcRemoveHandlerMock
  }
}))

const mockCall = vi.fn()
const mockRelay = { call: mockCall }

const mockManagerGet = vi.fn()
const mockManagerGetRelay = vi.fn()

vi.mock('../../../main/dev-server/dev-server-manager', () => ({
  DevServerManager: vi.fn()
}))

const mockAddRepo = vi.fn()
const mockStore = { addRepo: mockAddRepo }

// ── Handler extractors ─────────────────────────────────────────────────────────

type Handler = (event: null, params: unknown) => Promise<unknown>

let listRemoteDir: Handler
let addRemote: Handler
let cloneRemote: Handler
let scanRemote: Handler

beforeEach(async () => {
  vi.resetModules()
  ipcHandleMock.mockReset()
  ipcRemoveHandlerMock.mockReset()
  mockCall.mockReset()
  mockManagerGet.mockReset()
  mockManagerGetRelay.mockReset()
  mockAddRepo.mockReset()

  const { registerRepoRemoteIpcHandlers } = await import('../../../main/ipc/repo-remote-ipc')

  const fakeManager = {
    get: mockManagerGet,
    getRelay: mockManagerGetRelay
  }

  registerRepoRemoteIpcHandlers(fakeManager as never, mockStore as never)

  const handlers = new Map<string, Handler>()
  for (const call of ipcHandleMock.mock.calls) {
    handlers.set(call[0] as string, call[1] as Handler)
  }

  listRemoteDir = handlers.get('repo.listRemoteDirectory')!
  addRemote = handlers.get('repo.addRemote')!
  cloneRemote = handlers.get('repo.cloneRemote')!
  scanRemote = handlers.get('repo.scanRemote')!
})

// ── repo.listRemoteDirectory ──────────────────────────────────────────────────

describe('repo.listRemoteDirectory', () => {
  it('trả về directories trên dev server path', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    const expected = {
      entries: [
        { name: 'projects', path: '/home/alice/projects', isDirectory: true, isGitRepo: false }
      ],
      platform: 'linux'
    }
    mockCall.mockResolvedValue(expected)

    const result = await listRemoteDir(null, {
      devServerId: 'ds-1',
      path: '/home/alice',
      includeGitStatus: false
    })
    expect(mockCall).toHaveBeenCalledWith('fs.listDirectory', {
      path: '/home/alice',
      includeGitStatus: false
    })
    expect(result).toEqual(expected)
  })

  it('includeGitStatus = true → đánh dấu git repos', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    const expected = {
      entries: [
        { name: 'my-app', path: '/home/alice/my-app', isDirectory: true, isGitRepo: true }
      ],
      platform: 'linux'
    }
    mockCall.mockResolvedValue(expected)

    const result = (await listRemoteDir(null, {
      devServerId: 'ds-1',
      path: '/home/alice',
      includeGitStatus: true
    })) as typeof expected
    expect(result.entries[0].isGitRepo).toBe(true)
  })

  it('path không tồn tại → throw Error từ relay', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockRejectedValue(new Error('Cannot list directory /nonexistent: ENOENT'))

    await expect(
      listRemoteDir(null, { devServerId: 'ds-1', path: '/nonexistent' })
    ).rejects.toThrow('ENOENT')
  })

  it('relay không connected → throw Error', async () => {
    mockManagerGetRelay.mockReturnValue(null)
    await expect(
      listRemoteDir(null, { devServerId: 'ds-1', path: '/home/alice' })
    ).rejects.toThrow("Dev server 'ds-1' not connected")
  })
})

// ── repo.cloneRemote ──────────────────────────────────────────────────────────

describe('repo.cloneRemote', () => {
  const devServerId = 'ds-2'
  const mockServer = {
    id: devServerId,
    workspaceDir: '/home/alice/workspaces',
    sshTargetId: 'ssh-target-1'
  }

  it('gọi git.clone trên relay với url + targetPath', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockManagerGet.mockReturnValue(mockServer)
    mockCall.mockResolvedValue({ path: '/home/alice/workspaces/my-repo' })

    await cloneRemote(null, {
      devServerId,
      url: 'https://github.com/org/my-repo.git'
    })

    expect(mockCall).toHaveBeenCalledWith('git.clone', {
      url: 'https://github.com/org/my-repo.git',
      targetPath: '/home/alice/workspaces/my-repo'
    })
  })

  it('add repo vào store sau khi clone thành công', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockManagerGet.mockReturnValue(mockServer)
    mockCall.mockResolvedValue({ path: '/home/alice/workspaces/my-repo' })

    await cloneRemote(null, {
      devServerId,
      url: 'https://github.com/org/my-repo.git'
    })

    expect(mockAddRepo).toHaveBeenCalledTimes(1)
    const savedRepo = mockAddRepo.mock.calls[0][0]
    expect(savedRepo.devServerId).toBe(devServerId)
    expect(savedRepo.path).toBe('/home/alice/workspaces/my-repo')
  })

  it('targetDir mặc định = devServer.workspaceDir + repoName', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockManagerGet.mockReturnValue(mockServer)
    mockCall.mockResolvedValue({ path: '/home/alice/workspaces/my-repo' })

    const result = (await cloneRemote(null, {
      devServerId,
      url: 'https://github.com/org/my-repo.git'
    })) as { path: string }

    expect(result.path).toBe('/home/alice/workspaces/my-repo')
  })

  it('targetDir được cung cấp → dùng targetDir', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockManagerGet.mockReturnValue(mockServer)
    mockCall.mockResolvedValue({ path: '/custom/path/my-repo' })

    const result = (await cloneRemote(null, {
      devServerId,
      url: 'https://github.com/org/my-repo.git',
      targetDir: '/custom/path/my-repo'
    })) as { path: string }

    expect(result.path).toBe('/custom/path/my-repo')
    expect(mockCall).toHaveBeenCalledWith('git.clone', {
      url: 'https://github.com/org/my-repo.git',
      targetPath: '/custom/path/my-repo'
    })
  })
})

// ── repo.scanRemote ───────────────────────────────────────────────────────────

describe('repo.scanRemote', () => {
  const devServerId = 'ds-3'

  it('chỉ trả về entries có isGitRepo = true', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue({
      platform: 'linux',
      entries: [
        { name: 'my-app', path: '/home/alice/my-app', isDirectory: true, isGitRepo: true },
        { name: 'docs', path: '/home/alice/docs', isDirectory: true, isGitRepo: false },
        { name: 'backend', path: '/home/alice/backend', isDirectory: true, isGitRepo: true }
      ]
    })

    const result = (await scanRemote(null, {
      devServerId,
      rootPath: '/home/alice'
    })) as { path: string; name: string }[]

    expect(result).toHaveLength(2)
    expect(result.map((r) => r.name)).toEqual(['my-app', 'backend'])
  })

  it('0 git repos → trả về []', async () => {
    mockManagerGetRelay.mockReturnValue(mockRelay)
    mockCall.mockResolvedValue({
      platform: 'linux',
      entries: [
        { name: 'docs', path: '/home/alice/docs', isDirectory: true, isGitRepo: false }
      ]
    })

    const result = await scanRemote(null, { devServerId, rootPath: '/home/alice' })
    expect(result).toEqual([])
  })
})
