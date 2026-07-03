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

const renderPage = (actionPath, actionRequest) => {
  mockHttpClient.mockImplementation((url, options = {}) => {
    if (url === '/api/ai/status') {
      return Promise.resolve({ json: { services: [] } })
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
  fireEvent.click(screen.getAllByRole('checkbox')[0])
}

const chooseModel = async (model) => {
  const dialog = await screen.findByRole('dialog')
  fireEvent.mouseDown(
    within(dialog).getByRole('button', { name: 'Gemini 2.5' }),
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
    expect(requests[0]).toEqual({
      songIds: ['song-1', 'song-2'],
      provider: 'gemini-3.5',
    })
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
