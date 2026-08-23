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
import { MemoryRouter, Route } from 'react-router-dom'
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
const songsWithLyrics = songs.map((song, index) => ({
  ...song,
  lyrics: `Complete fetched lyrics for song ${index + 1}`,
}))

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
  embeddingBackend: 'ollama',
  embeddingModel: 'embeddinggemma',
  embeddingLocal: true,
  offlineMode: true,
}

const renderPage = (
  actionPath,
  actionRequest,
  ragStatus = defaultRAGStatus,
  aiStatus = { services: [], whisperModel: 'large-v3' },
) => {
  mockHttpClient.mockImplementation((url, options = {}) => {
    if (url === '/api/ai/status') {
      return Promise.resolve({ json: aiStatus })
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

  render(
    <MemoryRouter initialEntries={['/ai-tool']}>
      <AiToolPage />
    </MemoryRouter>,
  )
  fireEvent.click(
    screen.getByRole('checkbox', { name: 'Select all added songs' }),
  )
}

const renderPageWithoutSelection = (
  handlers = {},
  { queuedSongs = null } = {},
) => {
  // The shared test localStorage stub has no removeItem, so an empty queue is
  // expressed as an empty list.
  localStorage.setItem('aiToolAddedSongs', JSON.stringify(queuedSongs || []))
  mockHttpClient.mockImplementation((url, options = {}) => {
    if (url === '/api/ai/status') {
      return Promise.resolve({
        json: { services: [], whisperModel: 'large-v3' },
      })
    }
    if (url === '/api/ai/rag/status') {
      return Promise.resolve({ json: defaultRAGStatus })
    }
    if (url.startsWith('/api/song?')) {
      return Promise.resolve({ json: queuedSongs || [] })
    }
    const handler = Object.entries(handlers).find(([path]) =>
      url.startsWith(path),
    )
    if (handler) return handler[1](url, options)
    return Promise.resolve({ json: {} })
  })

  render(
    <MemoryRouter initialEntries={['/ai-tool']}>
      <AiToolPage />
    </MemoryRouter>,
  )
}

const openRAGControls = async () => {
  const toggle = await screen.findByRole('button', {
    name: /RAG controls/,
  })
  if (toggle.getAttribute('aria-expanded') === 'false') fireEvent.click(toggle)
  return screen.findByRole('region', { name: 'RAG controls' })
}

const openExplicitActions = () => {
  fireEvent.click(screen.getByRole('button', { name: 'Explicit' }))
  return screen.getByRole('menu', { name: 'Explicit actions' })
}

const openMetadataActions = () => {
  fireEvent.click(screen.getByRole('button', { name: 'Metadata' }))
  return screen.getByRole('menu', { name: 'Metadata actions' })
}

const clickExplicitAction = (name) => {
  fireEvent.click(within(openExplicitActions()).getByRole('menuitem', { name }))
}

const clickMetadataAction = (name) => {
  fireEvent.click(within(openMetadataActions()).getByRole('menuitem', { name }))
}

const openColumnMenu = () => {
  const metadataMenu = openMetadataActions()
  fireEvent.click(
    within(metadataMenu).getByRole('menuitem', {
      name: 'Columns to display',
    }),
  )
  return screen.getByRole('menu')
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

  it('navigates to the dashboard and playlist tools from the tabs', async () => {
    const location = { pathname: '/ai-tool' }
    render(
      <MemoryRouter initialEntries={['/ai-tool']}>
        <AiToolPage />
        <Route
          path="*"
          render={({ location: current }) => {
            location.pathname = current.pathname
            return null
          }}
        />
      </MemoryRouter>,
    )

    fireEvent.click(await screen.findByRole('button', { name: 'AI Dashboard' }))
    expect(location.pathname).toBe('/ai-dashboard')

    fireEvent.click(screen.getByRole('button', { name: 'Playlist AI Tool' }))
    expect(location.pathname).toBe('/playlist-ai-tool')
  })

  it('stops an in-progress lyrics fetch', async () => {
    const requests = []
    renderPage('/api/ai/lyrics/fetch-job', (url, options = {}) => {
      if (url.endsWith('/status'))
        return Promise.resolve({ json: { status: 'idle' } })
      requests.push({ url, method: options.method })
      if (options.method === 'DELETE') {
        return Promise.resolve({
          json: { status: 'stopped', running: false, done: 0, total: 2 },
        })
      }
      return Promise.resolve({
        json: { status: 'running', running: true, done: 0, total: 2 },
      })
    })

    clickExplicitAction('Fetch Lyrics')
    await screen.findByRole('button', { name: 'Stop' })

    fireEvent.click(screen.getByRole('button', { name: 'Stop' }))

    await waitFor(() =>
      expect(requests).toEqual([
        { url: '/api/ai/lyrics/fetch-job', method: 'POST' },
        { url: '/api/ai/lyrics/fetch-job', method: 'DELETE' },
      ]),
    )
    expect(await screen.findByText(/Stopped/)).toBeInTheDocument()
  })

  it('shows the read-only RAG configuration status', async () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    expect(await screen.findByLabelText('RAG status: Enabled')).toBeVisible()
    expect(screen.getByLabelText('Vector DB status: Online')).toBeVisible()

    const controls = await openRAGControls()
    expect(
      within(controls).getByText('Vector URL: http://vector.test:6333'),
    ).toBeInTheDocument()
    expect(
      within(controls).getByText('Collection: test_songs'),
    ).toBeInTheDocument()
    expect(within(controls).getByText('Exists')).toBeInTheDocument()
    expect(within(controls).getByText('Indexed: 42')).toBeInTheDocument()
    expect(within(controls).getByText('Top K: 12')).toBeInTheDocument()
    expect(
      within(controls).getByText(
        'Embeddings: Offline protected · ollama (embeddinggemma)',
      ),
    ).toBeInTheDocument()
    expect(mockHttpClient).toHaveBeenCalledWith('/api/ai/rag/status')
  })

  it('keeps RAG controls collapsed by default without hiding model health', async () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    const toggle = await screen.findByRole('button', {
      name: /RAG controls/,
    })
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByRole('region', { name: 'RAG controls' })).toBeNull()
    expect(screen.getByLabelText('RAG status: Enabled')).toBeVisible()
    expect(screen.getByLabelText('Vector DB status: Online')).toBeVisible()
    expect(screen.getByText('Gemma 26B')).toBeVisible()
    expect(screen.getByText('Whisper')).toBeVisible()

    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(
      await screen.findByRole('region', { name: 'RAG controls' }),
    ).toBeVisible()
  })

  it('shows Whisper as busy instead of offline while it is fetching', async () => {
    renderPage(
      '/api/unused',
      () => Promise.resolve({ json: {} }),
      defaultRAGStatus,
      {
        services: [
          { id: 'whisper', label: 'Whisper', online: true, state: 'busy' },
        ],
        whisperModel: 'large-v3',
      },
    )

    const whisper = await screen.findByText('Whisper')
    expect(within(whisper.parentElement).getByText('Busy')).toBeInTheDocument()
  })

  it('shows Qdrant offline and its error message', async () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }), {
      ...defaultRAGStatus,
      vectorDbOnline: false,
      collectionExists: false,
      indexedCount: 0,
      error: 'Qdrant unavailable: connection refused',
    })

    expect(await screen.findByLabelText('RAG status: Enabled')).toBeVisible()
    expect(screen.getByLabelText('Vector DB status: Offline')).toBeVisible()
    const controls = await openRAGControls()
    expect(within(controls).getByText('Missing')).toBeInTheDocument()
    expect(within(controls).getByText('Indexed: 0')).toBeInTheDocument()
    expect(
      within(controls).getByText('Qdrant unavailable: connection refused'),
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

    expect(await screen.findByLabelText('RAG status: Disabled')).toBeVisible()
    expect(screen.getByLabelText('Vector DB status: Offline')).toBeVisible()
    const controls = await openRAGControls()
    expect(within(controls).getByText('Missing')).toBeInTheDocument()
    expect(
      within(controls).getByRole('spinbutton', { name: 'Songs to index' }),
    ).toHaveValue(50)
    expect(
      within(controls).getByRole('button', { name: 'Index 50 songs' }),
    ).toBeDisabled()
  })

  it('toggles RAG off at runtime', async () => {
    const requests = []
    renderPage('/api/ai/rag/enabled', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { enabled: false } })
    })

    const controls = await openRAGControls()
    fireEvent.click(
      within(controls).getByRole('button', { name: 'Disable RAG' }),
    )

    await waitFor(() => expect(requests).toEqual([{ enabled: false }]))
    expect(
      within(controls).getByRole('button', { name: 'Enable RAG' }),
    ).toBeInTheDocument()
    expect(screen.getByLabelText('RAG status: Disabled')).toBeVisible()
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

    await openRAGControls()
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
      screen.getByText(
        'Index skips unchanged songs. Refresh updates the latest information for songs already stored in Qdrant.',
      ),
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

    await openRAGControls()
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

    await openRAGControls()
    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Refresh 50 indexed songs',
      }),
    )

    await waitFor(() => expect(requests).toEqual([{ limit: 50, force: true }]))
    expect(
      await screen.findByText('Refreshed 50, skipped 0, failed 0.'),
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        'Index skips unchanged songs. Refresh updates the latest information for songs already stored in Qdrant.',
      ),
    ).toBeInTheDocument()
  })

  it('adds fetched song lyrics to Qdrant', async () => {
    const requests = []
    renderPage('/api/ai/rag/lyrics', (_url, options) => {
      requests.push({ method: options.method, body: JSON.parse(options.body) })
      return Promise.resolve({
        json: { indexed: 8, skipped: 2, failed: 0 },
      })
    })

    const controls = await openRAGControls()
    const addButton = within(controls).getByRole('button', {
      name: 'Add Qdrant lyrics',
    })
    await waitFor(() => expect(addButton).toBeEnabled())
    fireEvent.click(addButton)

    await waitFor(() =>
      expect(requests).toEqual([{ method: 'POST', body: { limit: 500 } }]),
    )
    expect(
      await screen.findByText(
        'Added or updated 8 Qdrant lyrics, skipped 2, failed 0.',
      ),
    ).toBeInTheDocument()
  })

  it('views and searches words stored inside Qdrant lyrics', async () => {
    const requests = []
    renderPage('/api/ai/rag/lyrics', (url) => {
      requests.push(url)
      const isSearch = url.includes('query=vikash')
      return Promise.resolve({
        json: {
          collection: 'test_songs',
          count: 1,
          query: isSearch ? 'vikash' : '',
          songs: [
            {
              songId: 'song-v',
              title: isSearch ? 'Vikash Song' : 'Stored Song',
              artist: 'Artist',
              album: 'Album',
              year: 2026,
              genre: 'Pop',
              hasLyrics: true,
              lyricsText: isSearch
                ? 'hello vikash welcome home'
                : 'already stored lyrics',
            },
          ],
        },
      })
    })

    const controls = await openRAGControls()
    const viewButton = within(controls).getByRole('button', {
      name: 'View Qdrant lyrics',
    })
    await waitFor(() => expect(viewButton).toBeEnabled())
    fireEvent.click(viewButton)

    const dialog = await screen.findByRole('dialog', {
      name: 'Qdrant lyrics',
    })
    expect(requests).toEqual(['/api/ai/rag/lyrics?limit=500'])
    expect(
      within(dialog).getByRole('table', { name: 'Qdrant lyrics table' }),
    ).toBeInTheDocument()
    expect(
      within(dialog).getByText('already stored lyrics'),
    ).toBeInTheDocument()

    fireEvent.change(
      within(dialog).getByRole('textbox', {
        name: 'Search words in Qdrant lyrics',
      }),
      { target: { value: 'vikash' } },
    )
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Search lyrics' }),
    )

    await waitFor(() =>
      expect(requests).toEqual([
        '/api/ai/rag/lyrics?limit=500',
        '/api/ai/rag/lyrics?limit=500&query=vikash',
      ]),
    )
    expect(await within(dialog).findByText('Vikash Song')).toBeInTheDocument()
    expect(
      within(dialog).getByText(/hello vikash welcome home/),
    ).toBeInTheDocument()

    fireEvent.click(within(dialog).getByRole('button', { name: 'View lyrics' }))
    const details = await screen.findByRole('dialog', { name: 'Vikash Song' })
    expect(
      within(details).getByText('hello vikash welcome home'),
    ).toBeInTheDocument()
  })

  it('clears the indexed RAG collection only after confirmation', async () => {
    const requests = []
    renderPage('/api/ai/rag/index', (_url, options) => {
      requests.push(options.method)
      return Promise.resolve({
        json: { collection: 'test_songs', cleared: true },
      })
    })

    const controls = await openRAGControls()
    const button = within(controls).getByRole('button', {
      name: 'Clear indexed songs',
    })
    await waitFor(() => expect(button).toBeEnabled())
    fireEvent.click(button)

    // Nothing is deleted until the dialog is accepted.
    expect(requests).toEqual([])
    const dialog = await screen.findByRole('dialog', {
      name: 'Clear indexed songs?',
    })
    expect(
      within(dialog).getByText(/your Navidrome songs and files stay untouched/),
    ).toBeInTheDocument()

    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))
    expect(requests).toEqual([])

    fireEvent.click(button)
    fireEvent.click(
      within(
        await screen.findByRole('dialog', { name: 'Clear indexed songs?' }),
      ).getByRole('button', { name: 'Clear index' }),
    )

    await waitFor(() => expect(requests).toEqual(['DELETE']))
    expect(
      await screen.findByText(
        'Cleared indexed songs from test_songs. Reindex when ready.',
      ),
    ).toBeInTheDocument()
    await waitFor(() =>
      expect(
        mockHttpClient.mock.calls.filter(
          ([url]) => url === '/api/ai/rag/status',
        ),
      ).toHaveLength(2),
    )
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

    const controls = await openRAGControls()
    const viewButton = within(controls).getByRole('button', {
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

    await openRAGControls()
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
              lyricSnippet: 'We are the ⟦champions⟧',
            },
          ],
        },
      })
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Open RAG' }))
    const ragDialog = await screen.findByRole('dialog', { name: 'RAG' })
    fireEvent.change(
      within(ragDialog).getByLabelText('Ask RAG about your library'),
      {
        target: { value: 'What should I play?' },
      },
    )
    fireEvent.click(within(ragDialog).getByTitle('Send RAG message'))

    expect(await screen.findByText('Try Bright Song.')).toBeInTheDocument()
    expect(requests[0].useRag).toBe(true)
    expect(requests[0].provider).toBe('deepseek-v3.2')
    expect(await screen.findByText('Sources')).toBeInTheDocument()
    expect(
      await screen.findByText(/Bright Song — Artist · 0\.910/),
    ).toBeInTheDocument()
    expect(screen.getByText('We are the ⟦champions⟧')).toBeInTheDocument()
  })

  it('labels exact lyrics responses as direct Qdrant results', async () => {
    renderPage('/api/ai/chat', () =>
      Promise.resolve({
        json: {
          response:
            'Here is 1 indexed song containing "thank you" in the lyrics:\n\n1. Gratitude — Singer\n   …say ⟦thank you⟧ my friend…',
          provider: 'gemma-3-4b',
          direct: true,
          sources: [
            {
              songId: 'song-1',
              title: 'Gratitude',
              artist: 'Singer',
              lyricSnippet: '…say ⟦thank you⟧ my friend…',
            },
          ],
        },
      }),
    )

    fireEvent.click(await screen.findByRole('button', { name: 'Open RAG' }))
    const ragDialog = await screen.findByRole('dialog', { name: 'RAG' })
    fireEvent.change(
      within(ragDialog).getByLabelText('Ask RAG about your library'),
      { target: { value: 'songs with the words thank you in it' } },
    )
    fireEvent.click(within(ragDialog).getByTitle('Send RAG message'))

    expect(
      await within(ragDialog).findByText('Qdrant exact search'),
    ).toBeInTheDocument()
    expect(
      await within(ragDialog).findByText(/Here is 1 indexed song containing/),
    ).toBeInTheDocument()
    expect(within(ragDialog).queryByText('Sources')).not.toBeInTheDocument()
  })

  it('shows an opt-in developer trace with exact prompts and raw responses', async () => {
    const requests = []
    renderPage('/api/ai/chat', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({
        json: {
          response: 'Try Bright Song.',
          provider: 'gemma-3-4b',
          model: 'gemma3:4b',
          trace: {
            requestId: 'trace-123',
            provider: 'gemma-3-4b',
            model: 'gemma3:4b',
            useRag: true,
            durationMs: 42,
            stages: [
              {
                id: 'retrieval',
                label: 'Searched the indexed music library',
                status: 'completed',
                durationMs: 12,
                detail: 'Qdrant vector similarity search',
                input: { query: 'bright songs', topK: 12 },
                output: { count: 1 },
              },
              {
                id: 'prompt',
                label: 'Built the final answer prompt',
                status: 'completed',
                prompt: 'EXACT FINAL PROMPT WITH RETRIEVED CONTEXT',
              },
              {
                id: 'answer',
                label: 'Called AI for the final answer',
                status: 'completed',
                durationMs: 30,
                prompt: 'EXACT FINAL PROMPT WITH RETRIEVED CONTEXT',
                response: 'RAW PROVIDER RESPONSE',
              },
            ],
          },
        },
      })
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Open RAG' }))
    const ragDialog = await screen.findByRole('dialog', { name: 'RAG' })
    const traceSwitch = within(ragDialog).getByRole('checkbox', {
      name: 'Developer Trace',
    })
    expect(traceSwitch).not.toBeChecked()
    fireEvent.click(traceSwitch)
    expect(localStorage.getItem('aiToolChatDeveloperTrace')).toBe('true')

    fireEvent.change(
      within(ragDialog).getByLabelText('Ask RAG about your library'),
      { target: { value: 'Show bright songs' } },
    )
    fireEvent.click(within(ragDialog).getByTitle('Send RAG message'))

    expect(
      await within(ragDialog).findByText('Try Bright Song.'),
    ).toBeInTheDocument()
    expect(requests[0].developerTrace).toBe(true)
    fireEvent.click(
      within(ragDialog).getByRole('button', { name: /Behind the scenes/i }),
    )
    expect(
      await within(ragDialog).findByText(/Request trace-123/),
    ).toBeInTheDocument()

    fireEvent.click(
      within(ragDialog).getByRole('button', {
        name: /Built the final answer prompt/i,
      }),
    )
    expect(
      await within(ragDialog).findByText(
        'EXACT FINAL PROMPT WITH RETRIEVED CONTEXT',
      ),
    ).toBeInTheDocument()

    fireEvent.click(
      within(ragDialog).getByRole('button', {
        name: /Called AI for the final answer/i,
      }),
    )
    expect(
      await within(ragDialog).findByText('RAW PROVIDER RESPONSE'),
    ).toBeInTheDocument()
  })

  it('sends recent RAG conversation history with a follow-up', async () => {
    const requests = []
    renderPage('/api/ai/chat', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({
        json: {
          response:
            requests.length === 1
              ? 'Try these upbeat rock songs.'
              : 'Here are the clean ones.',
          provider: 'gemma-3-4b',
        },
      })
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Open RAG' }))
    const ragDialog = await screen.findByRole('dialog', { name: 'RAG' })
    const input = within(ragDialog).getByLabelText('Ask RAG about your library')

    fireEvent.change(input, { target: { value: 'Show upbeat rock songs' } })
    fireEvent.click(within(ragDialog).getByTitle('Send RAG message'))
    await within(ragDialog).findByText('Try these upbeat rock songs.')

    fireEvent.change(input, { target: { value: 'Only the clean ones' } })
    fireEvent.click(within(ragDialog).getByTitle('Send RAG message'))
    await within(ragDialog).findByText('Here are the clean ones.')

    expect(requests[0].history).toEqual([])
    expect(requests[1].history).toEqual([
      { role: 'user', content: 'Show upbeat rock songs' },
      { role: 'assistant', content: 'Try these upbeat rock songs.' },
    ])
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
    fireEvent.change(within(normalDialog).getByLabelText('Ask AI anything'), {
      target: { value: 'How are you?' },
    })
    fireEvent.click(within(normalDialog).getByTitle('Send normal message'))

    expect(
      await within(normalDialog).findByText('Normal chat response.'),
    ).toBeInTheDocument()
    expect(requests).toHaveLength(1)
    expect(requests[0].useRag).toBe(false)
    expect(requests[0].provider).toBe('deepseek-v3.2')
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

    fireEvent.click(await screen.findByRole('button', { name: 'Open RAG' }))
    const ragDialog = await screen.findByRole('dialog', { name: 'RAG' })
    fireEvent.change(
      within(ragDialog).getByLabelText('Ask RAG about your library'),
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
    renderPage('/api/ai/lyrics/fetch-job', (url, options = {}) => {
      if (url.endsWith('/status'))
        return Promise.resolve({ json: { status: 'idle' } })
      requests.push({
        url,
        method: options.method,
        body: JSON.parse(options.body),
      })
      return Promise.resolve({
        json: { status: 'running', running: true, done: 0, total: 1 },
      })
    })

    clickExplicitAction('Fetch Lyrics')

    await waitFor(() =>
      expect(requests).toEqual([
        {
          url: '/api/ai/lyrics/fetch-job',
          method: 'POST',
          body: { songIds: ['song-2'] },
        },
      ]),
    )
  })

  it('toggles automatic fetching for all songs on the AI page', async () => {
    const requests = []
    renderPage('/api/ai/lyrics/fetch-job', (url, options = {}) => {
      if (url.endsWith('/status'))
        return Promise.resolve({ json: { status: 'idle' } })
      if (options.method === 'DELETE') {
        requests.push({ url, method: options.method })
        return Promise.resolve({
          json: { status: 'stopped', running: false, done: 0, total: 2 },
        })
      }
      requests.push({
        url,
        method: options.method,
        body: JSON.parse(options.body),
      })
      return Promise.resolve({
        json: { status: 'running', running: true, done: 0, total: 2 },
      })
    })

    const setIntervalSpy = vi.spyOn(window, 'setInterval')
    const clearIntervalSpy = vi.spyOn(window, 'clearInterval')
    try {
      const explicitMenu = openExplicitActions()
      const switchControl = within(explicitMenu).getByRole('checkbox', {
        name: 'Fetch All Song Lyrics',
      })
      expect(switchControl).not.toBeChecked()
      fireEvent.click(switchControl)

      await waitFor(() =>
        expect(requests).toEqual([
          {
            url: '/api/ai/lyrics/fetch-job',
            method: 'POST',
            body: { songIds: ['song-1', 'song-2'] },
          },
        ]),
      )
      expect(setIntervalSpy).toHaveBeenCalledWith(
        expect.any(Function),
        10 * 60 * 1000,
      )
      expect(switchControl).toBeChecked()
      expect(
        await within(explicitMenu).findByRole('menuitem', {
          name: 'Fetching Lyrics...',
        }),
      ).toHaveAttribute('aria-disabled', 'true')
      expect(await screen.findByText(/0\/2 done, 2 left/)).toBeInTheDocument()
      await waitFor(() =>
        expect(localStorage.getItem('aiToolAutoFetchAllLyrics')).toBe('true'),
      )

      fireEvent.click(switchControl)

      expect(switchControl).not.toBeChecked()
      expect(screen.queryByText(/0\/2 done, 2 left/)).not.toBeInTheDocument()
      await waitFor(() =>
        expect(requests).toEqual([
          {
            url: '/api/ai/lyrics/fetch-job',
            method: 'POST',
            body: { songIds: ['song-1', 'song-2'] },
          },
          { url: '/api/ai/lyrics/fetch-job', method: 'DELETE' },
        ]),
      )
      expect(
        await within(explicitMenu).findByRole('menuitem', {
          name: 'Fetch Lyrics',
        }),
      ).toBeInTheDocument()
      await waitFor(() =>
        expect(localStorage.getItem('aiToolAutoFetchAllLyrics')).toBe('false'),
      )
      expect(clearIntervalSpy).toHaveBeenCalled()
    } finally {
      setIntervalSpy.mockRestore()
      clearIntervalSpy.mockRestore()
    }
  })

  it('keeps automatic lyric fetching enabled after a page refresh', async () => {
    localStorage.setItem('aiToolAutoFetchAllLyrics', 'true')
    const requests = []
    renderPage('/api/ai/lyrics/fetch-job', (url, options = {}) => {
      if (url.endsWith('/status'))
        return Promise.resolve({ json: { status: 'idle' } })
      requests.push({
        url,
        method: options.method,
        body: JSON.parse(options.body),
      })
      return Promise.resolve({
        json: { status: 'running', running: true, done: 0, total: 2 },
      })
    })

    const switchControl = within(openExplicitActions()).getByRole('checkbox', {
      name: 'Fetch All Song Lyrics',
    })
    expect(switchControl).toBeChecked()
    await waitFor(() =>
      expect(requests).toEqual([
        {
          url: '/api/ai/lyrics/fetch-job',
          method: 'POST',
          body: { songIds: ['song-1', 'song-2'] },
        },
      ]),
    )
  })

  it('continuously fetches metadata with the selected default provider', async () => {
    const requests = []
    renderPage('/api/ai/fetch-metadata', (_url, options = {}) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    const metadataMenu = openMetadataActions()
    const providerSelector = within(metadataMenu).getByRole('button', {
      name: 'Default Metadata AI Provider',
    })
    fireEvent.mouseDown(providerSelector)
    fireEvent.click(
      await screen.findByRole('option', { name: 'DeepSeek V3.2' }),
    )

    await waitFor(() =>
      expect(localStorage.getItem('aiToolMetadataAIProvider')).toBe(
        'deepseek-v3.2',
      ),
    )

    const setIntervalSpy = vi.spyOn(window, 'setInterval')
    const clearIntervalSpy = vi.spyOn(window, 'clearInterval')
    try {
      const switchControl = within(metadataMenu).getByRole('checkbox', {
        name: 'Fetch All Song Metadata',
      })
      expect(switchControl).not.toBeChecked()
      fireEvent.click(switchControl)

      await waitFor(() =>
        expect(requests).toEqual([
          { songIds: ['song-1'], provider: 'deepseek-v3.2' },
          { songIds: ['song-2'], provider: 'deepseek-v3.2' },
        ]),
      )
      expect(setIntervalSpy).toHaveBeenCalledWith(
        expect.any(Function),
        10 * 60 * 1000,
      )
      expect(switchControl).toBeChecked()
      expect(await screen.findByText(/2\/2 done, 0 left/)).toBeInTheDocument()
      await waitFor(() =>
        expect(localStorage.getItem('aiToolAutoFetchAllMetadata')).toBe('true'),
      )

      fireEvent.click(switchControl)

      expect(switchControl).not.toBeChecked()
      expect(screen.queryByText(/2\/2 done, 0 left/)).not.toBeInTheDocument()
      await waitFor(() =>
        expect(localStorage.getItem('aiToolAutoFetchAllMetadata')).toBe(
          'false',
        ),
      )
      expect(clearIntervalSpy).toHaveBeenCalled()
    } finally {
      setIntervalSpy.mockRestore()
      clearIntervalSpy.mockRestore()
    }
  })

  it('fetches metadata with the saved provider without prompting again', async () => {
    localStorage.setItem('aiToolMetadataAIProvider', 'gemini-2.5')
    const requests = []
    renderPage('/api/ai/fetch-metadata', (_url, options = {}) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    clickMetadataAction('Fetch AI Metadata')

    await waitFor(() =>
      expect(requests).toEqual([
        { songIds: ['song-1'], provider: 'gemini-2.5' },
        { songIds: ['song-2'], provider: 'gemini-2.5' },
      ]),
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('stops automatic metadata fetching when disabled and labels its spinner', async () => {
    const signals = []
    renderPage('/api/ai/fetch-metadata', createAbortableRequest(signals))

    const metadataMenu = openMetadataActions()
    const switchControl = within(metadataMenu).getByRole('checkbox', {
      name: 'Fetch All Song Metadata',
    })
    fireEvent.click(switchControl)

    expect(
      await within(metadataMenu).findByRole('menuitem', {
        name: 'Fetching Metadata...',
      }),
    ).toHaveAttribute('aria-disabled', 'true')
    expect(signals).toHaveLength(1)

    fireEvent.click(switchControl)

    await waitFor(() => expect(signals[0].aborted).toBe(true))
    expect(
      await within(metadataMenu).findByRole('menuitem', {
        name: 'Fetch AI Metadata',
      }),
    ).toBeInTheDocument()
  })

  it('reconnects to a running lyrics job and highlights its current song', async () => {
    renderPage('/api/ai/lyrics/fetch-job/status', () =>
      Promise.resolve({
        json: {
          status: 'running',
          running: true,
          currentSongId: 'song-2',
          currentTitle: 'Second song',
          done: 7,
          total: 11,
          startedAt: new Date(Date.now() - 7000).toISOString(),
        },
      }),
    )

    expect(
      await screen.findByText(/Fetching lyrics: Second song/),
    ).toBeInTheDocument()
    expect(await screen.findByText(/7\/11 done, 4 left/)).toBeInTheDocument()
    expect(screen.getByText('Second song').closest('tr')).toHaveAttribute(
      'data-lyrics-fetching',
      'true',
    )
    expect(screen.getByText('First song').closest('tr')).not.toHaveAttribute(
      'data-lyrics-fetching',
    )
  })

  it('classifies only complete fetched lyrics while another song is fetching', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(songsWithLyrics))
    renderPage('/api/ai/lyrics/fetch-job/status', () =>
      Promise.resolve({
        json: {
          status: 'running',
          running: true,
          songIds: ['song-1'],
          currentTitle: 'First song',
          done: 0,
          total: 1,
        },
      }),
    )

    expect(
      await screen.findByText(/Fetching lyrics: First song/),
    ).toBeInTheDocument()
    const whisper = screen.getByText('Whisper')
    expect(within(whisper.parentElement).getByText('Busy')).toBeInTheDocument()
    const metadataMenu = openMetadataActions()
    expect(
      within(metadataMenu).getByRole('menuitem', {
        name: 'Fetch AI Metadata',
      }),
    ).not.toHaveAttribute('aria-disabled', 'true')
    fireEvent.click(
      within(metadataMenu).getByRole('button', {
        name: 'Close Metadata actions',
      }),
    )

    const explicitMenu = openExplicitActions()
    const classifyButton = within(explicitMenu).getByRole('menuitem', {
      name: 'Classify Explicit',
    })
    expect(classifyButton).not.toHaveAttribute('aria-disabled', 'true')
    fireEvent.click(classifyButton)

    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByText(
        'DeepSeek V3.2 will analyze the saved lyrics for 1 song.',
      ),
    ).toBeInTheDocument()
  })

  it('runs and stops metadata independently while Whisper keeps fetching', async () => {
    const metadataSignals = []
    renderPage('/api/ai/', (url, options = {}) => {
      if (url === '/api/ai/lyrics/fetch-job/status') {
        return Promise.resolve({
          json: {
            status: 'running',
            running: true,
            songIds: ['song-1'],
            currentTitle: 'First song',
            done: 0,
            total: 1,
          },
        })
      }
      if (url === '/api/ai/fetch-metadata') {
        return createAbortableRequest(metadataSignals)(url, options)
      }
      return Promise.resolve({ json: {} })
    })

    expect(
      await screen.findByText(/Fetching lyrics: First song/),
    ).toBeInTheDocument()
    clickMetadataAction('Fetch AI Metadata')

    expect(
      await screen.findByText(/Fetching AI metadata: First song/),
    ).toBeInTheDocument()
    const stopButtons = await screen.findAllByRole('button', { name: 'Stop' })
    expect(stopButtons).toHaveLength(2)
    fireEvent.click(stopButtons[1])

    await waitFor(() => expect(metadataSignals[0].aborted).toBe(true))
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: 'Stop' })).toHaveLength(1),
    )
    expect(screen.getByText(/Whisper running/)).toBeInTheDocument()
  })

  it('shows how long Whisper runs and the completed lyrics fetch time', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify([songs[0]]))
    const now = vi.spyOn(Date, 'now').mockReturnValue(1000)

    try {
      renderPage('/api/ai/lyrics/fetch-job', (url, options = {}) => {
        if (url.endsWith('/status'))
          return Promise.resolve({ json: { status: 'idle' } })
        expect(options.method).toBe('POST')
        return Promise.resolve({
          json: {
            status: 'complete',
            running: false,
            currentTitle: 'Complete',
            done: 1,
            total: 1,
            startedAt: new Date(1000).toISOString(),
            finishedAt: new Date(6200).toISOString(),
            results: [{ songId: 'song-1', success: true }],
          },
        })
      })

      clickExplicitAction('Fetch Lyrics')

      expect(
        await screen.findByText(
          /Whisper finished fetching lyrics in 5 seconds/,
        ),
      ).toBeInTheDocument()
    } finally {
      now.mockRestore()
    }
  })

  it('keeps a failed whole-song fetch unavailable', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify([songs[0]]))
    renderPage('/api/ai/lyrics/fetch-job', (url) => {
      if (url.endsWith('/status'))
        return Promise.resolve({ json: { status: 'idle' } })
      return Promise.resolve({
        json: {
          status: 'failed',
          running: false,
          currentTitle: 'Failed',
          done: 1,
          total: 1,
          error: 'Whisper did not finish the complete song',
          results: [
            {
              songId: 'song-1',
              success: false,
              status: 'failed',
              error: 'Whisper did not finish the complete song',
            },
          ],
        },
      })
    })

    clickExplicitAction('Fetch Lyrics')

    expect(
      await screen.findByText('Whisper did not finish the complete song'),
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Available' }),
    ).not.toBeInTheDocument()
    expect(
      JSON.parse(localStorage.getItem('aiToolAddedSongs'))[0].lyrics || '',
    ).toBe('')
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

    clickExplicitAction('Delete Lyrics')

    const dialog = await screen.findByRole('dialog', {
      name: 'Delete saved lyrics?',
    })
    expect(requests).toEqual([])
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Delete 2 lyrics' }),
    )

    await waitFor(() => expect(requests).toHaveLength(2))
    expect(requests).toEqual([
      { url: '/api/ai/songs/song-1/lyrics', method: 'DELETE' },
      { url: '/api/ai/songs/song-2/lyrics', method: 'DELETE' },
    ])
    await waitFor(() =>
      expect(screen.getAllByText('Not fetched')).toHaveLength(2),
    )
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

    const dialog = await screen.findByRole('dialog', {
      name: 'Delete saved lyrics?',
    })
    expect(within(dialog).getByText(/“First song”/)).toBeInTheDocument()
    expect(requests).toEqual([])
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Delete lyrics' }),
    )

    await waitFor(() => expect(requests).toEqual(['DELETE']))
    expect(await screen.findByText('Not fetched')).toBeInTheDocument()
  })

  it('fetches row metadata with the saved provider without prompting again', async () => {
    localStorage.setItem('aiToolMetadataAIProvider', 'gemini-3.5')
    const requests = []
    renderPage('/api/ai/fetch-metadata', (_url, options = {}) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    fireEvent.click(screen.getAllByRole('button', { name: 'Actions' })[0])
    fireEvent.click(
      await screen.findByRole('menuitem', { name: 'Fetch AI Metadata' }),
    )

    await waitFor(() =>
      expect(requests).toEqual([
        { songIds: ['song-1'], provider: 'gemini-3.5' },
      ]),
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('stops an in-progress metadata fetch', async () => {
    localStorage.setItem('aiToolMetadataAIProvider', 'gemma-26b')
    const signals = []
    const requests = []
    renderPage('/api/ai/fetch-metadata', (url, options) => {
      requests.push(JSON.parse(options.body))
      return createAbortableRequest(signals)(url, options)
    })

    clickMetadataAction('Fetch AI Metadata')
    const stopButton = await screen.findByRole('button', { name: 'Stop' })
    expect(signals).toHaveLength(1)
    expect(requests[0].provider).toBe('gemma-26b')

    fireEvent.click(stopButton)

    await waitFor(() => expect(signals[0].aborted).toBe(true))
    expect(await screen.findByText(/Stopped/)).toBeInTheDocument()
    expect(signals).toHaveLength(1)
  })

  it('always uses DeepSeek to classify complete fetched lyrics', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(songsWithLyrics))
    localStorage.setItem('aiToolDefaultProviderV3', 'gemini-3.5')
    const requests = []
    renderPage('/api/ai/classify-explicit', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    clickExplicitAction('Classify Explicit')
    const dialog = await screen.findByRole('dialog')
    expect(
      within(dialog).getByText(
        'DeepSeek V3.2 will analyze the saved lyrics for 2 songs.',
      ),
    ).toBeInTheDocument()
    expect(
      within(dialog).queryByRole('button', { name: 'Gemini 3.5' }),
    ).not.toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Classify' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toMatchObject({
      songIds: ['song-1', 'song-2'],
      provider: 'deepseek-v3.2',
    })
    expect(requests[0].includedWords).toContain('fuck')
    expect(requests[0].includedWords).toContain('fucked')
    expect(requests[0].includedWords).toContain('cocksucker')
    expect(requests[0].excludedWords).toContain('damn')
  })

  it('does not submit songs whose fetched lyrics are missing', () => {
    renderPage('/api/ai/classify-explicit', () =>
      Promise.resolve({ json: { songs: [] } }),
    )

    expect(
      within(openExplicitActions()).getByRole('menuitem', {
        name: 'Classify Explicit',
      }),
    ).toHaveAttribute('aria-disabled', 'true')
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
          explicitEvidence: ['We dance together all night'],
          explicitProvider: 'deepseek-v3.2',
          explicitBasis: 'saved lyrics',
        },
      ]),
    )
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    fireEvent.click(await screen.findByRole('button', { name: 'Clean' }))

    expect(
      await screen.findByText('No qualifying explicit language was found.'),
    ).toBeInTheDocument()
    expect(screen.getByText('Confidence: 94%')).toBeInTheDocument()
    expect(screen.getByText('Provider: DeepSeek V3.2')).toBeInTheDocument()
    expect(screen.getByText('Basis: saved lyrics')).toBeInTheDocument()
    expect(
      screen.getByText('“We dance together all night”'),
    ).toBeInTheDocument()
  })

  it('edits explicit and excluded word rules used for classification', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(songsWithLyrics))
    const requests = []
    renderPage('/api/ai/classify-explicit', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    clickExplicitAction('Explicit word rules')
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
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    )

    clickExplicitAction('Classify Explicit')
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Classify' }))

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].includedWords).toEqual([
      'custom strong',
      'second strong',
    ])
    expect(requests[0].excludedWords).toEqual(['custom mild'])
  })

  it('upgrades untouched legacy explicit word defaults without replacing custom rules', async () => {
    localStorage.setItem(
      'aiToolExplicitWordRules',
      JSON.stringify({
        included: [
          'fuck',
          'fucking',
          'motherfucker',
          'shit',
          'bitch',
          'cunt',
          'nigga',
          'nigger',
          'pussy',
          'dick',
          'cock',
        ],
        excluded: [
          'damn',
          'hell',
          'crap',
          'ass',
          'alcohol',
          'drunk',
          'weed',
          'marijuana',
          'kiss',
          'kissing',
          'sexy',
          'gun',
          'kill',
        ],
      }),
    )
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    clickExplicitAction('Explicit word rules')
    expect(
      (
        await screen.findByRole('textbox', {
          name: 'Words categorised as explicit',
        })
      ).value,
    ).toContain('fucked')
    expect(
      screen.getByRole('textbox', {
        name: 'Words excluded from explicit categorisation',
      }).value,
    ).toContain('goddamn')
  })

  it('uses the selected default provider for metadata', async () => {
    const requests = []
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    const metadataMenu = openMetadataActions()
    const providerSelector = within(metadataMenu).getByRole('button', {
      name: 'Default Metadata AI Provider',
    })
    fireEvent.mouseDown(providerSelector)
    fireEvent.click(await screen.findByRole('option', { name: 'Gemma 3:4b' }))

    fireEvent.click(
      within(metadataMenu).getByRole('menuitem', {
        name: 'Fetch AI Metadata',
      }),
    )

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].provider).toBe('gemma-3-4b')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('batches multiple songs into one request when a batch size is chosen', async () => {
    const requests = []
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    const metadataMenu = openMetadataActions()
    const batchSelector = within(metadataMenu).getByRole('button', {
      name: 'Songs per AI Prompt',
    })
    fireEvent.mouseDown(batchSelector)
    fireEvent.click(await screen.findByRole('option', { name: '2' }))

    await waitFor(() =>
      expect(localStorage.getItem('aiToolMetadataBatchSize')).toBe('2'),
    )

    fireEvent.click(
      within(metadataMenu).getByRole('menuitem', {
        name: 'Fetch AI Metadata',
      }),
    )

    // Both songs go out in a single request instead of one request each.
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].songIds).toEqual(['song-1', 'song-2'])
  })

  it('shows confidence for the fetched genre', async () => {
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      const request = JSON.parse(options.body)
      return Promise.resolve({
        json: {
          songs: [
            {
              id: request.songIds[0],
              aiGenre: 'Indie rock',
              genreConfidence: 73,
            },
          ],
        },
      })
    })

    clickMetadataAction('Fetch AI Metadata')

    await waitFor(() => {
      expect(screen.getAllByText('73%')).toHaveLength(2)
    })
  })

  it('leaves existing album and year untouched when fetching genre', async () => {
    localStorage.setItem(
      'aiToolAddedSongs',
      JSON.stringify([
        {
          ...songs[0],
          album: 'Original Album',
          year: 1997,
        },
      ]),
    )
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      const request = JSON.parse(options.body)
      return Promise.resolve({
        json: {
          songs: [
            {
              id: request.songIds[0],
              aiGenre: 'Trip Hop',
              genreConfidence: 85,
            },
          ],
        },
      })
    })

    clickMetadataAction('Fetch AI Metadata')

    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem('aiToolAddedSongs'))[0]
      expect(saved.aiGenre).toBe('Trip Hop')
      expect(saved.album).toBe('Original Album')
      expect(saved.year).toBe(1997)
      expect(saved.metadataConfidence.genre).toBe(85)
    })
    expect(
      screen.getByText('Original Album').closest('td').className,
    ).toContain('valueExisting')
    expect(screen.getByText('1997').closest('td').className).toContain(
      'valueExisting',
    )
  })

  it('shows all three fetched genres in their own columns', async () => {
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      const request = JSON.parse(options.body)
      return Promise.resolve({
        json: {
          songs: [
            {
              id: request.songIds[0],
              spotifyGenre: 'Indie Pop',
              musicBrainzGenre: 'Dream Pop',
              aiGenre: 'Shoegaze',
              genreConfidence: 100,
            },
          ],
        },
      })
    })

    clickMetadataAction('Fetch AI Metadata')

    await waitFor(() => {
      expect(screen.getAllByText('Indie Pop').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Dream Pop').length).toBeGreaterThan(0)
      expect(screen.getAllByText('Shoegaze').length).toBeGreaterThan(0)
    })
    // Column headers exist for each source.
    expect(screen.getAllByText('Spotify Genre').length).toBeGreaterThan(0)
    expect(screen.getAllByText('iTunes Genre').length).toBeGreaterThan(0)
  })

  it('shows AI subgenre and token count in their own columns', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify([songs[0]]))
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      const request = JSON.parse(options.body)
      return Promise.resolve({
        json: {
          songs: [
            {
              id: request.songIds[0],
              aiGenre: 'Dance/Electronic',
              aiSubgenre: 'House',
              genreConfidence: 95,
              aiTokens: { input: 210, output: 24, total: 234 },
            },
          ],
        },
      })
    })

    clickMetadataAction('Fetch AI Metadata')

    await waitFor(() => {
      expect(screen.getAllByText('House').length).toBeGreaterThan(0)
    })
    const tokenButton = await screen.findByRole('button', {
      name: 'View token breakdown for First song',
    })
    expect(tokenButton).toHaveTextContent('234')
    expect(screen.getAllByText('AI Subgenre').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Tokens').length).toBeGreaterThan(0)

    fireEvent.click(tokenButton)
    const tokenDialog = await screen.findByRole('dialog', {
      name: 'Token Usage Breakdown',
    })
    expect(tokenDialog).toHaveTextContent(
      '210 input + 24 output = 234 calculated tokens.',
    )
    expect(
      within(tokenDialog).getByRole('row', { name: 'Input tokens 210' }),
    ).toBeInTheDocument()
    expect(
      within(tokenDialog).getByRole('row', { name: 'Output tokens 24' }),
    ).toBeInTheDocument()
    expect(
      within(tokenDialog).getByRole('row', { name: 'Total tokens shown 234' }),
    ).toBeInTheDocument()
  })

  it('opens developer traces from the fetched iTunes and AI genres', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify([songs[0]]))
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      const request = JSON.parse(options.body)
      return Promise.resolve({
        json: {
          songs: [
            {
              id: request.songIds[0],
              musicBrainzGenre: 'Dream Pop',
              aiGenre: 'Shoegaze',
              genreConfidence: 100,
              genreDeveloperTrace: {
                itunes: {
                  source: 'itunes',
                  request:
                    'https://itunes.apple.com/search?entity=song&term=Artist+First+song',
                  songUrl:
                    'https://music.apple.com/us/album/first-song/123456?i=789012',
                  response:
                    '{"results":[{"trackName":"First song","artistName":"Artist","primaryGenreName":"Dream Pop","trackViewUrl":"https://music.apple.com/us/album/first-song/123456?i=789012"}]}',
                  fetchedGenre: 'Dream Pop',
                },
                ai: {
                  source: 'ai',
                  provider: 'deepseek-v3.2',
                  model: 'deepseek.v3.2',
                  prompt: 'Classify the exact genre for First song.',
                  response:
                    '{"genre":"Shoegaze","subgenre":"","basis":"song","confidence":95}',
                  fetchedGenre: 'Shoegaze',
                  attempts: [
                    {
                      number: 1,
                      prompt: 'Classify the exact genre for First song.',
                      response:
                        '{"genre":"Shoegaze","subgenre":"","basis":"song","confidence":95}',
                    },
                  ],
                },
              },
            },
          ],
        },
      })
    })

    clickMetadataAction('Fetch AI Metadata')

    fireEvent.click(
      await screen.findByRole('button', {
        name: 'View iTunes genre developer trace for First song',
      }),
    )
    const itunesDialog = await screen.findByRole('dialog', {
      name: 'iTunes Genre Developer Trace',
    })
    expect(
      within(itunesDialog).getByText(/itunes\.apple\.com\/search/),
    ).toBeInTheDocument()
    expect(
      within(itunesDialog).getByText(/primaryGenreName.*Dream Pop/),
    ).toBeInTheDocument()
    expect(itunesDialog).toHaveTextContent('Fetched genre: Dream Pop')
    const itunesSongLink = within(itunesDialog).getByRole('link', {
      name: 'Open First song on iTunes',
    })
    expect(itunesSongLink).toHaveAttribute(
      'href',
      'https://music.apple.com/us/album/first-song/123456?i=789012',
    )
    expect(itunesSongLink).toHaveAttribute('target', '_blank')
    expect(itunesSongLink).toHaveAttribute('rel', 'noopener noreferrer')
    fireEvent.click(within(itunesDialog).getByRole('button', { name: 'Close' }))
    await waitFor(() =>
      expect(screen.queryByRole('dialog')).not.toBeInTheDocument(),
    )

    fireEvent.click(
      screen.getByRole('button', {
        name: 'View AI genre developer trace for First song',
      }),
    )
    const aiDialog = await screen.findByRole('dialog', {
      name: 'AI Genre Developer Trace',
    })
    expect(
      within(aiDialog).getByText('Classify the exact genre for First song.'),
    ).toBeInTheDocument()
    expect(within(aiDialog).getByText(/"genre":"Shoegaze"/)).toBeInTheDocument()
    expect(aiDialog).toHaveTextContent('Provider: DeepSeek V3.2')
    expect(aiDialog).toHaveTextContent('Model: deepseek.v3.2')

    const saved = JSON.parse(localStorage.getItem('aiToolAddedSongs'))[0]
    expect(saved.genreDeveloperTrace.itunes.fetchedGenre).toBe('Dream Pop')
    expect(saved.genreDeveloperTrace.ai.fetchedGenre).toBe('Shoegaze')
  })

  it('keeps the RAG chat closed until opened', async () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    await screen.findByRole('button', { name: 'Open RAG' })
    expect(
      screen.queryByRole('dialog', { name: 'RAG' }),
    ).not.toBeInTheDocument()
  })

  it('explains how a genre confidence score was resolved when clicked', async () => {
    renderPage('/api/ai/fetch-metadata', (_url, options) => {
      const request = JSON.parse(options.body)
      return Promise.resolve({
        json: {
          songs: [
            {
              id: request.songIds[0],
              aiGenre: 'French House',
              genreConfidence: 100,
              confidenceBreakdown: {
                genre: {
                  source: 'verified',
                  spotify: 'French House',
                  musicBrainz: 'French House',
                  ai: 'French House',
                  confidence: 100,
                },
              },
            },
          ],
        },
      })
    })

    clickMetadataAction('Fetch AI Metadata')

    const badge = await screen.findByText('100%')
    fireEvent.click(badge)

    const breakdownDialog = await screen.findByRole('dialog')
    expect(
      within(breakdownDialog).getByText(/How the Genre confidence was/i),
    ).toBeInTheDocument()
    expect(
      within(breakdownDialog).getByText(/Two independent sources agree/i),
    ).toBeInTheDocument()
    expect(within(breakdownDialog).getByText('Spotify')).toBeInTheDocument()
    expect(within(breakdownDialog).getByText('iTunes')).toBeInTheDocument()
  })

  it('clears only AI-fetched metadata for selected songs', async () => {
    const queuedSongs = [
      {
        ...songs[0],
        album: 'AI Album',
        year: 2024,
        aiGenre: 'AI Rock',
        genreDeveloperTrace: {
          ai: { prompt: 'old prompt', response: 'old response' },
        },
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

    clickMetadataAction('Clear Fetched Metadata')

    const dialog = await screen.findByRole('dialog', {
      name: 'Clear fetched metadata?',
    })
    expect(requests).toEqual([])
    fireEvent.click(
      within(dialog).getByRole('button', { name: 'Clear metadata' }),
    )

    await waitFor(() => expect(requests).toHaveLength(1))
    // Fetched genres are stored server-side, so the bulk clear must clear them
    // there as well as in the browser.
    const genreFlags = {
      aiGenre: true,
      aiSubgenre: true,
      spotifyGenre: true,
      musicBrainzGenre: true,
      genreConfidence: true,
    }
    expect(requests[0]).toEqual({
      songs: [
        { id: 'song-1', album: true, year: true, ...genreFlags },
        { id: 'song-2', album: false, year: false, ...genreFlags },
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
      expect(saved[0].genreDeveloperTrace).toBeUndefined()
      expect(saved[1]).toMatchObject({
        album: 'Original Album',
        year: 1999,
        aiGenre: '',
        metadataConfidence: {},
      })
    })
  })

  it('clears one genre column from its header without touching the others', async () => {
    const queuedSongs = songs.map((song) => ({
      ...song,
      musicBrainzGenre: 'iTunes Rock',
      aiGenre: 'AI Rock',
      spotifyGenre: 'Spotify Rock',
      genreDeveloperTrace: {
        itunes: { request: 'itunes request', response: 'itunes response' },
        ai: { prompt: 'ai prompt', response: 'ai response' },
      },
      aiFields: { musicBrainzGenre: true, aiGenre: true, spotifyGenre: true },
    }))
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(queuedSongs))
    const requests = []
    renderPage('/api/ai/clear-metadata', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songIds: ['song-1', 'song-2'] } })
    })

    fireEvent.click(
      screen.getByRole('button', { name: 'Clear fetched iTunes Genre' }),
    )
    fireEvent.click(
      within(screen.getByRole('menu')).getByRole('menuitem', {
        name: 'Clear for 2 selected songs',
      }),
    )

    // Fetched genres are stored server-side, so clearing one column must clear
    // it there too or it returns on the next reconcile.
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toEqual({
      songs: [
        { id: 'song-1', musicBrainzGenre: true },
        { id: 'song-2', musicBrainzGenre: true },
      ],
    })
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem('aiToolAddedSongs'))
      expect(saved[0].musicBrainzGenre).toBe('')
      expect(saved[0].aiGenre).toBe('AI Rock')
      expect(saved[0].spotifyGenre).toBe('Spotify Rock')
      expect(saved[0].genreDeveloperTrace.itunes).toBeUndefined()
      expect(saved[0].genreDeveloperTrace.ai).toBeDefined()
      expect(saved[1].musicBrainzGenre).toBe('')
    })
  })

  it('clears the persisted explicit column from its header', async () => {
    const queuedSongs = songs.map((song) => ({
      ...song,
      explicitStatus: 'e',
      explicitReason: 'strong language',
      aiGenre: 'AI Rock',
      aiFields: { explicitStatus: true, aiGenre: true },
    }))
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(queuedSongs))
    const requests = []
    renderPage('/api/ai/clear-metadata', (_url, options) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songIds: ['song-1', 'song-2'] } })
    })

    fireEvent.click(
      screen.getByRole('button', { name: 'Clear fetched Explicit' }),
    )
    fireEvent.click(
      within(screen.getByRole('menu')).getByRole('menuitem', {
        name: 'Clear for 2 selected songs',
      }),
    )

    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0]).toEqual({
      songs: [
        { id: 'song-1', explicit: true },
        { id: 'song-2', explicit: true },
      ],
    })
    await waitFor(() => {
      const saved = JSON.parse(localStorage.getItem('aiToolAddedSongs'))
      expect(saved[0].explicitStatus).toBe('')
      expect(saved[0].explicitReason).toBe('')
      expect(saved[0].aiGenre).toBe('AI Rock')
      expect(saved[1].explicitStatus).toBe('')
    })
  })

  it('lets the user choose visible columns from the Columns menu', () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    const menu = openColumnMenu()
    fireEvent.click(within(menu).getByRole('menuitem', { name: 'Album' }))
    fireEvent.keyDown(menu, { key: 'Escape' })

    expect(
      screen.queryByRole('columnheader', { name: 'Album' }),
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Year' }),
    ).toBeInTheDocument()

    const reopenedMenu = openColumnMenu()
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

    const menu = openColumnMenu()
    const confidenceToggle = within(menu).getByRole('menuitem', {
      name: 'All Confidence Columns',
    })
    fireEvent.click(confidenceToggle)
    fireEvent.keyDown(menu, { key: 'Escape' })

    expect(
      screen.queryByRole('columnheader', { name: 'Genre Confidence' }),
    ).not.toBeInTheDocument()

    const reopenedMenu = openColumnMenu()
    fireEvent.click(
      within(reopenedMenu).getByRole('menuitem', {
        name: 'All Confidence Columns',
      }),
    )
    fireEvent.keyDown(reopenedMenu, { key: 'Escape' })

    expect(
      screen.getByRole('columnheader', { name: 'Genre Confidence' }),
    ).toBeInTheDocument()
  })

  it('hides and shows confidence columns from the toolbar', () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    clickMetadataAction('Hide Confidence')
    expect(
      screen.queryByRole('columnheader', { name: 'Genre Confidence' }),
    ).not.toBeInTheDocument()

    clickMetadataAction('Show Confidence')
    expect(
      screen.getByRole('columnheader', { name: 'Genre Confidence' }),
    ).toBeInTheDocument()
  })

  it('collapses and expands the song tools toolbar', async () => {
    renderPage('/api/unused', () => Promise.resolve({ json: {} }))

    const toggle = screen.getByRole('button', { name: /Song tools/ })
    expect(screen.getByRole('button', { name: 'Add songs' })).toBeVisible()

    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'false')
    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: 'Add songs' }),
      ).not.toBeInTheDocument(),
    )

    fireEvent.click(toggle)
    expect(toggle).toHaveAttribute('aria-expanded', 'true')
    expect(
      await screen.findByRole('button', { name: 'Add songs' }),
    ).toBeVisible()
  })
  it('explains the empty queue instead of showing a bare table', async () => {
    renderPageWithoutSelection()

    expect(await screen.findByText('No songs added yet.')).toBeInTheDocument()
    expect(
      screen.getByText(/pick\s+tracks from your library/i),
    ).toBeInTheDocument()
  })

  it('says lyrics are not fetched rather than failed', async () => {
    renderPageWithoutSelection({}, { queuedSongs: songs })

    await waitFor(() =>
      expect(screen.getAllByText('Not fetched')).toHaveLength(2),
    )
    expect(screen.queryByText('Failed')).not.toBeInTheDocument()
  })

  it('filters the library picker and can reload it', async () => {
    mockGetList.mockResolvedValue({
      data: [
        { id: 'song-1', title: 'First song', artist: 'Artist' },
        { id: 'song-9', title: 'Deep cut', artist: 'Another band' },
      ],
    })
    renderPageWithoutSelection()

    fireEvent.click(await screen.findByRole('button', { name: 'Add songs' }))
    expect(await screen.findByText('Deep cut')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Search your library'), {
      target: { value: 'another band' },
    })

    expect(screen.getByText('Deep cut')).toBeInTheDocument()
    expect(screen.queryByText('First song')).not.toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Search your library'), {
      target: { value: 'nothing matches this' },
    })
    expect(
      screen.getByText('No songs in your library match that search.'),
    ).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Refresh library' }))
    await waitFor(() => expect(mockGetList).toHaveBeenCalledTimes(2))
  })

  it('reports a library that will not load and offers a retry', async () => {
    mockGetList.mockRejectedValueOnce(new Error('Library is offline'))
    mockGetList.mockResolvedValueOnce({ data: songs })
    renderPageWithoutSelection()

    fireEvent.click(await screen.findByRole('button', { name: 'Add songs' }))

    expect(await screen.findByText('Library is offline')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Retry' }))

    expect(await screen.findByText('First song')).toBeInTheDocument()
    expect(screen.queryByText('Library is offline')).not.toBeInTheDocument()
  })

  it('warns instead of crashing when browser storage is full', async () => {
    // The queue is seeded first; only the writes the page makes afterwards hit
    // the full-storage error.
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(songs))
    mockHttpClient.mockImplementation((url) => {
      if (url === '/api/ai/status') {
        return Promise.resolve({
          json: { services: [], whisperModel: 'large-v3' },
        })
      }
      if (url === '/api/ai/rag/status') {
        return Promise.resolve({ json: defaultRAGStatus })
      }
      if (url.startsWith('/api/song?')) return Promise.resolve({ json: songs })
      return Promise.resolve({ json: {} })
    })
    const setItem = vi
      .spyOn(localStorage, 'setItem')
      .mockImplementation((key) => {
        if (key === 'aiToolAddedSongs') {
          const error = new Error('QuotaExceededError')
          error.name = 'QuotaExceededError'
          throw error
        }
      })

    try {
      render(
        <MemoryRouter initialEntries={['/ai-tool']}>
          <AiToolPage />
        </MemoryRouter>,
      )

      expect(await screen.findByText(/Browser storage is full/)).toBeVisible()
      // The queue itself still renders, so the page keeps working.
      expect(screen.getByText('First song')).toBeInTheDocument()
    } finally {
      setItem.mockRestore()
    }
  })

  it('shows a dismissible banner when an action fails', async () => {
    renderPage('/api/ai/fetch-metadata', () =>
      Promise.reject(new Error('The AI provider is unreachable')),
    )

    clickMetadataAction('Fetch AI Metadata')

    const banner = await screen.findByRole('alert')
    expect(
      within(banner).getByText('The AI provider is unreachable'),
    ).toBeInTheDocument()

    fireEvent.click(
      within(banner).getByRole('button', { name: 'Dismiss error' }),
    )

    await waitFor(() =>
      expect(screen.queryByRole('alert')).not.toBeInTheDocument(),
    )
  })

  it('skips songs that already have a fetched genre when auto-fetching', async () => {
    localStorage.setItem(
      'aiToolAddedSongs',
      JSON.stringify([{ ...songs[0], aiGenre: 'Indie rock' }, { ...songs[1] }]),
    )
    const requests = []
    renderPage('/api/ai/fetch-metadata', (_url, options = {}) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { songs: [] } })
    })

    const metadataMenu = openMetadataActions()
    fireEvent.click(
      within(metadataMenu).getByRole('checkbox', {
        name: 'Fetch All Song Metadata',
      }),
    )

    // Only the song without a genre is sent, so a finished queue stops
    // spending tokens on answers it already has.
    await waitFor(() =>
      expect(requests.map((request) => request.songIds)).toEqual([['song-2']]),
    )
  })

  it('shows progress for explicit classification and can stop it', async () => {
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(songsWithLyrics))
    const signals = []
    renderPage('/api/ai/classify-explicit', createAbortableRequest(signals))

    clickExplicitAction('Classify Explicit')
    fireEvent.click(await screen.findByRole('button', { name: 'Classify' }))

    expect(await screen.findByText(/2 songs in progress/)).toBeInTheDocument()
    expect(
      await screen.findByText(/^Classifying explicit content:/),
    ).toBeInTheDocument()

    // The Classify dialog is still fading out and keeps the page behind it
    // hidden from role queries, so reach the Stop button by its text.
    fireEvent.click(screen.getByText('Stop').closest('button'))

    await waitFor(() => expect(signals[0].aborted).toBe(true))
    expect(await screen.findByText(/Stopped/)).toBeInTheDocument()
  })

  it('asks the server for a long queue in batches', async () => {
    const manySongs = Array.from({ length: 250 }, (_, index) => ({
      id: `song-${index + 1}`,
      title: `Song ${index + 1}`,
      artist: 'Artist',
    }))
    localStorage.setItem('aiToolAddedSongs', JSON.stringify(manySongs))
    mockHttpClient.mockImplementation((url) => {
      if (url === '/api/ai/status') {
        return Promise.resolve({
          json: { services: [], whisperModel: 'large-v3' },
        })
      }
      if (url === '/api/ai/rag/status') {
        return Promise.resolve({ json: defaultRAGStatus })
      }
      if (url.startsWith('/api/song?')) {
        const ids = [...new URLSearchParams(url.split('?')[1]).getAll('id')]
        return Promise.resolve({
          json: ids.map((id) => manySongs.find((song) => song.id === id)),
        })
      }
      return Promise.resolve({ json: {} })
    })

    render(
      <MemoryRouter initialEntries={['/ai-tool']}>
        <AiToolPage />
      </MemoryRouter>,
    )

    await waitFor(() => {
      const songRequests = mockHttpClient.mock.calls
        .map(([url]) => url)
        .filter((url) => url.startsWith('/api/song?'))
      expect(songRequests).toHaveLength(3)
      // No single URL may carry the whole queue, or it overflows the server's
      // URL limit and the reconcile silently stops working.
      songRequests.forEach((url) => expect(url.length).toBeLessThan(4000))
    })
  })

  it('sends chat on Enter but keeps Shift+Enter for a new line', async () => {
    const requests = []
    renderPage('/api/ai/chat', (_url, options = {}) => {
      requests.push(JSON.parse(options.body))
      return Promise.resolve({ json: { response: 'Answer', sources: [] } })
    })

    fireEvent.click(await screen.findByRole('button', { name: 'Open RAG' }))
    const ragDialog = await screen.findByRole('dialog', { name: 'RAG' })
    const input = within(ragDialog).getByLabelText('Ask RAG about your library')

    fireEvent.change(input, { target: { value: 'First line' } })
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true })
    expect(requests).toEqual([])

    // A keystroke still inside an IME composition must not submit either.
    fireEvent.keyDown(input, { key: 'Enter', keyCode: 229 })
    expect(requests).toEqual([])

    fireEvent.keyDown(input, { key: 'Enter' })
    await waitFor(() => expect(requests).toHaveLength(1))
    expect(requests[0].message).toBe('First line')
  })
})
