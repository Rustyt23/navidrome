import React from 'react'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ThemeProvider, createTheme } from '@material-ui/core/styles'
import SetDecisionButton from './DecisionButtons'

const mocks = vi.hoisted(() => ({
  httpClient: vi.fn(),
  notify: vi.fn(),
  refresh: vi.fn(),
  unselectAll: vi.fn(),
}))

vi.mock('../dataProvider', () => ({ httpClient: mocks.httpClient }))
vi.mock('react-admin', async (importOriginal) => ({
  ...(await importOriginal()),
  useNotify: () => mocks.notify,
  useRefresh: () => mocks.refresh,
  useUnselectAll: () => mocks.unselectAll,
  useTranslate: () => (key) =>
    ({
      'resources.lufs2.actions.gain_ceiling': 'Gain to ceiling',
      'resources.lufs2.actions.skip': 'Leave alone',
    })[key] || key,
}))

afterEach(cleanup)
beforeEach(() => {
  vi.clearAllMocks()
  mocks.httpClient.mockResolvedValue({ json: { updated: 1 } })
})

const renderButton = (decision = 'gain_ceiling') =>
  render(
    <ThemeProvider theme={createTheme()}>
      <SetDecisionButton
        decision={decision}
        selectedIds={['song-1', 'song-2']}
        resource="lufs2"
      />
    </ThemeProvider>,
  )

describe('Saving a decision', () => {
  it.each([
    ['gain_ceiling', 'Gain to ceiling'],
    ['skip', 'Leave alone'],
  ])('saves %s without starting processing', async (decision, label) => {
    renderButton(decision)
    fireEvent.click(screen.getByRole('button', { name: label }))

    await waitFor(() => expect(mocks.refresh).toHaveBeenCalledOnce())

    expect(mocks.httpClient).toHaveBeenCalledOnce()
    const [url, request] = mocks.httpClient.mock.calls[0]
    expect(url).toBe('/api/song/loudness/decision')
    expect(request.method).toBe('PUT')
    expect(JSON.parse(request.body)).toEqual({
      ids: ['song-1', 'song-2'],
      decision,
    })
    expect(mocks.notify).toHaveBeenCalledWith(
      'resources.lufs2.notifications.decisionSaved',
      'info',
      { smart_count: 1 },
    )
    expect(mocks.unselectAll).toHaveBeenCalledWith('lufs2')
    await waitFor(() =>
      expect(screen.getByRole('button', { name: label })).toBeEnabled(),
    )
  })

  it('keeps a failed save retryable without clearing the selection', async () => {
    mocks.httpClient.mockRejectedValueOnce(new Error('Could not save choice'))
    renderButton()
    const button = screen.getByRole('button', { name: 'Gain to ceiling' })
    fireEvent.click(button)

    await waitFor(() => expect(button).toBeEnabled())

    expect(mocks.notify).toHaveBeenCalledWith(
      'Could not save choice',
      'warning',
    )
    expect(mocks.unselectAll).not.toHaveBeenCalled()
    expect(mocks.refresh).not.toHaveBeenCalled()

    fireEvent.click(button)

    await waitFor(() => expect(mocks.refresh).toHaveBeenCalledOnce())
    expect(mocks.httpClient).toHaveBeenCalledTimes(2)
    expect(mocks.notify).toHaveBeenLastCalledWith(
      'resources.lufs2.notifications.decisionSaved',
      'info',
      { smart_count: 1 },
    )
  })
})
