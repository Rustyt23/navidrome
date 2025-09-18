import * as React from 'react'
import { TestContext } from 'ra-test'
import { DataProviderContext } from 'react-admin'
import {
  cleanup,
  fireEvent,
  render,
  waitFor,
  screen,
} from '@testing-library/react'
import { AddToPlaylistDialog } from './AddToPlaylistDialog'
import { describe, beforeAll, afterEach, it, expect, vi } from 'vitest'

const hoistedMocks = vi.hoisted(() => ({
  addTracksToPlaylistMock: vi.fn().mockResolvedValue({
    addedCount: 2,
    skippedCount: 0,
  }),
}))

vi.mock('../playlist/addTracksToPlaylist', () => ({
  addTracksToPlaylist: hoistedMocks.addTracksToPlaylistMock,
  formatPlaylistToast: vi
    .fn()
    .mockImplementation(({ addedCount, skippedCount }) =>
      skippedCount > 0
        ? `Added ${addedCount} • Skipped ${skippedCount} duplicates`
        : `Added ${addedCount} songs`,
    ),
}))

const { addTracksToPlaylistMock } = hoistedMocks

const mockData = [
  { id: 'sample-id1', name: 'sample playlist 1', ownerId: 'admin' },
  { id: 'sample-id2', name: 'sample playlist 2', ownerId: 'admin' },
]
const mockIndexedData = {
  'sample-id1': {
    id: 'sample-id1',
    name: 'sample playlist 1',
    ownerId: 'admin',
  },
  'sample-id2': {
    id: 'sample-id2',
    name: 'sample playlist 2',
    ownerId: 'admin',
  },
}
const selectedIds = ['song-1', 'song-2']

const createTestUtils = (mockDataProvider) =>
  render(
    <DataProviderContext.Provider value={mockDataProvider}>
      <TestContext
        initialState={{
          addToPlaylistDialog: {
            open: true,
            selectedIds: selectedIds,
          },
          admin: {
            ui: { optimistic: false },
            resources: {
              playlist: {
                data: mockIndexedData,
                list: {
                  cachedRequests: {
                    '{"pagination":{"page":1,"perPage":-1},"sort":{"field":"name","order":"ASC"},"filter":{"smart":false}}':
                      {
                        ids: ['sample-id1', 'sample-id2'],
                        total: 2,
                      },
                  },
                },
              },
            },
          },
        }}
      >
        <AddToPlaylistDialog />
      </TestContext>
    </DataProviderContext.Provider>,
  )

describe('AddToPlaylistDialog', () => {
  beforeAll(() => localStorage.setItem('userId', 'admin'))
  afterEach(() => {
    addTracksToPlaylistMock.mockClear()
    vi.clearAllMocks()
    cleanup()
  })

  it('skips duplicate songs when adding to existing playlists', async () => {
    const mockDataProvider = {
      getList: vi
        .fn()
        .mockResolvedValue({ data: mockData, total: mockData.length }),
      getOne: vi.fn().mockResolvedValue({ data: { id: 'song-3' }, total: 1 }),
      addToPlaylist: vi.fn().mockResolvedValue({ data: { added: 2, skipped: 1 } }),
    }

    createTestUtils(mockDataProvider)

    // Filter to see sample playlists
    let textBox = screen.getByRole('textbox')
    fireEvent.change(textBox, { target: { value: 'sample' } })

    // Click on first playlist
    const firstPlaylist = screen.getByText('sample playlist 1')
    fireEvent.click(firstPlaylist)

    // Click on second playlist
    const secondPlaylist = screen.getByText('sample playlist 2')
    fireEvent.click(secondPlaylist)

    await waitFor(() => {
      expect(screen.getByTestId('playlist-add')).not.toBeDisabled()
    })
    fireEvent.click(screen.getByTestId('playlist-add'))
    await waitFor(() => {
      expect(addTracksToPlaylistMock).toHaveBeenCalledTimes(2)
    })
    const firstCall = addTracksToPlaylistMock.mock.calls[0]
    expect(firstCall[1]).toBe('sample-id1')
    expect(firstCall[2]).toEqual({ ids: selectedIds })
    expect(firstCall[3]).toEqual({ skipDuplicates: true })

    const secondCall = addTracksToPlaylistMock.mock.calls[1]
    expect(secondCall[1]).toBe('sample-id2')
    expect(secondCall[2]).toEqual({ ids: selectedIds })
    expect(secondCall[3]).toEqual({ skipDuplicates: true })
  })

  it('adds distinct songs to a new playlist', async () => {
    addTracksToPlaylistMock.mockResolvedValueOnce({
      addedCount: 2,
      skippedCount: 0,
    })
    const mockDataProvider = {
      getList: vi
        .fn()
        .mockResolvedValue({ data: mockData, total: mockData.length }),
      getOne: vi.fn().mockResolvedValue({ data: { id: 'song-3' }, total: 1 }),
      create: vi.fn().mockResolvedValue({
        data: { id: 'created-id1', name: 'created-name' },
      }),
      addToPlaylist: vi.fn().mockResolvedValue({ data: { added: 2, skipped: 0 } }),
    }

    createTestUtils(mockDataProvider)

    // Type a new playlist name and press Enter to create it
    let textBox = screen.getByRole('textbox')
    fireEvent.change(textBox, { target: { value: 'sample' } })
    fireEvent.keyDown(textBox, { key: 'Enter' })

    await waitFor(() => {
      expect(screen.getByTestId('playlist-add')).not.toBeDisabled()
    })
    fireEvent.click(screen.getByTestId('playlist-add'))
    await waitFor(() => {
      expect(mockDataProvider.create).toHaveBeenNthCalledWith(1, 'playlist', {
        data: { name: 'sample' },
      })
    })
    await waitFor(() => {
      expect(addTracksToPlaylistMock).toHaveBeenCalled()
    })
    const call = addTracksToPlaylistMock.mock.calls.at(-1)
    expect(call[1]).toBe('created-id1')
    expect(call[2]).toEqual({ ids: selectedIds })
    expect(call[3]).toEqual({ skipDuplicates: true, existingTrackIds: new Set() })
  })

  it('adds distinct songs to multiple new playlists', async () => {
    addTracksToPlaylistMock.mockResolvedValue({ addedCount: 2, skippedCount: 0 })
    const mockDataProvider = {
      getList: vi
        .fn()
        .mockResolvedValue({ data: mockData, total: mockData.length }),
      getOne: vi.fn().mockResolvedValue({ data: { id: 'song-3' }, total: 1 }),
      create: vi.fn().mockResolvedValue({
        data: { id: 'created-id1', name: 'created-name' },
      }),
      addToPlaylist: vi.fn().mockResolvedValue({ data: { added: 2, skipped: 0 } }),
    }

    createTestUtils(mockDataProvider)

    // Create first playlist
    let textBox = screen.getByRole('textbox')
    fireEvent.change(textBox, { target: { value: 'sample' } })
    fireEvent.keyDown(textBox, { key: 'Enter' })

    // Create second playlist
    fireEvent.change(textBox, { target: { value: 'new playlist' } })
    fireEvent.keyDown(textBox, { key: 'Enter' })

    await waitFor(() => {
      expect(screen.getByTestId('playlist-add')).not.toBeDisabled()
    })
    fireEvent.click(screen.getByTestId('playlist-add'))
    await waitFor(() => {
      expect(mockDataProvider.create).toHaveBeenCalledTimes(2)
    })
    expect(addTracksToPlaylistMock).toHaveBeenCalledTimes(2)
  })
})
