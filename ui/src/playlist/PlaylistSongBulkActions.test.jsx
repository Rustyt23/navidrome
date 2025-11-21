import { describe, it, expect, vi } from 'vitest'
import { resolveSelectedMediaIds } from './PlaylistSongBulkActions.jsx'

describe('resolveSelectedMediaIds', () => {
  it('returns media ids from the current page data when already loaded', async () => {
    const data = {
      '1': { id: '1', mediaFileId: 'media-1' },
      '2': { id: '2', mediaFileId: 'media-2' },
    }

    const selectedIds = ['1', '2']
    const dataProvider = { getList: vi.fn() }

    const result = await resolveSelectedMediaIds({
      selectedIds,
      data,
      playlistId: 'playlist-id',
      dataProvider,
    })

    expect(dataProvider.getList).not.toHaveBeenCalled()
    expect(result).toEqual(['media-1', 'media-2'])
  })

  it('fetches playlist tracks to resolve media ids when needed', async () => {
    const selectedIds = ['1', '2', '3']
    const data = { '1': { id: '1', mediaFileId: 'media-1' } }
    const dataProvider = {
      getList: vi.fn().mockResolvedValue({
        data: [
          { id: '1', mediaFileId: 'media-1' },
          { id: '2', mediaFileId: 'media-2' },
          { id: '3', mediaFileId: 'media-3' },
        ],
      }),
    }

    const result = await resolveSelectedMediaIds({
      selectedIds,
      data,
      playlistId: 'playlist-id',
      dataProvider,
    })

    expect(dataProvider.getList).toHaveBeenCalledWith('playlistTrack', {
      filter: { playlist_id: 'playlist-id' },
      pagination: { page: 1, perPage: 0 },
      sort: { field: 'id', order: 'ASC' },
    })
    expect(result).toEqual(['media-1', 'media-2', 'media-3'])
  })

  it('falls back to the selected ids when the request fails', async () => {
    const selectedIds = ['1', '2']
    const dataProvider = {
      getList: vi.fn().mockRejectedValue(new Error('network error')),
    }

    const result = await resolveSelectedMediaIds({
      selectedIds,
      data: {},
      playlistId: 'playlist-id',
      dataProvider,
    })

    expect(result).toEqual(selectedIds)
  })
})
