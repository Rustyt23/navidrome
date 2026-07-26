import React from 'react'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { MemoryRouter } from 'react-router-dom'
import PlaylistAiToolPage from './PlaylistAiToolPage'

const { mockGetList, mockHttpClient } = vi.hoisted(() => ({
  mockGetList: vi.fn(),
  mockHttpClient: vi.fn(),
}))

vi.mock('../dataProvider', () => ({ httpClient: mockHttpClient }))

vi.mock('react-admin', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    Title: () => null,
    // Return a new wrapper on each render to match React Admin providers that
    // do not keep method identity stable.
    useDataProvider: () => ({
      getList: (...args) => mockGetList(...args),
    }),
  }
})

const playlist = {
  id: 'playlist-1',
  name: 'Store Mix',
  ownerName: 'Owner',
  public: false,
  songCount: 4,
  duration: 720,
  updatedAt: '2026-07-05T00:00:00Z',
}

const analysis = {
  playlistId: playlist.id,
  name: playlist.name,
  summary: 'Four-song retail mix with one explicit-risk song.',
  songCount: 4,
  duration: 720,
  genreSummary: [{ name: 'Pop', count: 4, percent: 100 }],
  artistSummary: [{ name: 'Artist', count: 2, percent: 50 }],
  explicitRisk: {
    level: 'high',
    count: 1,
    percent: 25,
    songs: [{ songId: 'risk', title: 'Risk', artist: 'Artist' }],
  },
  metadataIssues: [
    {
      songId: 'missing',
      title: 'Missing',
      artist: 'Artist',
      missing: ['LUFS'],
    },
  ],
  duplicateSongs: [],
  loudnessIssues: [],
  badFitSongs: [],
  suggestedReplacements: [
    {
      forSong: { songId: 'risk', title: 'Risk', artist: 'Artist' },
      reasons: ['explicit-risk song'],
      suggestions: [
        {
          songId: 'safe',
          title: 'Safe Song',
          artist: 'Other Artist',
          genre: 'Pop',
          bpm: 120,
          lufs: -12,
        },
      ],
    },
  ],
  recommendations: ['Review the explicit-risk song.'],
}

const recommendationResponse = {
  type: 'underused_songs',
  summary: 'Clean songs with low play counts.',
  count: 1,
  results: [
    {
      songId: 'underused-1',
      title: 'Hidden Gem',
      artist: 'New Artist',
      album: 'New Album',
      genre: 'Pop',
      year: 2024,
      explicit: false,
      explicitStatus: 'clean',
      bpm: 118,
      lufs: -12.2,
      playCount: 3,
      score: 0.91,
      reason: 'Underused clean track with 3 plays and complete metadata',
    },
  ],
}

