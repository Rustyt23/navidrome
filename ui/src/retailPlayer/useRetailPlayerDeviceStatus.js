import { useCallback, useEffect, useMemo, useState } from 'react'
import { useDataProvider } from 'react-admin'
import subsonic from '../subsonic'
import httpClient from '../dataProvider/httpClient'
import { baseUrl } from '../utils'
import useRetailPlayerDevices from './useRetailPlayerDevices'
import { buildDeviceSlug, deviceSlugKey, normalizeValue } from './deviceUtils'

const buildStatusUrl = (deviceId) =>
  deviceId ? `/api/retailplayer/devices/${encodeURIComponent(deviceId)}/status` : null

const clamp = (value, min, max) => Math.min(Math.max(value, min), max)

const parseVolume = (value) => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return clamp(Math.round(value), 0, 100)
  }
  if (typeof value === 'string') {
    const parsed = Number.parseFloat(value)
    if (Number.isFinite(parsed)) {
      return clamp(Math.round(parsed), 0, 100)
    }
  }
  return null
}

const ensureArray = (value) => (Array.isArray(value) ? value : [])

const readNestedValue = (source, path) => {
  if (!source || typeof source !== 'object' || !path) {
    return undefined
  }

  const segments = String(path)
    .split('.')
    .map((segment) => segment.trim())
    .filter(Boolean)

  if (!segments.length) {
    return undefined
  }

  return segments.reduce((current, segment) => {
    if (!current || typeof current !== 'object') {
      return undefined
    }

    if (Object.prototype.hasOwnProperty.call(current, segment)) {
      return current[segment]
    }

    const segmentLower = segment.toLowerCase()
    const matchingKey = Object.keys(current).find(
      (key) => String(key).toLowerCase() === segmentLower,
    )

    if (matchingKey) {
      return current[matchingKey]
    }

    return undefined
  }, source)
}

const pickFirstStringValue = (source, candidates) => {
  if (!source || typeof source !== 'object' || !Array.isArray(candidates)) {
    return ''
  }

  for (let index = 0; index < candidates.length; index += 1) {
    const candidate = candidates[index]
    if (!candidate) {
      continue
    }

    const rawValue = readNestedValue(source, candidate)
    const normalized = normalizeValue(rawValue)
    if (normalized) {
      return normalized
    }
  }

  return ''
}

const parseStreamArtistTitle = (value) => {
  const normalized = normalizeValue(value)
  if (!normalized) {
    return { artist: '', title: '' }
  }

  const parts = normalized
    .split('-')
    .map((part) => part.trim())
    .filter(Boolean)

  if (parts.length < 2) {
    return { artist: '', title: '' }
  }

  const [artist, ...titleParts] = parts
  const title = titleParts.join(' - ').trim()

  return {
    artist,
    title,
  }
}

