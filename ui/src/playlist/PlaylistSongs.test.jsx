import { describe, it, expect, vi } from 'vitest'
import { selectPlaylistTrackIds } from './PlaylistSongs.jsx'

const createRecords = (count, offset = 1) =>
  Array.from({ length: count }, (_, index) => ({ id: index + offset }))

describe('selectPlaylistTrackIds', () => {
  it('requests all playlist tracks when selecting the current page and more records exist', async () => {
    const idsToSelect = Array.from({ length: 50 }, (_, index) => index + 1)
    const contextTotal = 63
    const onSelect = vi.fn()
    const getList = vi
      .fn()
      .mockResolvedValue({ data: createRecords(contextTotal) })

    await selectPlaylistTrackIds({
      idsToSelect,
      pageIds: idsToSelect,
      selectedIds: [],
      contextTotal,
      filterValues: { foo: 'bar' },
      playlistId: 'playlist-id',
      currentSort: { field: 'title', order: 'DESC' },
      dataProvider: { getList },
      onSelect,
    })

    expect(getList).toHaveBeenCalledWith('playlistTrack', {
      filter: { foo: 'bar', playlist_id: 'playlist-id' },
      pagination: { page: 1, perPage: 0 },
      sort: { field: 'title', order: 'DESC' },
    })

    expect(onSelect).toHaveBeenCalledWith(
      expect.arrayContaining(Array.from({ length: contextTotal }, (_, index) => index + 1)),
    )
    expect(onSelect.mock.calls[0][0]).toHaveLength(contextTotal)
  })

  it('forwards idsToSelect when there is nothing else to load', async () => {
    const idsToSelect = ['track-1', 'track-2']
    const onSelect = vi.fn()
    const getList = vi.fn()

    await selectPlaylistTrackIds({
      idsToSelect,
      pageIds: idsToSelect,
      selectedIds: [],
      contextTotal: idsToSelect.length,
      filterValues: {},
      playlistId: 'playlist-id',
      currentSort: undefined,
      dataProvider: { getList },
      onSelect,
    })

    expect(getList).not.toHaveBeenCalled()
    expect(onSelect).toHaveBeenCalledWith(idsToSelect)
  })
})
