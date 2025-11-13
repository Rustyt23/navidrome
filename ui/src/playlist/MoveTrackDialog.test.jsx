import React from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import MoveTrackDialog from './MoveTrackDialog'

vi.mock('react-admin', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    useTranslate: () => (key) => key,
  }
})

describe('MoveTrackDialog', () => {
  const defaultProps = {
    open: true,
    record: { id: '4', title: 'Test Song' },
    onClose: vi.fn(),
    onSubmit: vi.fn(),
    maxPosition: 10,
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('prefills the current position when provided', () => {
    render(
      <MoveTrackDialog
        {...defaultProps}
        currentPosition={3}
      />,
    )

    const input = screen.getByLabelText(
      'resources.playlist.dialog.moveTrackPositionLabel',
    )
    expect(input).toHaveValue(3)
  })

  it('submits the new position when confirmed', () => {
    const handleSubmit = vi.fn()
    render(
      <MoveTrackDialog
        {...defaultProps}
        onSubmit={handleSubmit}
        currentPosition={2}
      />,
    )

    const input = screen.getByLabelText(
      'resources.playlist.dialog.moveTrackPositionLabel',
    )
    fireEvent.change(input, { target: { value: '5' } })

    fireEvent.click(
      screen.getByRole('button', {
        name: 'resources.playlist.dialog.moveTrackConfirm',
      }),
    )

    expect(handleSubmit).toHaveBeenCalledWith(5)
  })

  it('disables confirmation when the value is invalid', () => {
    render(
      <MoveTrackDialog
        {...defaultProps}
        currentPosition={2}
      />,
    )

    const input = screen.getByLabelText(
      'resources.playlist.dialog.moveTrackPositionLabel',
    )
    fireEvent.change(input, { target: { value: '0' } })

    const confirmButton = screen.getByRole('button', {
      name: 'resources.playlist.dialog.moveTrackConfirm',
    })
    expect(confirmButton).toBeDisabled()
  })
})
