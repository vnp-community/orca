// @vitest-environment happy-dom
import '@testing-library/jest-dom/vitest'
import { render, screen, cleanup } from '@testing-library/react'
import { describe, expect, it, afterEach } from 'vitest'
import { ProfileSourceBadge } from '../ProfileSourceBadge'

describe('ProfileSourceBadge', () => {
  afterEach(cleanup)

  it('company source => default badge', () => {
    render(<ProfileSourceBadge source="company" />)
    expect(screen.getByText(/Company/i)).toBeInTheDocument()
  })

  it('dept source => secondary badge', () => {
    render(<ProfileSourceBadge source="dept" />)
    expect(screen.getByText(/Dept/i)).toBeInTheDocument()
  })

  it('locked=true => shows Lock icon + "Company Only" text', () => {
    render(<ProfileSourceBadge source="company" locked />)
    expect(screen.getByText(/Company Only/i)).toBeInTheDocument()
  })
})