const parseNowPlayingString = (value, channelCandidates, artistCandidates) => {
  const normalized = normalizeValue(value)
  if (!normalized) {
    return { title: '', artist: '' }
  }

  const channelSet = new Set(
    ensureArray(channelCandidates)
      .map((candidate) => normalizeValue(candidate))
      .filter(Boolean)
      .map((candidate) => candidate.toLowerCase()),
  )

  const artistSet = new Set(
    ensureArray(artistCandidates)
      .map((candidate) => normalizeValue(candidate))
      .filter(Boolean)
      .map((candidate) => candidate.toLowerCase()),
  )

  const separators = ['|', '-', '–', '—', '·', '/', '•']

  const evaluatePairs = (parts, separator) => {
    if (!Array.isArray(parts) || parts.length < 2) {
      return null
    }

    const trimmedParts = parts.map((part) => normalizeValue(part)).filter(Boolean)
    if (trimmedParts.length < 2) {
      return null
    }

    const joinedTail = trimmedParts.slice(1).join(separator === '|' ? '|' : ` ${separator} `)
    const joinedHead = trimmedParts
      .slice(0, trimmedParts.length - 1)
      .join(separator === '|' ? '|' : ` ${separator} `)

    const candidatePairs = []
    candidatePairs.push({ title: trimmedParts[0], artist: joinedTail })
    if (trimmedParts.length === 2) {
      candidatePairs.push({ title: trimmedParts[1], artist: trimmedParts[0] })
    } else if (trimmedParts.length > 2) {
      candidatePairs.push({ title: trimmedParts[trimmedParts.length - 1], artist: joinedHead })
    }

    for (let index = 0; index < candidatePairs.length; index += 1) {
      const pair = candidatePairs[index]
      const titleLower = normalizeValue(pair.title).toLowerCase()
      const artistLower = normalizeValue(pair.artist).toLowerCase()
      const titleMatchesChannel = channelSet.has(titleLower)
      const artistMatchesChannel = channelSet.has(artistLower)
      const artistMatchesKnown = artistSet.has(artistLower)
      const titleMatchesKnownArtist = artistSet.has(titleLower)

      if (titleMatchesChannel && !artistMatchesChannel) {
        continue
      }

      if (titleMatchesKnownArtist && !artistMatchesKnown) {
        continue
      }

      return pair
    }

    const fallbackPair = candidatePairs.find((pair) => {
      const titleLower = normalizeValue(pair.title).toLowerCase()
      return !channelSet.has(titleLower)
    })

    return fallbackPair || null
  }

  for (let index = 0; index < separators.length; index += 1) {
    const separator = separators[index]
    if (!normalized.includes(separator)) {
      continue
    }

    const parts = normalized.split(separator)
    const evaluated = evaluatePairs(parts, separator)
    if (evaluated) {
      return evaluated
    }
  }

  return { title: '', artist: '' }
}

const sanitizeCandidate = (value, invalidCandidates) => {
  const normalized = normalizeValue(value)
  if (!normalized) {
    return ''
  }

  const invalidSet = new Set(
    ensureArray(invalidCandidates)
      .map((candidate) => normalizeValue(candidate))
      .filter(Boolean)
      .map((candidate) => candidate.toLowerCase()),
  )

  return invalidSet.has(normalized.toLowerCase()) ? '' : normalized
}

const mapChannelListResponse = (payload) =>
  ensureArray(payload?.channels)
    .map((item, index) => {
      if (!item || typeof item !== 'object') {
        return null
      }

      const id = pickFirstStringValue(item, ['id', 'channelId', 'channel_id'])
      const name = pickFirstStringValue(item, [
        'name',
        'channelName',
        'channel_name',
        'label',
        'title',
      ])

      if (!id && !name) {
        return null
      }

      const resolvedName = name || id || `Channel ${index + 1}`

      return {
        id: id || resolvedName,
        name: resolvedName,
      }
    })
    .filter(Boolean)

