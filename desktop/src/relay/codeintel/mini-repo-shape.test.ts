/**
 * Quyết định về đuôi tệp:
 * Để tránh việc tsc và oxlint duyệt nhầm mã nguồn TypeScript mẫu trong __fixtures__, 
 * các tệp TypeScript mẫu được đặt đuôi .ts.fixture (sẽ được đổi tên khi sao ra thư mục tạm ở task 03).
 * Các tệp Go, SQL giữ nguyên đuôi vì Go toolchain không tự động duyệt tệp ở ngoài module, 
 * và chúng ta có thể cô lập.
 */
import { describe, it, expect } from 'vitest'
import fs from 'fs'
import path from 'path'

describe('mini-repo shape', () => {
  const repoDir = path.join(__dirname, '__fixtures__', 'mini-repo')

  function getAllFiles(dirPath: string, arrayOfFiles: string[] = []) {
    const files = fs.readdirSync(dirPath)

    files.forEach(function (file) {
      if (fs.statSync(dirPath + '/' + file).isDirectory()) {
        arrayOfFiles = getAllFiles(dirPath + '/' + file, arrayOfFiles)
      } else {
        arrayOfFiles.push(path.join(dirPath, '/', file))
      }
    })

    return arrayOfFiles
  }

  it('has 15 to 25 files', () => {
    const files = getAllFiles(repoDir)
    expect(files.length).toBeGreaterThanOrEqual(15)
    expect(files.length).toBeLessThanOrEqual(25)
  })

  it('total size is <= 200 KiB', () => {
    const files = getAllFiles(repoDir)
    let totalSize = 0
    for (const f of files) {
      totalSize += fs.statSync(f).size
    }
    expect(totalSize).toBeLessThanOrEqual(200 * 1024)
  })

  it('has duplicate symbol name (User) in different files', () => {
    const goUserPath = path.join(repoDir, 'internal', 'domain', 'user.go')
    const tsUserPath = path.join(repoDir, 'ui', 'User.ts.fixture')
    
    expect(fs.existsSync(goUserPath)).toBe(true)
    expect(fs.existsSync(tsUserPath)).toBe(true)
    
    const goContent = fs.readFileSync(goUserPath, 'utf8')
    const tsContent = fs.readFileSync(tsUserPath, 'utf8')
    
    expect(goContent).toContain('type User struct')
    expect(tsContent).toContain('export interface User')
  })

  it('has strings with |, \\n, quotes', () => {
    const constPath = path.join(repoDir, 'internal', 'domain', 'constants.go')
    expect(fs.existsSync(constPath)).toBe(true)
    const content = fs.readFileSync(constPath, 'utf8')
    expect(content).toContain('|')
    expect(content).toContain('\\n')
    expect(content).toContain("'quotes'")
  })

  it('has file with spaces in name', () => {
    const spacePath = path.join(repoDir, 'space file.txt')
    expect(fs.existsSync(spacePath)).toBe(true)
  })

  it('has no .git, .gitnexus, .codegraph directories', () => {
    expect(fs.existsSync(path.join(repoDir, '.git'))).toBe(false)
    expect(fs.existsSync(path.join(repoDir, '.gitnexus'))).toBe(false)
    expect(fs.existsSync(path.join(repoDir, '.codegraph'))).toBe(false)
  })

  it('has no token leaks', () => {
    const files = getAllFiles(repoDir)
    const tokenRegex = /(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36}|[A-Za-z0-9+/]{40,}/
    
    for (const f of files) {
      const content = fs.readFileSync(f, 'utf8')
      // Only check simple regex for fake tokens since this is just a fixture test.
      // Dòng rất dài có chữ 'a' lặp 2000 lần sẽ bị match bởi [A-Za-z0-9+/]{40,}
      // Nên ta sẽ exclude file long_line.go khỏi token leak test, 
      // hoặc dùng regex nghiêm ngặt hơn.
      if (!f.endsWith('long_line.go')) {
        expect(content).not.toMatch(/(ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{36}/)
        // A generic high-entropy secret regex is harder, so we stick to known prefixes for this simple check.
      }
    }
  })
})
