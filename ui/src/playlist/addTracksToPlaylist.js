import isPlainObject from 'lodash/isPlainObject'

const PAGE_SIZE = 500

const collectPlaylistTrackIds = async (dataProvider, playlistId) => {
  const collected = new Set()
  let page = 1
  let total = Infinity

  while (collected.size < total) {
    const response = await dataProvider.getList('playlistTrack', {
      pagination: { page, perPage: PAGE_SIZE },
      sort: { field: 'id', order: 'ASC' },
      filter: { playlist_id: playlistId },
    })

    const tracks = response?.data ?? []
    tracks.forEach((track) => {
      if (track?.mediaFileId) {
        collected.add(track.mediaFileId)
      }
    })

    total = typeof response?.total === 'number' ? response.total : collected.size
    if (tracks.length === 0 || collected.size >= total) {
      break
    }
    page += 1
  }

  return collected
}

const normalizePayload = (payload) => {
  if (!payload || !isPlainObject(payload)) {
    return {}
  }
  const { ids, albumIds, artistIds, discs, ...rest } = payload
  const normalized = { ...rest }
  if (Array.isArray(ids)) {
    normalized.ids = ids
  }
  if (Array.isArray(albumIds) && albumIds.length) {
    normalized.albumIds = albumIds
  }
  if (Array.isArray(artistIds) && artistIds.length) {
    normalized.artistIds = artistIds
  }
  if (Array.isArray(discs) && discs.length) {
    normalized.discs = discs
  }
  return normalized
}

export const addTracksToPlaylist = async (
  dataProvider,
  playlistId,
  payload = {},
  options = {},
) => {
  const { skipDuplicates = true, existingTrackIds = null } = options
  const normalized = normalizePayload(payload)
  let clientSkipped = 0

  if (
    skipDuplicates &&
    Array.isArray(normalized.ids) &&
    normalized.ids.length > 0 &&
    playlistId
  ) {
    let baseSet = existingTrackIds
    try {
      if (!baseSet) {
        baseSet = await collectPlaylistTrackIds(dataProvider, playlistId)
      }
    } catch (error) {
      baseSet = baseSet instanceof Set ? baseSet : new Set()
    }

    const seen = baseSet instanceof Set ? new Set(baseSet) : new Set()
    const filtered = []

    normalized.ids.forEach((id) => {
      if (!id) {
        return
      }
      if (seen.has(id)) {
        clientSkipped += 1
        return
      }
      seen.add(id)
      filtered.push(id)
    })

    normalized.ids = filtered
  }

  const hasIds = Array.isArray(normalized.ids) && normalized.ids.length > 0
  const hasAlbums = Array.isArray(normalized.albumIds) && normalized.albumIds.length > 0
  const hasArtists =
    Array.isArray(normalized.artistIds) && normalized.artistIds.length > 0
  const hasDiscs = Array.isArray(normalized.discs) && normalized.discs.length > 0

  if (!hasIds && !hasAlbums && !hasArtists && !hasDiscs) {
    return { addedCount: 0, skippedCount: clientSkipped }
  }

  const response = await dataProvider.addToPlaylist(playlistId, normalized)
  const addedFromServer = Number(response?.data?.added ?? 0)
  const skippedFromServer = Number(response?.data?.skipped ?? 0)

  return {
    addedCount: addedFromServer,
    skippedCount: skippedFromServer + clientSkipped,
  }
}

export const formatPlaylistToast = ({ addedCount = 0, skippedCount = 0 } = {}) => {
  if (addedCount === 0 && skippedCount > 0) {
    return 'No changes — duplicates'
  }
  if (skippedCount > 0) {
    return `Added ${addedCount} • Skipped ${skippedCount} duplicates`
  }
  return `Added ${addedCount} songs`
}

