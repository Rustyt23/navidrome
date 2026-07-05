import React from 'react'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AiDashboardPage from './AiDashboardPage'

const { mockHttpClient } = vi.hoisted(() => ({ mockHttpClient: vi.fn() }))

vi.mock('../dataProvider', () => ({ httpClient: mockHttpClient }))
vi.mock('react-admin', async (importOriginal) => {
  const actual = await importOriginal()
  return { ...actual, Title: () => null }
})

const report = {
  libraryHealthScore: 82,
  totalSongs: 100,
  indexedSongs: 80,
  indexedCoveragePercent: 80,
  songsWithLyrics: 70,
  songsMissingLyrics: 30,
  songsWithGenre: 90,
  songsMissingGenre: 10,
  songsWithYear: 85,
  songsMissingYear: 15,
  songsWithBpm: 75,
  songsMissingBpm: 25,
  songsWithLufs: 65,
  songsMissingLufs: 35,
  cleanSongs: 75,
  explicitSongs: 10,
  reviewNeededSongs: 15,
  explicitRiskSongs: 10,
  duplicateRiskCount: 2,
  loudnessIssueCount: 3,
  overplayedSongs: 8,
  underusedSongs: 20,
  playlistsTotal: 10,
  playlistsAnalyzed: 10,
  playlistsWithIssues: 4,
  recommendationsAvailable: 35,
  ragEnabled: true,
  vectorDbOnline: true,
  collectionExists: true,
}

describe('AiDashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockHttpClient.mockResolvedValue({ json: report })
  })

  afterEach(() => cleanup())

  it('renders dashboard summary cards and quality panels', async () => {
    render(<AiDashboardPage />)

    expect(await screen.findByText('Library Health Score')).toBeInTheDocument()
    expect(screen.getByText('82/100')).toBeInTheDocument()
    expect(screen.getByText('Total Songs')).toBeInTheDocument()
    expect(screen.getByText('RAG Indexed Songs')).toBeInTheDocument()
    expect(screen.getByText('Playlists With Issues')).toBeInTheDocument()
    expect(screen.getByText('Metadata Quality')).toBeInTheDocument()
    expect(screen.getByText('Lyrics Coverage')).toBeInTheDocument()
    expect(screen.getByText('Explicit / Retail Risk')).toBeInTheDocument()
    expect(screen.getByText('Recommendation Summary')).toBeInTheDocument()
    expect(mockHttpClient).toHaveBeenCalledWith('/api/ai/rag/reports/dashboard')
  })

  it('refreshes the dashboard on demand', async () => {
    render(<AiDashboardPage />)
    const refresh = await screen.findByRole('button', {
      name: 'Refresh Dashboard',
    })
    fireEvent.click(refresh)
    await waitFor(() => expect(mockHttpClient).toHaveBeenCalledTimes(2))
  })

  it('exports the current report as JSON and CSV', async () => {
    const createObjectURL = vi.fn(() => 'blob:dashboard')
    const revokeObjectURL = vi.fn()
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: createObjectURL,
    })
    Object.defineProperty(URL, 'revokeObjectURL', {
      configurable: true,
      value: revokeObjectURL,
    })
    const downloads = []
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(function () {
        downloads.push(this.download)
      })

    render(<AiDashboardPage />)
    await screen.findByText('Library Health Score')
    fireEvent.click(screen.getByRole('button', { name: 'Export JSON' }))
    fireEvent.click(screen.getByRole('button', { name: 'Export CSV' }))

    expect(createObjectURL).toHaveBeenCalledTimes(2)
    expect(revokeObjectURL).toHaveBeenCalledTimes(2)
    expect(downloads).toEqual(['ai-dashboard.json', 'ai-dashboard.csv'])
    click.mockRestore()
  })
})