describe('PlaylistAiToolPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetList.mockResolvedValue({ data: [playlist], total: 1 })
    mockHttpClient.mockResolvedValue({ json: analysis })
  })

  it('renders the playlist intelligence table', async () => {
    render(
      <MemoryRouter initialEntries={['/playlist-ai-tool']}>
        <PlaylistAiToolPage />
      </MemoryRouter>,
    )

    expect(await screen.findByText('Store Mix')).toBeInTheDocument()
    expect(screen.getAllByText('Owner')).toHaveLength(2)
    expect(screen.getByText('Private')).toBeInTheDocument()
    expect(screen.getByText('AI analysis status')).toBeInTheDocument()
    expect(screen.getByText('Not analyzed')).toBeInTheDocument()
    expect(mockGetList).toHaveBeenCalledTimes(1)
    expect(mockHttpClient).not.toHaveBeenCalled()
    expect(mockGetList).toHaveBeenCalledWith('playlist', {
      pagination: { page: 1, perPage: 500 },
      sort: { field: 'name', order: 'ASC' },
      filter: {},
    })
  })

  it('indexes playlists only after the index button is clicked', async () => {
    mockHttpClient.mockResolvedValueOnce({
      json: {
        playlists: { indexed: 1, skipped: 0, failed: 0 },
      },
    })
    render(
      <MemoryRouter initialEntries={['/playlist-ai-tool']}>
        <PlaylistAiToolPage />
      </MemoryRouter>,
    )

    const indexButton = await screen.findByRole('button', {
      name: 'Index playlists',
    })
    expect(mockHttpClient).not.toHaveBeenCalled()
    fireEvent.click(indexButton)

    await waitFor(() =>
      expect(mockHttpClient).toHaveBeenCalledWith('/api/ai/rag/index', {
        method: 'POST',
        body: JSON.stringify({
          includeSongs: false,
          includePlaylists: true,
          playlistLimit: 1,
          force: false,
        }),
      }),
    )
    expect(
      await screen.findByText('Indexed 1 playlists, skipped 0, failed 0.'),
    ).toBeInTheDocument()
  })

  it('renders actionable recommendation controls without a selected playlist', async () => {
    mockHttpClient.mockResolvedValueOnce({ json: recommendationResponse })
    render(
      <MemoryRouter initialEntries={['/playlist-ai-tool']}>
        <PlaylistAiToolPage />
      </MemoryRouter>,
    )

    expect(await screen.findByText('Store Mix')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Get Recommendations' }),
    ).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: 'Underused Songs' }))

    await waitFor(() =>
      expect(mockHttpClient).toHaveBeenCalledWith('/api/ai/rag/recommend', {
        method: 'POST',
        body: JSON.stringify({ type: 'underused_songs', limit: 20 }),
      }),
    )
    const table = await screen.findByRole('table', {
      name: 'Recommendation results',
    })
    expect(within(table).getByText('Hidden Gem')).toBeInTheDocument()
    expect(within(table).getByText('0.910')).toBeInTheDocument()
    expect(within(table).getByText('Clean')).toBeInTheDocument()
    expect(within(table).getByRole('button', { name: 'Preview' })).toBeEnabled()
    for (const action of ['Add', 'Replace', 'Remove', 'Ignore', 'Block']) {
      expect(within(table).getByRole('button', { name: action })).toBeDisabled()
    }
  })

  // A song nobody has classified must never be presented as Clean: that is what
  // let unrated songs pass as retail-safe.
  it('shows an unclassified song as Unknown rather than Clean', async () => {
    mockHttpClient.mockImplementation((url) => {
      if (url === '/api/ai/rag/recommend') {
        return Promise.resolve({
          json: {
            ...recommendationResponse,
            results: [
              {
                ...recommendationResponse.results[0],
                explicitStatus: 'unknown',
              },
            ],
          },
        })
      }
      return Promise.resolve({ json: {} })
    })
    render(
      <MemoryRouter initialEntries={['/playlist-ai-tool']}>
        <PlaylistAiToolPage />
      </MemoryRouter>,
    )

    expect(await screen.findByText('Store Mix')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Retail-safe Picks' }))

    const table = await screen.findByRole('table', {
      name: 'Recommendation results',
    })
    expect(within(table).getByText('Unknown')).toBeInTheDocument()
    expect(within(table).queryByText('Clean')).not.toBeInTheDocument()
  })

  it('adds an accepted recommendation to the selected draft, never the live playlist', async () => {
    const draft = {
      id: 'draft-1',
      playlistId: playlist.id,
      name: 'AI working draft',
      status: 'draft',
      proposedTrackIds: ['current-1'],
      changes: [],
    }
    mockHttpClient.mockImplementation((url, options) => {
      if (url.startsWith('/api/playlist-draft?')) {
        return Promise.resolve({ json: { drafts: [draft] } })
      }
      if (url === '/api/playlist-draft/draft-1/diff') {
        return Promise.resolve({
          json: {
            draftId: draft.id,
            before: [
              {
                mediaFileId: 'current-1',
                title: 'Current Song',
                artist: 'Artist',
                position: 0,
              },
            ],
            after: [
              {
                mediaFileId: 'current-1',
                title: 'Current Song',
                artist: 'Artist',
                position: 0,
              },
            ],
            added: [],
            removed: [],
            moved: [],
            unchanged: 1,
          },
        })
      }
      if (url === '/api/ai/rag/recommend') {
        return Promise.resolve({ json: recommendationResponse })
      }
      if (url === '/api/playlist-draft/draft-1/tracks') {
        return Promise.resolve({
          json: {
            ...draft,
            proposedTrackIds: ['current-1', 'underused-1'],
          },
        })
      }
      return Promise.resolve({ json: options ? {} : analysis })
    })

    render(
      <MemoryRouter initialEntries={['/playlist-ai-tool']}>
        <PlaylistAiToolPage />
      </MemoryRouter>,
    )
    fireEvent.click(
      await screen.findByRole('checkbox', { name: 'Select Store Mix' }),
    )
    expect(await screen.findByText('AI working draft')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Underused Songs' }))
    const table = await screen.findByRole('table', {
      name: 'Recommendation results',
    })
    fireEvent.click(within(table).getByRole('button', { name: 'Add' }))

    await waitFor(() =>
      expect(mockHttpClient).toHaveBeenCalledWith(
        '/api/playlist-draft/draft-1/tracks',
        {
          method: 'PUT',
          body: JSON.stringify({
            operations: [
              {
                kind: 'add',
                mediaFileId: 'underused-1',
                reason:
                  'Underused clean track with 3 plays and complete metadata',
                source: 'ai',
                confidence: 91,
              },
            ],
          }),
        },
      ),
    )
    expect(
      mockHttpClient.mock.calls.some(([url]) =>
        /^\/api\/playlist(?:\/|$)/.test(url),
      ),
    ).toBe(false)
    expect(
      await screen.findByText(/live playlist was not changed/i),
    ).toBeInTheDocument()
  })

  it('shows the backend reason when recommendations are unavailable', async () => {
    mockHttpClient.mockRejectedValueOnce({
      message: 'Service Unavailable',
      body: { error: 'Qdrant unavailable: connection refused' },
    })
    render(
      <MemoryRouter initialEntries={['/playlist-ai-tool']}>
        <PlaylistAiToolPage />
      </MemoryRouter>,
    )

    expect(await screen.findByText('Store Mix')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Retail-safe Picks' }))

    expect(
      await screen.findByText('Qdrant unavailable: connection refused'),
    ).toBeInTheDocument()
    expect(screen.queryByText('Service Unavailable')).not.toBeInTheDocument()
  })

  it('analyzes a playlist and renders its report', async () => {
    render(
      <MemoryRouter initialEntries={['/playlist-ai-tool']}>
        <PlaylistAiToolPage />
      </MemoryRouter>,
    )
    fireEvent.click(
      await screen.findByRole('button', { name: 'Analyze playlist' }),
    )

    await waitFor(() =>
      expect(mockHttpClient).toHaveBeenCalledWith(
        '/api/ai/rag/playlist/analyze',
        {
          method: 'POST',
          body: JSON.stringify({ playlistId: playlist.id }),
        },
      ),
    )

    const report = await screen.findByRole('dialog', {
      name: /Playlist AI report — Store Mix/,
    })
    expect(within(report).getByText(analysis.summary)).toBeInTheDocument()
    expect(within(report).getByText(/Safe Song/)).toBeInTheDocument()
    fireEvent.click(within(report).getByRole('button', { name: 'Close' }))
    expect(
      await screen.findByRole('button', { name: 'Re-analyze playlist' }),
    ).toBeInTheDocument()
  })
})
