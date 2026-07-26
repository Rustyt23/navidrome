import React from 'react'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PlaylistHistoryPanel from './PlaylistHistoryPanel'

const { mockHttpClient } = vi.hoisted(() => ({
  mockHttpClient: vi.fn(),
}))

vi.mock('../dataProvider', () => ({ httpClient: mockHttpClient }))

const version = {
  id: 'version-1',
  playlistId: 'playlist-1',
  version: 1,
  draftId: 'draft-1',
  publishedBy: 'publisher',
  publishedAt: '2026-07-23T12:00:00Z',
  approvedBy: 'reviewer',
  approvedAt: '2026-07-23T11:55:00Z',
  aiModelVersion: 'embedding-v2',
  aiIndexVersion: '3',
  rulesetVersion: 'playlist-recommendation-v1',
  changeSummary: {
    added: 0,
    removed: 0,
    replaced: 1,
    reordered: 0,
    aiSelected: 1,
    manualSelected: 0,
  },
  before: [
    { mediaFileId: 'old', title: 'Old Song', artist: 'Artist', position: 0 },
  ],
  after: [
    { mediaFileId: 'new', title: 'New Song', artist: 'Artist 2', position: 0 },
  ],
  changeDetails: [
    {
      kind: 'replace',
      mediaFileId: 'new',
      title: 'New Song',
      replacedId: 'old',
      replacedTitle: 'Old Song',
      source: 'ai',
      aiSelected: true,
      reason: 'Cleaner replacement',
    },
  ],
}

describe('PlaylistHistoryPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockHttpClient.mockImplementation((url) => {
      if (url.startsWith('/api/playlist-history/audit?')) {
        return Promise.resolve({
          json: {
            events: [
              {
                id: 'event-1',
                eventType: 'published',
                actor: 'publisher',
                summary: 'Playlist version 1 published',
                versionNumber: 1,
                createdAt: '2026-07-23T12:00:00Z',
              },
            ],
          },
        })
      }
      if (url.startsWith('/api/playlist-history?')) {
        return Promise.resolve({ json: { versions: [version] } })
      }
      if (url === '/api/playlist-history/version-1/rollback') {
        return Promise.resolve({
          json: {
            id: 'rollback-draft',
            status: 'draft',
            rollbackVersionNumber: 1,
          },
        })
      }
      return Promise.resolve({ json: {} })
    })
  })

  it('shows immutable versions, approval, provenance, comparison and audit', async () => {
    render(
      <PlaylistHistoryPanel
        playlist={{ id: 'playlist-1', name: 'Store Mix' }}
        refreshToken={0}
      />,
    )

    const history = await screen.findByRole('table', {
      name: 'Playlist version history',
    })
    expect(within(history).getByText('Version 1')).toBeInTheDocument()
    expect(within(history).getByText('publisher')).toBeInTheDocument()
    expect(within(history).getByText('reviewer')).toBeInTheDocument()
    expect(within(history).getByText('1 replaced')).toBeInTheDocument()
    expect(within(history).getByText('1 AI / 0 manual')).toBeInTheDocument()
    expect(within(history).getByText('Model: embedding-v2')).toBeInTheDocument()

    expect(screen.getByText('Old Song — Artist')).toBeInTheDocument()
    expect(screen.getByText('New Song — Artist 2')).toBeInTheDocument()
    expect(screen.getByText(/AI-generated/)).toBeInTheDocument()

    const audit = screen.getByRole('table', {
      name: 'Playlist audit events',
    })
    expect(within(audit).getByText('Published')).toBeInTheDocument()
    expect(
      within(audit).getByText('Playlist version 1 published'),
    ).toBeInTheDocument()
  })

  it('creates a rollback draft without calling a live playlist mutation', async () => {
    const onRollbackCreated = vi.fn()
    render(
      <PlaylistHistoryPanel
        playlist={{ id: 'playlist-1', name: 'Store Mix' }}
        refreshToken={0}
        onRollbackCreated={onRollbackCreated}
      />,
    )

    fireEvent.click(
      await screen.findByRole('button', { name: 'Create rollback draft' }),
    )
    await waitFor(() =>
      expect(mockHttpClient).toHaveBeenCalledWith(
        '/api/playlist-history/version-1/rollback',
        { method: 'POST' },
      ),
    )
    expect(onRollbackCreated).toHaveBeenCalledWith(
      expect.objectContaining({
        id: 'rollback-draft',
        rollbackVersionNumber: 1,
      }),
    )
    expect(
      mockHttpClient.mock.calls.some(([url]) =>
        /^\/api\/playlist(?:\/|$)/.test(url),
      ),
    ).toBe(false)
  })
})
