const TITLE_PATHS = [
  'trackTitle',
  'track_title',
  'track.title',
  'track.name',
  'currentTrack.title',
  'current_track.title',
  'nowPlaying.title',
  'now_playing.title',
  'songTitle',
  'song_title',
  'song.title',
  'streamTitle',
  'stream_title',
  'title',
]

const ARTIST_PATHS = [
  'trackArtist',
  'track_artist',
  'track.artist',
  'track.performer',
  'currentTrack.artist',
  'current_track.artist',
  'nowPlaying.artist',
  'now_playing.artist',
  'songArtist',
  'song_artist',
  'song.artist',
  'artistName',
  'artist_name',
  'albumArtist',
  'album_artist',
  'performer',
  'artist',
]

export const normalizeValue = (value) => {
  if (typeof value === 'string') {
    return value.trim()
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value)
  }
  return ''
}

const getPathValue = (source, path) => {
  if (!source || typeof source !== 'object') {
    return undefined
  }

  return path.split('.').reduce((value, key) => {
    if (!value || typeof value !== 'object') {
      return undefined
    }
    return value[key]
  }, source)
}

const pickFirstValue = (source, paths) => {
  for (const path of paths) {
    const value = normalizeValue(getPathValue(source, path))
    if (value) {
      return value
    }
  }
  return ''
}

const normalizedEquals = (left, right) => {
  const normalizedLeft = normalizeValue(left).toLocaleLowerCase()
  const normalizedRight = normalizeValue(right).toLocaleLowerCase()
  return Boolean(
    normalizedLeft && normalizedRight && normalizedLeft === normalizedRight,
  )
}

const scoreMetadataEntry = (entry, status) => {
  if (!entry || typeof entry !== 'object') {
    return -1
  }

  let score = 0
  const activeResource = normalizeValue(status?.activeResource)
  const activeStreamName = normalizeValue(status?.activeStreamName)
  const activeStream = normalizeValue(status?.activeStream)
  const metadata =
    entry.metadata && typeof entry.metadata === 'object' ? entry.metadata : {}

  if (normalizedEquals(entry.activeResource, activeResource)) {
    score += 12
  }
  if (normalizedEquals(entry.channelName, activeStreamName)) {
    score += 10
  }
  if (
    normalizedEquals(entry.filename, activeStreamName) ||
    normalizedEquals(entry.filename, activeStream)
  ) {
    score += 8
  }
  if (pickFirstValue(metadata, TITLE_PATHS)) {
    score += 2
  }
  if (pickFirstValue(metadata, ARTIST_PATHS)) {
    score += 1
  }

  return score
}

const selectMetadata = (streamMetadata, status) => {
  if (!Array.isArray(streamMetadata)) {
    return {}
  }

  return (
    streamMetadata.reduce(
      (best, entry) => {
        const score = scoreMetadataEntry(entry, status)
        return score > best.score ? { entry, score } : best
      },
      { entry: null, score: -1 },
    ).entry || {}
  )
}

const filenameDetails = (streamName) => {
  const normalized = normalizeValue(streamName).replace(/\\/g, '/')
  const filename =
    normalized
      .split('/')
      .pop()
      ?.replace(/\.[a-z0-9]{2,5}$/i, '') || ''
  const separatorIndex = filename.indexOf(' - ')

  if (separatorIndex <= 0) {
    return { title: filename, artist: '' }
  }

  return {
    artist: filename.slice(0, separatorIndex).trim(),
    title: filename.slice(separatorIndex + 3).trim(),
  }
}

export const getDeviceFromLocation = (locationLike) => {
  const pathname = normalizeValue(locationLike?.pathname)
  const playerPathMarker = '/app/player/'
  const playerPathIndex = pathname.indexOf(playerPathMarker)
  if (playerPathIndex >= 0) {
    const encodedDevice = pathname
      .slice(playerPathIndex + playerPathMarker.length)
      .split('/')[0]
    if (encodedDevice) {
      try {
        return normalizeValue(decodeURIComponent(encodedDevice))
      } catch {
        return normalizeValue(encodedDevice)
      }
    }
  }
  return ''
}

export const getNavidromeBasePath = (pathname) => {
  const normalizedPath = normalizeValue(pathname)
  const appIndex = normalizedPath.indexOf('/app/')
  if (appIndex >= 0) {
    return normalizedPath.slice(0, appIndex)
  }
  if (normalizedPath.endsWith('/app')) {
    return normalizedPath.slice(0, -4)
  }
  return ''
}

export const buildRetailStatusUrl = (basePath, device) => {
  const normalizedBasePath = normalizeValue(basePath).replace(/\/$/, '')
  return `${normalizedBasePath}/api/retailplayer/devices/${encodeURIComponent(
    normalizeValue(device),
  )}/status`
}

export const getNowPlaying = (payload) => {
  if (!payload || typeof payload !== 'object') {
    return null
  }

  const status =
    payload.status && typeof payload.status === 'object' ? payload.status : {}
  const artwork =
    payload.artwork && typeof payload.artwork === 'object'
      ? payload.artwork
      : {}
  const selectedEntry = selectMetadata(payload.streamMetadata, status)
  const entryMetadata =
    selectedEntry.metadata && typeof selectedEntry.metadata === 'object'
      ? selectedEntry.metadata
      : {}
  const combinedMetadata = { ...selectedEntry, ...entryMetadata }
  const streamName =
    normalizeValue(status.activeStreamName) ||
    normalizeValue(status.activeStream)
  const parsedFilename = filenameDetails(streamName)
  const title =
    pickFirstValue(combinedMetadata, TITLE_PATHS) ||
    pickFirstValue(status, TITLE_PATHS) ||
    parsedFilename.title ||
    streamName
  const artist =
    pickFirstValue(combinedMetadata, ARTIST_PATHS) ||
    pickFirstValue(status, ARTIST_PATHS) ||
    parsedFilename.artist
  const streamUrl = normalizeValue(artwork.streamUrl)

  if (!streamUrl || !title) {
    return null
  }

  const signature =
    normalizeValue(artwork.mediaFileId) ||
    normalizeValue(status.activeResource) ||
    `${title}\u0000${artist}`

  return {
    artist,
    signature,
    streamUrl,
    title,
  }
}

export const calculateSpectrumMetrics = (frequencyData) => {
  if (!frequencyData || frequencyData.length === 0) {
    return { bass: 0, centroid: 0, energy: 0 }
  }

  const bassBins = Math.max(1, Math.round(frequencyData.length * 0.12))
  let bassTotal = 0
  let magnitudeTotal = 0
  let weightedTotal = 0
  let squareTotal = 0

  for (let index = 0; index < frequencyData.length; index += 1) {
    const magnitude = frequencyData[index] / 255
    magnitudeTotal += magnitude
    weightedTotal += magnitude * index
    squareTotal += magnitude * magnitude
    if (index < bassBins) {
      bassTotal += magnitude
    }
  }

  return {
    bass: bassTotal / bassBins,
    centroid:
      magnitudeTotal > 0
        ? weightedTotal / magnitudeTotal / Math.max(1, frequencyData.length - 1)
        : 0,
    energy: Math.sqrt(squareTotal / frequencyData.length),
  }
}
