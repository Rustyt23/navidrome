const DEFAULT_RETAIL_PLAYER_API_BASE_PATH = '/api/retailplayer'

const normalizeRetailPlayerBasePath = (rawBasePath) => {
  if (typeof rawBasePath !== 'string') {
    return DEFAULT_RETAIL_PLAYER_API_BASE_PATH
  }

  const trimmed = rawBasePath.trim()
  if (!trimmed) {
    return DEFAULT_RETAIL_PLAYER_API_BASE_PATH
  }

  const withoutTrailingSlash = trimmed.replace(/\/+$/, '')
  const withLeadingSlash =
    withoutTrailingSlash.startsWith('/') || withoutTrailingSlash === ''
      ? withoutTrailingSlash
      : `/${withoutTrailingSlash}`

  const normalized = withLeadingSlash || '/'

  if (normalized === '/') {
    return DEFAULT_RETAIL_PLAYER_API_BASE_PATH
  }

  return normalized
}

const cleanPathSegment = (segment) => {
  if (segment === undefined || segment === null) {
    return null
  }

  const stringValue = String(segment)
  const trimmed = stringValue.trim()
  if (!trimmed) {
    return null
  }

  return trimmed.replace(/^\/+/, '').replace(/\/+$/, '')
}

const buildRetailPlayerApiPath = (basePath, ...segments) => {
  const normalizedBasePath = normalizeRetailPlayerBasePath(basePath)
  const filteredSegments = segments
    .map((segment) => cleanPathSegment(segment))
    .filter(Boolean)

  if (!filteredSegments.length) {
    return normalizedBasePath
  }

  return `${normalizedBasePath}/${filteredSegments.join('/')}`
}

const buildRetailPlayerDevicePath = (basePath, deviceId, ...segments) => {
  if (deviceId === undefined || deviceId === null || deviceId === '') {
    return ''
  }

  return buildRetailPlayerApiPath(
    basePath,
    'devices',
    encodeURIComponent(String(deviceId)),
    ...segments,
  )
}

const buildRetailPlayerChannelListPath = (
  basePath,
  channelListId,
  ...segments
) => {
  if (channelListId === undefined || channelListId === null || channelListId === '') {
    return ''
  }

  return buildRetailPlayerApiPath(
    basePath,
    'channel-lists',
    encodeURIComponent(String(channelListId)),
    ...segments,
  )
}

export {
  DEFAULT_RETAIL_PLAYER_API_BASE_PATH,
  buildRetailPlayerApiPath,
  buildRetailPlayerChannelListPath,
  buildRetailPlayerDevicePath,
  normalizeRetailPlayerBasePath,
}
