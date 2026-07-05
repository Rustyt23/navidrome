import React from 'react'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
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

describe('PlaylistAiToolPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetList.mockResolvedValue({ data: [playlist], total: 1 })
    mockHttpClient.mockResolvedValue({ json: analysis })
  })

  it('renders the playlist intelligence table', async () => {
    render(<PlaylistAiToolPage />)

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
    render(<PlaylistAiToolPage />)

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

  it('analyzes a playlist and renders its report', async () => {
    render(<PlaylistAiToolPage />)
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
