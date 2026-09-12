import React from 'react'
import { fireEvent, render, screen, waitFor, act } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { OptimizeLufsButton } from './OptimizeLufsButton'

const mocks = vi.hoisted(() => ({
  notify: vi.fn(),
  follow: vi.fn(),
  optimize: vi.fn().mockResolvedValue({}),
}))

vi.mock('../lufs/useJobStatus', () => ({
  useJobStatus: () => ({ follow: mocks.follow }),
}))

vi.mock('react-admin', async () => ({
  ...(await vi.importActual('react-admin')),
  Button: ({ onClick, disabled, children }) => (
    <button onClick={onClick} disabled={disabled}>
      Optimise{children}
    </button>
  ),
  useNotify: () => mocks.notify,
  useTranslate: () => (value) => value,
  usePermissions: () => ({ permissions: 'admin' }),
  useRefresh: () => vi.fn(),
  useUnselectAll: () => vi.fn(),
  useDataProvider: () => ({ optimizeSongLoudness: mocks.optimize }),
}))

describe('optimisation completion notification', () => {
  it('warns about rejected songs even when no processing errors occurred', async () => {
    render(<OptimizeLufsButton resource="song" selectedIds={['a', 'b']} />)
    fireEvent.click(screen.getByRole('button'))
    await waitFor(() => expect(mocks.follow).toHaveBeenCalledOnce())
    expect(mocks.notify).not.toHaveBeenCalledWith(
      'resources.song.notifications.lufsOptimized',
      expect.anything(),
    )

    act(() =>
      mocks.follow.mock.calls[0][0]({
        running: false,
        normalized: 1,
        rejected: 1,
        skipped: 0,
        failed: 0,
      }),
    )
    expect(mocks.notify).toHaveBeenLastCalledWith(
      'resources.song.notifications.lufsOptimized',
      {
        type: 'warning',
        messageArgs: { normalized: 1, rejected: 1, skipped: 0, failed: 0 },
      },
    )
  })
})
