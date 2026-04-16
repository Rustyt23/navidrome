const normalizeTrackName = (track) => {
  if (!track) return ''
  return (
    track.title || track.name || track.songTitle || track.mediaFileTitle || track.path || ''
  )
}

const normalizeArtistName = (track) => {
  if (!track) return ''
  return track.artist || track.artistName || track.albumArtist || ''
}

export const buildDuplicateInfo = (tracksA = [], tracksB = []) => {
  const bByMediaId = new Set(
    tracksB
      .map((track) => track?.mediaFileId)
      .filter((mediaFileId) => mediaFileId !== undefined && mediaFileId !== null)
  )

  const deduped = new Map()
  tracksA.forEach((track) => {
    const mediaFileId = track?.mediaFileId
    if (!mediaFileId || !bByMediaId.has(mediaFileId) || deduped.has(mediaFileId)) {
      return
    }

    deduped.set(mediaFileId, {
      mediaFileId,
      title: normalizeTrackName(track),
      artist: normalizeArtistName(track),
    })
  })

  return Array.from(deduped.values()).sort((a, b) =>
    `${a.title} ${a.artist}`.localeCompare(`${b.title} ${b.artist}`)
  )
}

export const buildDuplicateTrackIdsByPlaylist = (tracksA = [], tracksB = []) => {
  const duplicateMediaIds = new Set(buildDuplicateInfo(tracksA, tracksB).map((t) => t.mediaFileId))

  const getTrackIds = (tracks) =>
    tracks
      .filter((track) => track?.id && duplicateMediaIds.has(track?.mediaFileId))
      .map((track) => track.id)

  return {
    left: getTrackIds(tracksA),
    right: getTrackIds(tracksB),
  }
}