const mapStatusPayloadToDevice = (baseDevice, payload, channelList) => {
  if (!baseDevice) {
    return null
  }

  const status = payload && typeof payload === 'object' ? payload.status || {} : {}
  const streamMetadata = ensureArray(payload?.streamMetadata).filter(
    (item) => item && typeof item === 'object',
  )

  const payloadArtwork =
    payload && typeof payload === 'object' ? payload.artwork || {} : {}
  const artworkId = normalizeValue(payloadArtwork.artworkId)
  const mediaFileId = normalizeValue(payloadArtwork.mediaFileId)

  const defaultChannelArtist =
    normalizeValue(baseDevice.organization) || baseDevice.name

  const normalizedChannelList = ensureArray(channelList)
    .map((channel, index) => {
      if (!channel || typeof channel !== 'object') {
        return null
      }

      const channelId = pickFirstStringValue(channel, ['id', 'channelId', 'channel_id'])
      const channelName = pickFirstStringValue(channel, [
        'name',
        'channelName',
        'channel_name',
        'label',
        'title',
      ])
      if (!channelId && !channelName) {
        return null
      }

      const label = channelName || channelId || `Channel ${index + 1}`
      const keyCandidates = [
        channelId,
        deviceSlugKey(label),
        label,
        `channel-${index + 1}`,
      ]
      const key =
        keyCandidates.find((candidate) => normalizeValue(candidate)) ||
        `channel-${index + 1}`

      return {
        key,
        label,
        artist: defaultChannelArtist || baseDevice.name,
        isActive: false,
        metadata: {
          channelId,
          channelName: channelName || label,
        },
        raw: channel,
      }
    })
    .filter(Boolean)

  const normalizedSchedules = streamMetadata
    .map((item, index) => {
      const metadata = item.metadata && typeof item.metadata === 'object' ? item.metadata : {}
      const channelName = pickFirstStringValue(item, [
        'channelName',
        'channel_name',
        'name',
      ])
      const keyCandidates = [
        channelName,
        pickFirstStringValue(item, ['activeResource', 'active_resource', 'resource']),
        pickFirstStringValue(item, ['filename', 'fileName', 'file_name']),
        `channel-${index + 1}`,
      ]
      const key = keyCandidates.find((candidate) => candidate) || `channel-${index + 1}`
      const labelCandidates = [
        channelName,
        pickFirstStringValue(metadata, ['title', 'label', 'name']),
        pickFirstStringValue(item, ['filename', 'fileName', 'file_name']),
        `Channel ${index + 1}`,
      ]
      const label = labelCandidates.find((candidate) => candidate) || key
      const artistCandidates = [
        pickFirstStringValue(metadata, [
          'artist',
          'performer',
          'artistName',
          'artist_name',
        ]),
        normalizeValue(baseDevice.channel),
        normalizeValue(baseDevice.organization),
        baseDevice.name,
      ]
      const artist = artistCandidates.find((candidate) => candidate) || baseDevice.name
      const volume = parseVolume(item.volume)

      return {
        key,
        label,
        artist,
        isActive: false,
        metadata: {
          ...metadata,
          channelName,
          activeResource: normalizeValue(item.activeResource),
          filename: normalizeValue(item.filename),
          volume,
        },
        raw: item,
      }
    })
    .filter(Boolean)

  const activeResource = normalizeValue(status.activeResource)
  const activeStreamName = normalizeValue(status.activeStreamName)
  const activeStream = normalizeValue(status.activeStream)
  const streamName = activeStreamName || activeStream
  const { artist: streamArtistFallback, title: streamTitleFallback } =
    parseStreamArtistTitle(activeStream || activeStreamName)

  const schedulesWithActive = normalizedSchedules.map((schedule, index) => {
    const metadata = schedule.metadata ? { ...schedule.metadata } : {}
    const matchesResource =
      activeResource && normalizeValue(metadata.activeResource) === activeResource
    const matchesChannel =
      activeStreamName && normalizeValue(metadata.channelName) === activeStreamName
    const isActive =
      matchesResource ||
      matchesChannel ||
      (!activeResource && !activeStreamName && index === 0)

    if (isActive) {
      const deviceChannelId = normalizeValue(baseDevice.channel)
      if (deviceChannelId && !metadata.channelId) {
        metadata.channelId = deviceChannelId
      }
    }

    return {
      ...schedule,
      metadata,
      isActive,
    }
  })

  const fallbackSchedule = {
    key: buildDeviceSlug(baseDevice) || baseDevice.id,
    label: normalizeValue(baseDevice.channel) || baseDevice.name,
    artist: normalizeValue(baseDevice.organization) || baseDevice.name,
    isActive: true,
    metadata: {
      channelId: normalizeValue(baseDevice.channel),
      channelName: normalizeValue(baseDevice.channel) || baseDevice.name,
    },
  }

  const metadataSchedules = schedulesWithActive.length
    ? schedulesWithActive
    : [fallbackSchedule]

  let schedules = metadataSchedules

  if (normalizedChannelList.length) {
    const metadataById = new Map()
    const metadataByName = new Map()

    metadataSchedules.forEach((schedule) => {
      const scheduleMetadata = schedule.metadata || {}
      const idKey = normalizeValue(scheduleMetadata.channelId)
      const nameKey = deviceSlugKey(
        normalizeValue(scheduleMetadata.channelName) || schedule.label || schedule.key,
      )
      if (idKey) {
        metadataById.set(idKey, schedule)
      }
      if (nameKey) {
        metadataByName.set(nameKey, schedule)
      }
    })

    const activeMetadata = metadataSchedules.find((schedule) => schedule.isActive) || null
    const activeId =
      normalizeValue(activeMetadata?.metadata?.channelId) || normalizeValue(baseDevice.channel)
    const activeNameKey = activeMetadata
      ? deviceSlugKey(
          normalizeValue(activeMetadata.metadata?.channelName) ||
            activeMetadata.label ||
            activeMetadata.key,
        )
      : ''

    let mergedSchedules = normalizedChannelList.map((schedule, index) => {
      const channelId = normalizeValue(schedule.metadata.channelId)
      const channelName = normalizeValue(schedule.metadata.channelName) || schedule.label
      const nameKey = deviceSlugKey(channelName)
      const metadataMatch =
        (channelId && metadataById.get(channelId)) ||
        (nameKey && metadataByName.get(nameKey)) ||
        null

      const baseMetadata = metadataMatch?.metadata || {}
      const mergedMetadata = { ...baseMetadata, ...schedule.metadata }
      if (!mergedMetadata.channelName) {
        mergedMetadata.channelName = channelName
      }
      if (!mergedMetadata.channelId && channelId) {
        mergedMetadata.channelId = channelId
      }

      const artist = metadataMatch?.artist || schedule.artist
      const isActive =
        Boolean(metadataMatch?.isActive) ||
        (activeId && channelId && channelId === activeId) ||
        (activeNameKey && nameKey && nameKey === activeNameKey)

      return {
        ...schedule,
        label: mergedMetadata.channelName || schedule.label,
        artist: artist || schedule.artist,
        metadata: mergedMetadata,
        isActive,
        raw: metadataMatch?.raw || schedule.raw,
      }
    })

    if (!mergedSchedules.some((schedule) => schedule.isActive) && mergedSchedules.length) {
      mergedSchedules = mergedSchedules.map((schedule, index) => ({
        ...schedule,
        isActive: index === 0,
      }))
    }

    const identifierSet = new Set()
    mergedSchedules.forEach((schedule) => {
      const idValue = normalizeValue(schedule.metadata?.channelId)
      const nameKey = deviceSlugKey(
        normalizeValue(schedule.metadata?.channelName) || schedule.label || schedule.key,
      )
      if (idValue) {
        identifierSet.add(`id:${idValue}`)
      }
      if (nameKey) {
        identifierSet.add(`name:${nameKey}`)
      }
    })

    metadataSchedules.forEach((schedule) => {
      const idValue = normalizeValue(schedule.metadata?.channelId)
      const nameKey = deviceSlugKey(
        normalizeValue(schedule.metadata?.channelName) || schedule.label || schedule.key,
      )
      const idToken = idValue ? `id:${idValue}` : ''
      const nameToken = nameKey ? `name:${nameKey}` : ''
      if ((idToken && identifierSet.has(idToken)) || (nameToken && identifierSet.has(nameToken))) {
        return
      }
      mergedSchedules.push(schedule)
    })

    schedules = mergedSchedules
  }

  const activeSchedule = schedules.find((schedule) => schedule.isActive) || schedules[0]

  const metadata = activeSchedule?.metadata || {}
  const metadataTitleRaw = pickFirstStringValue(metadata, [
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
  ])
  const statusTitleRaw = pickFirstStringValue(status, [
    'trackTitle',
    'track_title',
    'nowPlayingTitle',
    'now_playing_title',
    'currentTrackTitle',
    'current_track_title',
    'currentTitle',
    'current_title',
    'streamTitle',
    'stream_title',
    'title',
  ])
  const metadataArtistRaw = pickFirstStringValue(metadata, [
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
  ])
  const statusArtistRaw = pickFirstStringValue(status, [
    'trackArtist',
    'track_artist',
    'nowPlayingArtist',
    'now_playing_artist',
    'currentTrackArtist',
    'current_track_artist',
    'currentArtist',
    'current_artist',
    'artist',
  ])
  const scheduleLabel = normalizeValue(activeSchedule?.label)
  const scheduleArtist = normalizeValue(activeSchedule?.artist)

  const channelNameCandidate = pickFirstStringValue(metadata, [
    'channelName',
    'channel_name',
  ])
  const channelLabelCandidates = [
    channelNameCandidate,
    scheduleLabel,
    normalizeValue(baseDevice.channel),
    streamName,
    baseDevice.name,
  ].filter(Boolean)

  const combinedNowPlayingValues = [
    pickFirstStringValue(metadata, [
      'nowPlaying',
      'now_playing',
      'currentTrack',
      'current_track',
      'track',
      'streamTitle',
      'stream_title',
    ]),
    pickFirstStringValue(status, [
      'nowPlaying',
      'now_playing',
      'currentTrack',
      'current_track',
      'track',
      'streamTitle',
      'stream_title',
    ]),
  ]

  const knownArtistCandidates = [
    metadataArtistRaw,
    statusArtistRaw,
    scheduleArtist,
    streamArtistFallback,
    normalizeValue(baseDevice.organization),
    normalizeValue(baseDevice.channel),
  ].filter(Boolean)

  const parsedCombined =
    combinedNowPlayingValues
      .map((candidate) =>
        parseNowPlayingString(candidate, channelLabelCandidates, knownArtistCandidates),
      )
      .find((parsed) => parsed.title || parsed.artist) || { title: '', artist: '' }

  const useStreamTitleFallback =
    !metadataTitleRaw && !statusTitleRaw && Boolean(streamTitleFallback)

  const sanitizedMetadataTitle = sanitizeCandidate(
    metadataTitleRaw,
    channelLabelCandidates,
  )
  const sanitizedStatusTitle = sanitizeCandidate(statusTitleRaw, channelLabelCandidates)
  const sanitizedParsedTitle = sanitizeCandidate(parsedCombined.title, channelLabelCandidates)
  const sanitizedStreamFallback = sanitizeCandidate(
    useStreamTitleFallback ? streamTitleFallback : '',
    channelLabelCandidates,
  )

  const nowPlayingTitle =
    sanitizedMetadataTitle ||
    sanitizedStatusTitle ||
    sanitizedParsedTitle ||
    sanitizedStreamFallback ||
    scheduleLabel ||
    streamName ||
    baseDevice.name

  const useStreamArtistFallback =
    !metadataArtistRaw && !statusArtistRaw && Boolean(streamArtistFallback)

  const sanitizedParsedArtist = sanitizeCandidate(parsedCombined.artist, channelLabelCandidates)

  const nowPlayingArtist =
    metadataArtistRaw ||
    statusArtistRaw ||
    sanitizedParsedArtist ||
    (useStreamArtistFallback ? streamArtistFallback : '') ||
    scheduleArtist ||
    normalizeValue(baseDevice.channel) ||
    'Retail Player'

  const volume = metadata.volume ?? parseVolume(status.volume)

  const scheduleStatus = normalizeValue(status.scheduleStatus).toLowerCase()
  const isConnected =
    scheduleStatus === 'active' ||
    normalizeValue(status.ipAddress) !== '' ||
    normalizeValue(status.webSocketReconnect) !== ''
  const hasSignal =
    Boolean(normalizeValue(status.activeStream) || normalizeValue(status.activeStreamName)) ||
    Boolean(activeSchedule)

  const deviceTimeZone =
    normalizeValue(baseDevice.timeZone) || normalizeValue(status.timeZone)
  const localTime = typeof status.localTime === 'string' ? status.localTime : null

  const normalizedStatus = { ...status }
  if (deviceTimeZone && !normalizedStatus.timeZone) {
    normalizedStatus.timeZone = deviceTimeZone
  }
  if (localTime) {
    normalizedStatus.localTime = localTime
  }

  return {
    ...baseDevice,
    isConnected,
    hasSignal,
    isMuted: volume === 0,
    volume: Number.isFinite(volume) ? volume : 50,
    schedules,
    nowPlaying: {
      title: nowPlayingTitle || 'Now Playing',
      artist: nowPlayingArtist || 'Retail Player',
      album: normalizeValue(metadata.album),
      artworkUrl: normalizeValue(metadata.artworkUrl),
      streamName,
      artworkId,
      mediaFileId,
      metadata,
    },
    status: normalizedStatus,
    streamMetadata,
    timeZone: deviceTimeZone,
    localTime,
  }
}

