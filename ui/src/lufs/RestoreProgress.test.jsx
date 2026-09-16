import React from 'react'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RestoreProgress } from './RestoreProgress'

const mocks = vi.hoisted(() => ({ status: null }))

vi.mock('./useRestoreStatus', () => ({
  RESTORE_URL: '/api/song/loudness/restore',
  useRestoreStatus: () => ({ status: mocks.status }),
}))

vi.mock('./LufsJobButtons', () => ({
  StopLufsJobButton: ({ url, label, disabled }) => (
    <button data-url={url} disabled={disabled}>
      {label}
    </button>
  ),
}))

describe('RestoreProgress', () => {
  beforeEach(() => {
    mocks.status = null
  })

  it('shows how many songs are done out of how many, and a stop button', () => {
    mocks.status = {
      running: true,
      total: 608,
      processed: 150,
      restored: 147,
      skipped: 2,
      failed: 1,
    }
    render(<RestoreProgress />)

    expect(screen.getByText('Restoring songs 150 / 608')).toBeInTheDocument()
    expect(screen.getByText('25%')).toBeInTheDocument()
    expect(
      screen.getByText('147 restored · 2 had no stored original · 1 failed'),
    ).toBeInTheDocument()
    const stop = screen.getByText('resources.lufs.actions.stopRestore')
    expect(stop).toHaveAttribute('data-url', '/api/song/loudness/restore/stop')
    expect(stop).not.toBeDisabled()
  })

  it('keeps the stop button pressed once a stop is on its way', () => {
    mocks.status = { running: true, stopping: true, total: 10, processed: 4 }
    render(<RestoreProgress />)
    expect(
      screen.getByText('resources.lufs.actions.stopRestore'),
    ).toBeDisabled()
  })

  it('shows nothing when no restore is running', () => {
    mocks.status = { running: false, total: 10, processed: 10, restored: 10 }
    const { container } = render(<RestoreProgress />)
    expect(container).toBeEmptyDOMElement()
  })
})
