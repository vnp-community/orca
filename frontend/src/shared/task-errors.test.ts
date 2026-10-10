import { describe, expect, it } from 'vitest'
import { isTaskNotFoundError } from './task-errors'

describe('isTaskNotFoundError', () => {
  it.each([
    [{ code: 'runtime_error', message: 'TASK_NOT_FOUND: task not found' }],
    [{ code: 'runtime_error', message: 'rpc error: code = NotFound desc = task not found' }],
    [{ code: 'NOT_FOUND', message: 'x' }],
    [{ code: 'runtime_error', message: 'NOT_FOUND' }],
    [new Error('TASK_NOT_FOUND: no such task')]
  ])('accepts %o', (err) => {
    expect(isTaskNotFoundError(err)).toBe(true)
  })

  it.each([
    [{ code: 'runtime_error', message: 'TASK_DELETE_FAILED: failed to delete task' }],
    [{ code: 'runtime_error', message: 'TASK_NOT_FOUND_BY_NUMBER: no task with this number' }],
    [{ code: 'runtime_error', message: 'PROJECT_NOT_FOUND: project' }],
    [{ code: 'runtime_error', message: 'REQUEST_NOT_FOUND: x' }],
    [{ code: 'runtime_error', message: 'TASK_MISSING_ID: id is required' }],
    [{ code: 'runtime_error', message: 'rpc error: code = Internal desc = boom' }],
    ['TASK_NOT_FOUND'],
    [null],
    [undefined]
  ])('rejects %o', (err) => {
    expect(isTaskNotFoundError(err)).toBe(false)
  })
})