const initialStatusState = {
  data: null,
  error: null,
  isLoading: false,
  fetchedAt: null,
}

const initialChannelState = {
  data: [],
  error: null,
  isLoading: false,
  fetchedAt: null,
}

const useRetailPlayerDeviceStatus = (slugParam) => {
  const {
    devices,
    error: devicesError,
    isLoading: devicesLoading,
    isApiEnabled,
  } = useRetailPlayerDevices()

  const normalizedSlugKey = useMemo(() => {
    if (!slugParam) {
      return ''
    }
    try {
      return deviceSlugKey(decodeURIComponent(slugParam))
    } catch (err) {
      return deviceSlugKey(slugParam)
    }
  }, [slugParam])

  const baseDevice = useMemo(() => {
    if (!devices.length) {
      return null
    }
    if (!normalizedSlugKey) {
      return devices[0]
    }

    return (
      devices.find((device) => {
        const deviceKey = device.slugKey || deviceSlugKey(device.slug || device.name || device.id)
        return deviceKey === normalizedSlugKey
      }) ||
      devices.find((device) => deviceSlugKey(device.name) === normalizedSlugKey) ||
      devices.find((device) => deviceSlugKey(device.apiId) === normalizedSlugKey) ||
      devices.find((device) => deviceSlugKey(device.id) === normalizedSlugKey) ||
      null
    )
  }, [devices, normalizedSlugKey])

  const [statusState, setStatusState] = useState(initialStatusState)
  const [refreshIndex, setRefreshIndex] = useState(0)
  const [channelState, setChannelState] = useState(initialChannelState)

  const refresh = useCallback(() => {
    setRefreshIndex((previous) => previous + 1)
  }, [])

  useEffect(() => {
    if (!isApiEnabled) {
      setStatusState(initialStatusState)
      return undefined
    }

    if (devicesLoading) {
      return undefined
    }

    const deviceId = normalizeValue(baseDevice?.apiId)
    if (!deviceId) {
      setStatusState(initialStatusState)
      return undefined
    }

    const url = buildStatusUrl(deviceId)
    if (!url) {
      setStatusState(initialStatusState)
      return undefined
    }

    const abortController = new AbortController()
    setStatusState((previous) => ({ ...previous, isLoading: true, error: null }))

    httpClient(url, { signal: abortController.signal })
      .then(({ json }) => {
        if (abortController.signal.aborted) {
          return
        }
        setStatusState({ data: json, error: null, isLoading: false, fetchedAt: new Date() })
      })
      .catch((err) => {
        if (abortController.signal.aborted) {
          return
        }
        setStatusState({ data: null, error: err, isLoading: false, fetchedAt: new Date() })
      })

    return () => {
      abortController.abort()
    }
  }, [baseDevice?.apiId, devicesLoading, isApiEnabled, refreshIndex])

  useEffect(() => {
    if (!isApiEnabled) {
      setChannelState(initialChannelState)
      return undefined
    }

    if (devicesLoading) {
      return undefined
    }

    const channelListId = normalizeValue(baseDevice?.channelList)
    if (!channelListId) {
      setChannelState(initialChannelState)
      return undefined
    }

    const url = `/api/retailplayer/channel-lists/${encodeURIComponent(channelListId)}/channels`
    const abortController = new AbortController()
    setChannelState((previous) => ({ ...previous, isLoading: true, error: null }))

    httpClient(url, { signal: abortController.signal })
      .then(({ json }) => {
        if (abortController.signal.aborted) {
          return
        }
        setChannelState({
          data: mapChannelListResponse(json),
          error: null,
          isLoading: false,
          fetchedAt: new Date(),
        })
      })
      .catch((err) => {
        if (abortController.signal.aborted) {
          return
        }
        setChannelState({
          data: [],
          error: err,
          isLoading: false,
          fetchedAt: new Date(),
        })
      })

    return () => {
      abortController.abort()
    }
  }, [baseDevice?.channelList, devicesLoading, isApiEnabled, refreshIndex])

  const normalizedDevice = useMemo(
    () => mapStatusPayloadToDevice(baseDevice, statusState.data, channelState.data),
    [baseDevice, channelState.data, statusState.data],
  )

  const dataProvider = useDataProvider()
  const [artworkUrl, setArtworkUrl] = useState(null)

  const artworkSignature = useMemo(() => {
    if (!normalizedDevice) {
      return ''
    }
    const nowPlaying = normalizedDevice.nowPlaying || {}
    return [
      normalizedDevice.id,
      normalizedDevice.slug,
      normalizeValue(nowPlaying.artworkId),
      normalizeValue(nowPlaying.streamName),
      normalizeValue(nowPlaying.title),
      normalizeValue(nowPlaying.artist),
    ].join('::')
  }, [normalizedDevice])

  const nowPlaying = normalizedDevice?.nowPlaying || {}
  const existingArtwork = normalizeValue(nowPlaying.artworkUrl)
  const backendArtworkId = normalizeValue(nowPlaying.artworkId)
  const nowPlayingMetadata = nowPlaying.metadata || {}
  const metadataTitle = normalizeValue(nowPlayingMetadata.title)
  const metadataArtist = normalizeValue(nowPlayingMetadata.artist)

  useEffect(() => {
    if (!normalizedDevice) {
      setArtworkUrl(null)
      return undefined
    }

    if (existingArtwork) {
      setArtworkUrl(existingArtwork)
      return undefined
    }

    if (backendArtworkId) {
      const coverArtPath = subsonic.url('getCoverArt', backendArtworkId, {
        size: 300,
        square: true,
      })
      setArtworkUrl(baseUrl(coverArtPath))
      return undefined
    }

    if (!statusState.data) {
      setArtworkUrl(null)
      return undefined
    }

    if (!metadataTitle) {
      setArtworkUrl(null)
      return undefined
    }

    let isCancelled = false
    const filter = { title: metadataTitle }
    if (metadataArtist) {
      filter.artist = metadataArtist
    }

    const fetchArtwork = async () => {
      try {
        const response = await dataProvider.getList('song', {
          pagination: { page: 1, perPage: 1 },
          sort: { field: 'id', order: 'ASC' },
          filter,
        })
        if (isCancelled) {
          return
        }
        const songs = Array.isArray(response?.data) ? response.data : []
        if (songs.length > 0) {
          setArtworkUrl(subsonic.getCoverArtUrl(songs[0], 300, true))
          return
        }
        setArtworkUrl(null)
      } catch (err) {
        if (!isCancelled) {
          setArtworkUrl(null)
        }
      }
    }

    fetchArtwork()

    return () => {
      isCancelled = true
    }
  }, [
    artworkSignature,
    backendArtworkId,
    dataProvider,
    existingArtwork,
    metadataArtist,
    metadataTitle,
    normalizedDevice,
    statusState.data,
  ])

  const deviceWithArtwork = useMemo(() => {
    if (!normalizedDevice) {
      return null
    }

    if (existingArtwork || !artworkUrl) {
      return normalizedDevice
    }

    return {
      ...normalizedDevice,
      nowPlaying: {
        ...normalizedDevice.nowPlaying,
        artworkUrl,
      },
    }
  }, [artworkUrl, existingArtwork, normalizedDevice])

  const notFound =
    Boolean(normalizedSlugKey) &&
    !devicesLoading &&
    (!baseDevice || (statusState.error && statusState.error.status === 404))

  const statusError = statusState.error && statusState.error.status !== 404 ? statusState.error : null
  const error = statusError || devicesError || channelState.error || null

  return {
    device: deviceWithArtwork,
    baseDevice,
    refresh,
    isLoading: Boolean(devicesLoading || statusState.isLoading || channelState.isLoading),
    isDeviceListLoading: devicesLoading,
    isStatusLoading: statusState.isLoading,
    isChannelListLoading: channelState.isLoading,
    error,
    statusError: statusState.error,
    devicesError,
    channelListError: channelState.error,
    notFound,
    isApiEnabled,
    lastUpdated: statusState.fetchedAt,
  }
}

export default useRetailPlayerDeviceStatus
