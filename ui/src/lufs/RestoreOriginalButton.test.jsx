import React from 'react'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { RestoreAllOriginalsButton } from './RestoreOriginalButton'

const mocks = vi.hoisted(() => ({
  notify: vi.fn(),
  follow: vi.fn(),
  publish: vi.fn(),
  restoreAll: vi.fn(),
}))

vi.mock('./useRestoreStatus', () => ({
  useRestoreStatus: () => ({
    status: null,
    follow: mocks.follow,
    publish: mocks.publish,
  }),
}))

vi.mock('react-admin', async () => ({
  ...(await vi.importActual('react-admin')),
  Button: ({ onClick, disabled, label }) => (
    <button onClick={onClick} disabled={disabled}>
      {label}
    </button>
  ),
  Confirm: ({ isOpen, onConfirm }) =>
    isOpen ? <button onClick={onConfirm}>confirm</button> : null,
  useNotify: () => mocks.notify,
  useTranslate: () => (value) => value,
  usePermissions: () => ({ permissions: 'admin' }),
  useRefresh: () => vi.fn(),
  useDataProvider: () => ({ restoreAllSongLoudness: mocks.restoreAll }),
}))

const confirmRestoreAll = () => {
  render(<RestoreAllOriginalsButton />)
  fireEvent.click(screen.getByText('resources.lufs.actions.restoreAll'))
  expect(mocks.restoreAll).not.toHaveBeenCalled()
  fireEvent.click(screen.getByText('confirm'))
}

describe('RestoreAllOriginalsButton', () => {
  beforeEach(() => vi.clearAllMocks())

  it('asks first, then follows the restore job and reports its result', async () => {
    const started = { running: true, total: 4, processed: 0 }
    mocks.restoreAll.mockResolvedValue({ data: started, started: true })
    confirmRestoreAll()
    await waitFor(() => expect(mocks.follow).toHaveBeenCalledOnce())
    // The progress bar gets "0 of 4" straight from the reply.
    expect(mocks.publish).toHaveBeenCalledWith(started)

    act(() =>
      mocks.follow.mock.calls[0][0]({
        running: false,
        total: 4,
        restored: 3,
        skipped: 0,
        failed: 1,
      }),
    )
    expect(mocks.notify).toHaveBeenLastCalledWith(
      'resources.song.notifications.lufsRestored',
      { type: 'warning', messageArgs: { restored: 3, skipped: 0, failed: 1 } },
    )
  })

  it('says the restore was stopped, with how many songs it did not reach', async () => {
    mocks.restoreAll.mockResolvedValue({
      data: { running: true, total: 608 },
      started: true,
    })
    confirmRestoreAll()
    await waitFor(() => expect(mocks.follow).toHaveBeenCalledOnce())

    act(() =>
      mocks.follow.mock.calls[0][0]({
        running: false,
        total: 608,
        processed: 152,
        restored: 150,
        skipped: 0,
        failed: 0,
        cancelled: 2,
      }),
    )
    expect(mocks.notify).toHaveBeenLastCalledWith(
      'resources.song.notifications.lufsRestoreStopped',
      {
        type: 'warning',
        messageArgs: { restored: 150, skipped: 0, failed: 0, notRestored: 458 },
      },
    )
  })

  it('says so when there is nothing to restore', async () => {
    mocks.restoreAll.mockResolvedValue({
      data: { message: 'No song has a stored original' },
      started: false,
    })
    confirmRestoreAll()
    await waitFor(() =>
      expect(mocks.notify).toHaveBeenCalledWith(
        'No song has a stored original',
        { type: 'info' },
      ),
    )
    expect(mocks.follow).not.toHaveBeenCalled()
  })
})
