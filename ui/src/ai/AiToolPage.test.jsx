import React from 'react'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AiToolPage from './AiToolPage'

const { mockHttpClient, mockGetList } = vi.hoisted(() => ({
  mockHttpClient: vi.fn(),
  mockGetList: vi.fn(),
}))

vi.mock('../dataProvider', () => ({
  httpClient: mockHttpClient,
}))

vi.mock('react-admin', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    Title: () => null,
    useDataProvider: () => ({ getList: mockGetList }),
    useTranslate: () => (key, options) => options?._ || key,
  }
})

const songs = [
  { id: 'song-1', title: 'First song', artist: 'Artist' },
  { id: 'song-2', title: 'Second song', artist: 'Artist' },
]

const createAbortableRequest = (signals) => (_url, options) =>
  new Promise((_resolve, reject) => {
    signals.push(options.signal)
    options.signal.addEventListener('abort', () => {
      const error = new Error('Aborted')
      error.name = 'AbortError'
      reject(error)
    })
  })

const defaultRAGStatus = {
  enabled: true,
  vectorUrl: 'http://vector.test:6333',
  collection: 'test_songs',
  topK: 12,
  vectorDbOnline: true,
  collectionExists: true,
  indexedCount: 42,
}

const renderPage = (
  actionPath,
  actionRequest,
  ragStatus = defaultRAGStatus,
) => {
  mockHttpClient.mockImplementation((url, options = {}) => {
    if (url === '/api/ai/status') {
      return Promise.resolve({
        json: { services: [], whisperModel: 'large-v3' },
      })
    }
    if (url === '/api/ai/rag/status') {
      return Promise.resolve({ json: ragStatus })
    }
    if (url.startsWith('/api/song?')) {
      return Promise.resolve({ json: songs })
    }
    if (url.startsWith(actionPath)) {
      return actionRequest(url, options)
    }
    return Promise.resolve({ json: {} })
  })

  render(<AiToolPage />)
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Select all added songs' }),
  )
}

const chooseModel = async (model) => {
  const dialog = await screen.findByRole('dialog')
  fireEvent.mouseDown(
    within(dialog).getByRole('button', { name: 'Gemma 3:4b' }),
  )
  fireEvent.click(await screen.findByRole('option', { name: model }))
  return dialog
}

describe('AiToolPage AI actions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(songs))
    mockGetList.mockResolvedValue({ data: songs })
  })

  afterEach(() => {
    cleanup()
  })

  it('stops an in-progress lyrics fetch', async () => {
    const signals = []
    renderPage('/api/ai/songs/', createAbortableRequest(signals))

    fireEvent.click(screen.getByRole('button', { name: 'Fetch Lyrics' }))
    const stopButton = await screen.findByRole('button', { name: 'Stop' })
    expect(signals).toHaveLength(1)

    fireEvent.click(stopButton)

    await waitFor(() => expect(signals[0].aborted).toBe(true))
    expect(await screen.findByText(/Stopped/)).toBeInTheDocument()
    expect(signals).toHaveLength(1)
  })

  it('shows the read-only RAG configuration status', async () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    const status = await screen.findByRole('region', { name: 'RAG status' })
    expect(within(status).getByText('Enabled')).toBeInTheDocument()
    expect(
      within(status).getByText('Vector URL: http://vector.test:6333'),
    ).toBeInTheDocument()
    expect(
      within(status).getByText('Collection: test_songs'),
    ).toBeInTheDocument()
    expect(within(status).getByText('Online')).toBeInTheDocument()
    expect(within(status).getByText('Exists')).toBeInTheDocument()
    expect(within(status).getByText('Indexed: 42')).toBeInTheDocument()
    expect(within(status).getByText('Top K: 12')).toBeInTheDocument()
    expect(mockHttpClient).toHaveBeenCalledWith('/api/ai/rag/status')
  })

  it('shows Qdrant offline and its error message', async () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }), {
      ...defaultRAGStatus,
      vectorDbOnline: false,
      collectionExists: false,
      indexedCount: 0,
      error: 'Qdrant unavailable: connection refused',
    })

    const status = await screen.findByRole('region', { name: 'RAG status' })
    expect(within(status).getByText('Enabled')).toBeInTheDocument()
    expect(within(status).getByText('Offline')).toBeInTheDocument()
    expect(within(status).getByText('Missing')).toBeInTheDocument()
    expect(within(status).getByText('Indexed: 0')).toBeInTheDocument()
    expect(
      within(status).getByText('Qdrant unavailable: connection refused'),
    ).toBeInTheDocument()
  })

  it('shows the disabled RAG state', async () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }), {
      ...defaultRAGStatus,
      enabled: false,
      vectorDbOnline: false,
      collectionExists: false,
      indexedCount: 0,
    })

    const status = await screen.findByRole('region', { name: 'RAG status' })
    expect(within(status).getByText('Disabled')).toBeInTheDocument()
    expect(within(status).getByText('Offline')).toBeInTheDocument()
    expect(within(status).getByText('Missing')).toBeInTheDocument()
    expect(
      within(status).getByRole('spinbutton', { name: 'Songs to index' }),
    ).toHaveValue(50)
    expect(
      within(status).getByRole('button', { name: 'Index 50 songs' }),
    ).toBeDisabled()
  })

  it('toggles RAG off at runtime', async () => {
    const requests = []
    renderPage('/api/ai/rag/enabled', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { enabled: false } })
    })

    const status = await screen.findByRole('region', { name: 'RAG status' })
    fireEvent.click(within(status).getByRole('button', { name: 'Disable RAG' }))

    await waitFor(() => expect(requests).toEqual([{ enabled: false }]))
    expect(
      within(status).getByRole('button', { name: 'Enable RAG' }),
    ).toBeInTheDocument()
    expect(within(status).getByText('Disabled')).toBeInTheDocument()
  })

  it('selects the default Whisper model', async () => {
    const requests = []
    renderPage('/api/ai/whisper/model', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { model: 'small' } })
    })

    const selector = await screen.findByRole('button', { name: 'Large v3' })
    fireEvent.mouseDown(selector)
    fireEvent.click(await screen.findByRole('option', { name: 'Small' }))

    await waitFor(() => expect(requests).toEqual([{ model: 'small' }]))
    expect(screen.getByRole('button', { name: 'Small' })).toBeInTheDocument()
  })

  it('indexes the chosen number, skips existing songs, and refreshes status', async () => {
    const requests = []
    renderPage('/api/ai/rag/index', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({
        json: { indexed: 2, skipped: 1, failed: 0 },
      })
    })

    const limitInput = await screen.findByRole('spinbutton', {
      name: 'Songs to index',
    })
    fireEvent.change(limitInput, { target: { value: '75' } })
    const button = screen.getByRole('button', { name: 'Index 75 songs' })
    await waitFor(() => expect(button).toBeEnabled())
    fireEvent.click(button)

    await waitFor(() => expect(requests).toEqual([{ limit: 75, force: false }]))
    expect(
      await screen.findByText('Indexed 2, skipped 1, failed 0.'),
    ).toBeInTheDocument()
    expect(
      screen.getByText('Already indexed songs are skipped.'),
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(
        mockHttpClient.mock.calls.filter(
          ([url]) => url === '/api/ai/rag/status',
        ),
      ).toHaveLength(2),
    )
  })

  it('optionally indexes playlist RAG documents with songs', async () => {
    const requests = []
    renderPage('/api/ai/rag/index', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({
        json: {
          indexed: 2,
          skipped: 0,
          failed: 0,
          playlists: { indexed: 3, skipped: 1, failed: 0 },
        },
      })
    })

    fireEvent.click(
      await screen.findByRole('checkbox', { name: 'Include playlists' }),
    )
    fireEvent.click(screen.getByRole('button', { name: 'Index 50 songs' }))

    await waitFor(() =>
      expect(requests).toEqual([
        { limit: 50, force: false, includePlaylists: true },
      ]),
    )
    expect(
      await screen.findByText(
        'Indexed 2, skipped 0, failed 0. Playlists: indexed 3, skipped 1, failed 0.',
      ),
    ).toBeInTheDocument()
  })

  it('force refreshes indexed songs to backfill expanded payloads', async () => {
    const requests = []
    renderPage('/api/ai/rag/index', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({
        json: { indexed: 50, skipped: 0, failed: 0 },
      })
    })

    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Refresh 50 indexed songs',
      }),
    )

    await waitFor(() => expect(requests).toEqual([{ limit: 50, force: true }]))
    expect(
      await screen.findByText('Refreshed 50, skipped 0, failed 0.'),
    ).toBeInTheDocument()
  })

  it('opens the Qdrant collection and indexed songs table', async () => {
    const requests = []
    renderPage('/api/ai/rag/documents', (url) => {
      requests.push(url)
      return Promise.resolve({
        json: {
          collection: 'test_songs',
          indexedCount: 42,
          songs: [
            {
              songId: 'song-1',
              title: 'Bright Song',
              artist: 'Artist',
              album: 'Album',
              albumArtist: 'Album Artist',
              trackNumber: 3,
              discNumber: 1,
              year: 2020,
              genre: 'Pop',
              explicit: false,
              explicitStatus: 'clean',
              bpm: 100,
              lufs: -12.5,
              duration: 215,
              playCount: 7,
              lastPlayedAt: '2026-07-04T07:00:00Z',
              hasLyrics: true,
              codec: 'flac',
              bitRate: 900,
            },
          ],
        },
      })
    })

    const status = await screen.findByRole('region', { name: 'RAG status' })
    const viewButton = within(status).getByRole('button', {
      name: 'View indexed songs',
    })
    await waitFor(() => expect(viewButton).toBeEnabled())
    fireEvent.click(viewButton)

    const dialog = await screen.findByRole('dialog', {
      name: 'Indexed RAG songs',
    })
    expect(requests).toEqual(['/api/ai/rag/documents?limit=100'])
    expect(
      within(dialog).getByText(/Collection: test_songs · Showing 1 of 42/),
    ).toBeInTheDocument()
    const table = within(dialog).getByRole('table', {
      name: 'Indexed songs table',
    })
    expect(table).toBeInTheDocument()
    expect(within(table).getByText('Bright Song')).toBeInTheDocument()
    expect(within(table).getByText('03:35')).toBeInTheDocument()
    expect(within(table).getByText('flac · 900 kbps')).toBeInTheDocument()

    fireEvent.click(within(table).getByRole('button', { name: 'View' }))
    expect(await screen.findByText('Indexed song payload')).toBeInTheDocument()
    expect(screen.getByText(/"codec": "flac"/)).toBeInTheDocument()
  })

  it('selects all new songs and skips songs already added', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify([songs[0]]))
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    fireEvent.click(screen.getByRole('button', { name: 'Add songs' }))

    const selectAll = await screen.findByRole('checkbox', {
      name: 'Select all new songs',
    })
    const dialog = selectAll.closest('[role="dialog"]')
    const existingSong = within(dialog).getByRole('checkbox', {
      name: 'Select First song',
    })
    const newSong = within(dialog).getByRole('checkbox', {
      name: 'Select Second song',
    })
    expect(existingSong).toBeDisabled()
    expect(newSong).not.toBeChecked()
    expect(
      within(dialog).getByText('First song (Already added)'),
    ).toBeInTheDocument()

    fireEvent.click(selectAll)
    expect(newSong).toBeChecked()

    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Add selected songs' }),
    )

    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem('aiToolAddedSongs'))
      expect(saved.map((song) => song.id)).toEqual(['song-1', 'song-2'])
    })
  })

  it('searches RAG and renders song results', async () => {
    const requests = []
    renderPage('/api/ai/rag/search', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({
        json: {
          appliedFilters: {
            explicit: 'clean',
            genre: 'Pop',
            bpmMin: 100,
            durationMax: 240,
            hasLyrics: true,
          },
          count: 1,
          results: [
            {
              songId: 'song-1',
              title: 'Bright Song',
              artist: 'Artist',
              genre: 'Pop',
              explicit: false,
              score: 0.87,
            },
          ],
        },
      })
    })

    const input = screen.getByPlaceholderText('Test RAG search')
    fireEvent.change(input, { target: { value: 'clean upbeat songs' } })
    fireEvent.click(screen.getByRole('checkbox', { name: 'Clean only' }))
    fireEvent.change(screen.getByRole('textbox', { name: 'Genre' }), {
      target: { value: 'Pop' },
    })
    fireEvent.change(screen.getByRole('spinbutton', { name: 'BPM min' }), {
      target: { value: '100' },
    })
    fireEvent.change(
      screen.getByRole('spinbutton', { name: 'Max duration (sec)' }),
      { target: { value: '240' } },
    )
    fireEvent.click(screen.getByRole('checkbox', { name: 'Has lyrics' }))
    const button = screen.getByRole('button', { name: 'Search RAG' })
    await waitFor(() => expect(button).toBeEnabled())
    fireEvent.click(button)

    await waitFor(() =>
      expect(requests).toEqual([
        {
          query: 'clean upbeat songs',
          topK: 12,
          filters: {
            explicit: 'clean',
            genre: 'Pop',
            bpmMin: 100,
            durationMax: 240,
            hasLyrics: true,
          },
        },
      ]),
    )
    expect(
      await screen.findByText(
        /Bright Song — Artist · score 0\.870 · Pop · Clean/,
      ),
    ).toBeInTheDocument()
    expect(
      screen.getByText(/Applied filters: .*"explicit":"clean".*1 result/),
    ).toBeInTheDocument()
  })

  it('renders RAG sources under an AI chat answer', async () => {
    const requests = []
    renderPage('/api/ai/chat', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({
        json: {
          response: 'Try Bright Song.',
          provider: 'gemini-2.5',
          sources: [
            {
              songId: 'song-1',
              title: 'Bright Song',
              artist: 'Artist',
              score: 0.91,
            },
          ],
        },
      })
    })

    const ragDialog = await screen.findByRole('dialog', { name: 'RAG' })
    fireEvent.change(
      within(ragDialog).getByPlaceholderText('Ask RAG about your library...'),
      {
        target: { value: 'What should I play?' },
      },
    )
    fireEvent.click(within(ragDialog).getByTitle('Send RAG message'))

    expect(await screen.findByText('Try Bright Song.')).toBeInTheDocument()
    expect(requests[0].useRag).toBe(true)
    expect(requests[0].provider).toBe('gemma-3-4b')
    expect(await screen.findByText('Sources')).toBeInTheDocument()
    expect(
      await screen.findByText(/Bright Song — Artist · 0\.910/),
    ).toBeInTheDocument()
  })

  it('opens a separate normal chat that bypasses RAG', async () => {
    const requests = []
    renderPage('/api/ai/chat', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({
        json: {
          response: 'Normal chat response.',
          provider: 'gemma-3-4b',
        },
      })
    })

    fireEvent.click(screen.getByRole('button', { name: 'Open AI Chat' }))
    const normalDialog = await screen.findByRole('dialog', { name: 'AI Chat' })
    fireEvent.change(
      within(normalDialog).getByPlaceholderText('Ask AI anything...'),
      { target: { value: 'How are you?' } },
    )
    fireEvent.click(within(normalDialog).getByTitle('Send normal message'))

    expect(
      await within(normalDialog).findByText('Normal chat response.'),
    ).toBeInTheDocument()
    expect(requests).toHaveLength(1)
    expect(requests[0].useRag).toBe(false)
    expect(requests[0].provider).toBe('gemma-3-4b')
  })

  it('shows when chat falls back after a RAG failure', async () => {
    renderPage('/api/ai/chat', () =>
      Promise.resolve({
        json: {
          response: 'Hello from Gemma.',
          provider: 'gemma-26b',
          ragError: 'embedding unavailable',
        },
      }),
    )

    const ragDialog = await screen.findByRole('dialog', { name: 'RAG' })
    fireEvent.change(
      within(ragDialog).getByPlaceholderText('Ask RAG about your library...'),
      {
        target: { value: 'Hi' },
      },
    )
    fireEvent.click(within(ragDialog).getByTitle('Send RAG message'))

    expect(await screen.findByText('Hello from Gemma.')).toBeInTheDocument()
    expect(
      await screen.findByText(
        /RAG unavailable: embedding unavailable\. Answered without library context\./,
      ),
    ).toBeInTheDocument()
  })

  it('opens saved lyrics when Available is clicked', async () => {
    localStorage.setItem(
      'aiToolAddedSongs',
      JSON.stringify([{ ...songs[0], lyrics: 'saved' }]),
    )
    renderPage('/api/ai/songs/song-1/lyrics', () =>
      Promise.resolve({ json: { language: 'eng', text: 'Saved lyric line' } }),
    )

    fireEvent.click(await screen.findByRole('button', { name: 'Available' }))

    const lyricLine = await screen.findByText('Saved lyric line')
    expect(lyricLine).toBeInTheDocument()
    expect(
      within(lyricLine.closest('[role="dialog"]')).queryByRole('button', {
        name: 'Delete Lyrics',
      }),
    ).not.toBeInTheDocument()
  })

  it('skips songs that already have lyrics during bulk fetching', async () => {
    localStorage.setItem(
      'aiToolAddedSongs',
      JSON.stringify([{ ...songs[0], lyrics: 'saved' }, songs[1]]),
    )
    const requests = []
    renderPage('/api/ai/songs/', (url, options) => {
      requests.push({ url, method: options.method })
      return Promise.resolve({ json: { language: 'eng', text: 'Fetched' } })
    })

    fireEvent.click(screen.getByRole('button', { name: 'Fetch Lyrics' }))

    await waitFor(() =>
      expect(requests).toEqual([
        { url: '/api/ai/songs/song-2/lyrics/fetch', method: 'POST' },
      ]),
    )
  })

  it('shows how long Whisper runs and the completed lyrics fetch time', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify([songs[0]]))
    let resolveRequest
    const now = vi.spyOn(Date, 'now').mockReturnValue(1000)

    try {
      renderPage(
        '/api/ai/songs/song-1/lyrics/fetch',
        () =>
          new Promise((resolve) => {
            resolveRequest = resolve
          }),
      )

      fireEvent.click(screen.getByRole('button', { name: 'Fetch Lyrics' }))
      expect(
        await screen.findByText(/Whisper running for 0 seconds/),
      ).toBeInTheDocument()

      now.mockReturnValue(6200)
      resolveRequest({ json: { language: 'eng', text: 'Fetched' } })

      expect(
        await screen.findByText(
          /Whisper finished fetching lyrics in 5 seconds/,
        ),
      ).toBeInTheDocument()
    } finally {
      now.mockRestore()
    }
  })

  it('deletes lyrics for selected songs in bulk', async () => {
    localStorage.setItem(
      'aiToolAddedSongs',
      JSON.stringify(songs.map((song) => ({ ...song, lyrics: 'saved' }))),
    )
    const requests = []
    renderPage('/api/ai/songs/', (url, options) => {
      requests.push({ url, method: options.method })
      return Promise.resolve({ json: { deleted: true } })
    })

    fireEvent.click(screen.getByRole('button', { name: 'Delete Lyrics' }))

    await waitFor(() => expect(requests).toHaveLength(2))
    expect(requests).toEqual([
      { url: '/api/ai/songs/song-1/lyrics', method: 'DELETE' },
      { url: '/api/ai/songs/song-2/lyrics', method: 'DELETE' },
    ])
    await waitFor(() => expect(screen.getAllByText('Missing')).toHaveLength(2))
  })

  it('deletes lyrics for an individual song from its row actions', async () => {
    localStorage.setItem(
      'aiToolAddedSongs',
      JSON.stringify([{ ...songs[0], lyrics: 'saved' }]),
    )
    const requests = []
    renderPage('/api/ai/songs/song-1/lyrics', (_url, options) => {
      requests.push(options.method)
      return Promise.resolve({ json: { deleted: true } })
    })

    fireEvent.click(screen.getByRole('button', { name: 'Actions' }))
    fireEvent.click(
      await screen.findByRole('menuitem', { name: 'Delete Lyrics' }),
    )

    await waitFor(() => expect(requests).toEqual(['DELETE']))
    expect(await screen.findByText('Missing')).toBeInTheDocument()
  })

  it('stops an in-progress metadata fetch', async () => {
    const signals = []
    const requests = []
    renderPage('/api/ai/fetch-metadata', (url, options) => {
      requests.push(JSON.parse(options.body))
      return createAbortableRequest(signals)(url, options)
    })

    fireEvent.click(screen.getByRole('button', { name: 'Fetch AI Metadata' }))
    const dialog = await chooseModel('Gemma 26B')
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Fetch Metadata' }),
    )
    const stopButton = await screen.findByRole('button', { name: 'Stop' })
    expect(signals).toHaveLength(1)
    expect(requests[0].provider).toBe('gemma-26b')

    fireEvent.click(stopButton)

    await waitFor(() => expect(signals[0].aborted).toBe(true))
    expect(await screen.findByText(/Stopped/)).toBeInTheDocument()
    expect(signals).toHaveLength(1)
  })

  it('uses the selected model to classify explicit content', async () => {
    const requests = []
    renderPage('/api/ai/classify-explicit', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    fireEvent.click(screen.getByRole('button', { name: 'Classify Explicit' }))
    const dialog = await chooseModel('Gemini 3.5')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Classify' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toMatchObject({
      songIds: ['song-1', 'song-2'],
      provider: 'gemini-3.5',
    })
    expect(requests[0].includedWords).toContain('fuck')
    expect(requests[0].excludedWords).toContain('damn')
  })

  it('shows classification reasons when Clean or Explicit is clicked', async () => {
    localStorage.setItem(
      'aiToolAddedSongs',
      JSON.stringify([
        {
          ...songs[0],
          explicitStatus: 'c',
          explicitReason: 'No qualifying explicit language was found.',
          explicitConfidence: 94,
          explicitEvidence: [],
        },
      ]),
    )
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    fireEvent.click(await screen.findByRole('button', { name: 'Clean' }))

    expect(
      await screen.findByText('No qualifying explicit language was found.'),
    ).toBeInTheDocument()
    expect(screen.getByText('Confidence: 94%')).toBeInTheDocument()
  })

  it('edits explicit and excluded word rules used for classification', async () => {
    const requests = []
    renderPage('/api/ai/classify-explicit', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    fireEvent.click(screen.getByRole('button', { name: 'Explicit word rules' }))
    fireEvent.change(
      await screen.findByRole('textbox', {
        name: 'Words categorised as explicit',
      }),
      { target: { value: 'custom strong, second strong' } },
    )
    fireEvent.change(
      screen.getByRole('textbox', {
        name: 'Words excluded from explicit categorisation',
      }),
      { target: { value: 'custom mild' } },
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save rules' }))

    const classifyButton = await screen.findByRole('button', {
      name: 'Classify Explicit',
    })
    fireEvent.click(classifyButton)
    const dialog = await chooseModel('Gemini 3.5')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Classify' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].includedWords).toEqual([
      'custom strong',
      'second strong',
    ])
    expect(requests[0].excludedWords).toEqual(['custom mild'])
  })

  it('uses Gemma 3:4b for metadata when selected', async () => {
    const requests = []
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    fireEvent.click(screen.getByRole('button', { name: 'Fetch AI Metadata' }))
    const dialog = await chooseModel('Gemma 3:4b')
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Fetch Metadata' }),
    )

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].provider).toBe('gemma-3-4b')
  })

  it('shows confidence for fetched album, year, and genre', async () => {
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      const request = JSON.parse(options.body)
      return Promise.resolve({
        json: {
          songs: [
            {
              id: request.songIds[0],
              album: 'Fetched album',
              year: 2024,
              aiGenre: 'Indie rock',
              albumConfidence: 91,
              yearConfidence: 82,
              genreConfidence: 73,
            },
          ],
        },
      })
    })

    fireEvent.click(screen.getByRole('button', { name: 'Fetch AI Metadata' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Fetch Metadata' }),
    )

    await waitFor(() => {
      expect(screen.getAllByText('91% confidence')).toHaveLength(2)
      expect(screen.getAllByText('82% confidence')).toHaveLength(2)
      expect(screen.getAllByText('73% confidence')).toHaveLength(2)
    })
  })

  it('clears only AI-fetched metadata for selected songs', async () => {
    const queuedSongs = [
      {
        ...songs[0],
        album: 'AI Album',
        year: 2024,
        aiGenre: 'AI Rock',
        metadataConfidence: { album: 91, year: 82, genre: 73 },
        aiFields: { album: true, year: true, aiGenre: true },
      },
      {
        ...songs[1],
        album: 'Original Album',
        year: 1999,
        aiGenre: 'AI Pop',
        metadataConfidence: { genre: 88 },
        aiFields: { aiGenre: true },
      },
    ]
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(queuedSongs))
    const requests = []
    renderPage('/api/ai/clear-metadata', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songIds: ['song-1', 'song-2'] } })
    })

    fireEvent.click(
      screen.getByRole('button', { name: 'Clear Fetched Metadata' }),
    )

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toEqual({
      songs: [
        { id: 'song-1', album: true, year: true },
        { id: 'song-2', album: false, year: false },
      ],
    })
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem('aiToolAddedSongs'))
      expect(saved[0]).toMatchObject({
        album: '[Unknown Album]',
        year: 0,
        aiGenre: '',
        metadataConfidence: {},
      })
      expect(saved[1]).toMatchObject({
        album: 'Original Album',
        year: 1999,
        aiGenre: '',
        metadataConfidence: {},
      })
    })
  })

  it('lets the user choose visible columns from the Columns menu', () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
    const menu = screen.getByRole('menu')
    fireEvent.click(within(menu).getByRole('menuitem', { name: 'Album' }))
    fireEvent.keyDown(menu, { key: 'Escape' })

    expect(
      screen.queryByRole('columnheader', { name: 'Album' }),
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Year' }),
    ).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
    const reopenedMenu = screen.getByRole('menu')
    fireEvent.click(
      within(reopenedMenu).getByRole('menuitem', { name: 'Album' }),
    )
    fireEvent.keyDown(reopenedMenu, { key: 'Escape' })
    expect(
      screen.getByRole('columnheader', { name: 'Album' }),
    ).toBeInTheDocument()
  })

  it('toggles all confidence columns together from the Columns menu', () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
    const menu = screen.getByRole('menu')
    const confidenceToggle = within(menu).getByRole('menuitem', {
      name: 'All Confidence Columns',
    })
    fireEvent.click(confidenceToggle)
    fireEvent.keyDown(menu, { key: 'Escape' })

    expect(
      screen.queryByRole('columnheader', { name: 'Album Confidence' }),
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('columnheader', { name: 'Year Confidence' }),
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('columnheader', { name: 'Genre Confidence' }),
    ).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Columns' }))
    const reopenedMenu = screen.getByRole('menu')
    fireEvent.click(
      within(reopenedMenu).getByRole('menuitem', {
        name: 'All Confidence Columns',
      }),
    )
    fireEvent.keyDown(reopenedMenu, { key: 'Escape' })

    expect(
      screen.getByRole('columnheader', { name: 'Album Confidence' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Year Confidence' }),
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Genre Confidence' }),
    ).toBeInTheDocument()
  })
})
