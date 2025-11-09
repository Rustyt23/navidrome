import { useCallback, useEffect, useMemo, useState } from 'react'
import config from '../config'
import httpClient from '../dataProvider/httpClient'
import {
  buildDeviceSlug,
  deviceSlugKey,
  mapRetailPlayerDevice,
  normalizeValue,
} from './deviceUtils'

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

const extractErrorMessage = (error) => {
  if (!error) {
    return ''
  }

  const body = error.body
  if (typeof body === 'string') {
    return normalizeValue(body)
  }

  if (body && typeof body === 'object') {
    const candidates = [body.message, body.error, body.detail, body.title]
    const found = candidates.find((candidate) => normalizeValue(candidate))
    if (found) {
      return normalizeValue(found)
    }
  }

  return normalizeValue(error.message)
}

const buildRetailPlayerCoverArtPath = ({
  artworkId,
  title,
  artist,
  size,
  square,
}) => {
  const params = new URLSearchParams()
  if (artworkId) {
    params.set('artworkId', artworkId)
  }
  if (!artworkId) {
    if (title) {
      params.set('title', title)
    }
    if (artist) {
      params.set('artist', artist)
    }
  }
  if (typeof size === 'number' && Number.isFinite(size) && size > 0) {
    params.set('size', String(Math.round(size)))
  }
  if (square) {
    params.set('square', 'true')
  }
  const query = params.toString()
  return query ? `/api/retailplayer/cover-art?${query}` : '/api/retailplayer/cover-art'
}

const isIntegrationDisabledError = (error) => {
  if (!error || typeof error.status !== 'number') {
    return false
  }

  if (error.status !== 404) {
    return false
  }

  const message = extractErrorMessage(error)
  if (!message) {
    return false
  }

  return /integration disabled/i.test(message)
}

const parseActiveStreamInfo = (value) => {
  const normalized = normalizeValue(value)
  if (!normalized) {
    return { title: '', artist: '' }
  }

  const withoutExtension = normalized.replace(/\.[^./\\]+$/, '')
  const segments = withoutExtension.split(/\s*[-–—]\s*/)

  if (segments.length >= 2) {
    const [first, ...rest] = segments
    const artist = normalizeValue(first)
    const title = normalizeValue(rest.join(' - '))
    return {
      title: title || normalizeValue(withoutExtension),
      artist,
    }
  }

  return { title: normalizeValue(withoutExtension) || normalized, artist: '' }
}

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
    return current[segment]
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

const mapChannelListResponse = (payload) =>
  ensureArray(payload?.channels)
    .map((item) => {
      if (!item || typeof item !== 'object') {
        return null
      }

      const id = normalizeValue(item.id)
      const name = normalizeValue(item.name)

      if (!id && !name) {
        return null
      }

      return {
        id,
        name: name || id,
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

      const channelId = normalizeValue(channel.id)
      const channelName = normalizeValue(channel.name)
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
      const channelName = normalizeValue(item.channelName)
      const keyCandidates = [
        channelName,
        normalizeValue(item.activeResource),
        normalizeValue(item.filename),
        `channel-${index + 1}`,
      ]
      const key = keyCandidates.find((candidate) => candidate) || `channel-${index + 1}`
      const labelCandidates = [
        channelName,
        normalizeValue(metadata.title),
        normalizeValue(item.filename),
        `Channel ${index + 1}`,
      ]
      const label = labelCandidates.find((candidate) => candidate) || key
      const artistCandidates = [
        normalizeValue(metadata.artist),
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
  const normalizedActiveResource = activeResource.toLowerCase()
  const activeStreamName = normalizeValue(status.activeStreamName)
  const activeStream = normalizeValue(status.activeStream)
  const streamName = activeStreamName || activeStream

  const metadataCandidate =
    streamMetadata.find((item) => {
      if (!item || typeof item !== 'object') {
        return false
      }

      const metadata = item.metadata && typeof item.metadata === 'object' ? item.metadata : {}
      const hasTitle = normalizeValue(metadata.title)
      const hasArtist = normalizeValue(metadata.artist)
      if (!hasTitle && !hasArtist) {
        return false
      }

      const itemResource = normalizeValue(item.activeResource).toLowerCase()
      if (normalizedActiveResource && itemResource === normalizedActiveResource) {
        return true
      }

      const itemChannelName = normalizeValue(item.channelName)
      if (activeStreamName && itemChannelName && itemChannelName.toLowerCase() === activeStreamName.toLowerCase()) {
        return true
      }

      const itemFilename = normalizeValue(item.filename)
      if (streamName && itemFilename && itemFilename.toLowerCase() === streamName.toLowerCase()) {
        return true
      }

      return true
    }) ||
    streamMetadata.find((item) => {
      if (!item || typeof item !== 'object') {
        return false
      }
      const metadata = item.metadata && typeof item.metadata === 'object' ? item.metadata : {}
      return Boolean(normalizeValue(metadata.title) || normalizeValue(metadata.artist))
    }) ||
    null

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

  const activeScheduleMetadata =
    activeSchedule && activeSchedule.metadata && typeof activeSchedule.metadata === 'object'
      ? activeSchedule.metadata
      : {}
  const streamMetadataDetails =
    metadataCandidate && metadataCandidate.metadata && typeof metadataCandidate.metadata === 'object'
      ? metadataCandidate.metadata
      : {}

  const combinedMetadata = { ...activeScheduleMetadata }
  Object.entries(streamMetadataDetails).forEach(([key, value]) => {
    if (value === undefined || value === null) {
      return
    }

    if (typeof value === 'object') {
      combinedMetadata[key] = value
      return
    }

    const normalized = normalizeValue(value)
    if (normalized) {
      combinedMetadata[key] = normalized
    } else if (!(key in combinedMetadata)) {
      combinedMetadata[key] = value
    }
  })

  const metadataTitle = pickFirstStringValue(combinedMetadata, [
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
  const statusTitle = pickFirstStringValue(status, [
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
  const scheduleLabel = normalizeValue(activeSchedule?.label)

  const metadataArtist = pickFirstStringValue(combinedMetadata, [
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
  const statusArtist = pickFirstStringValue(status, [
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
  const scheduleArtist = normalizeValue(activeSchedule?.artist)
  const { title: parsedStreamTitle, artist: parsedStreamArtist } = parseActiveStreamInfo(streamName)

  let nowPlayingTitle =
    metadataTitle ||
    statusTitle ||
    parsedStreamTitle ||
    scheduleLabel ||
    streamName ||
    baseDevice.name

  let nowPlayingArtist =
    metadataArtist ||
    statusArtist ||
    parsedStreamArtist ||
    scheduleArtist ||
    normalizeValue(baseDevice.channel) ||
    'Retail Player'

  const metadataArtworkUrl = normalizeValue(combinedMetadata.artworkUrl)
  const metadataAlbum = normalizeValue(combinedMetadata.album)

  const isLoadingNowPlaying =
    normalizedActiveResource === 'none' && !streamName && !metadataTitle && !metadataArtist

  if (isLoadingNowPlaying) {
    nowPlayingTitle = 'Loading'
    nowPlayingArtist = ''
  }

  const volume = combinedMetadata.volume ?? parseVolume(status.volume)

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
      title: nowPlayingTitle || (isLoadingNowPlaying ? 'Loading' : 'Now Playing'),
      artist: nowPlayingArtist || (isLoadingNowPlaying ? '' : 'Retail Player'),
      album: metadataAlbum || normalizeValue(activeScheduleMetadata.album),
      artworkUrl:
        metadataArtworkUrl ||
        normalizeValue(activeScheduleMetadata.artworkUrl) ||
        normalizeValue(streamMetadataDetails.artworkUrl),
      streamName,
      artworkId,
      mediaFileId,
      metadata: { ...combinedMetadata },
      isLoading: isLoadingNowPlaying,
    },
    status: normalizedStatus,
    streamMetadata,
    timeZone: deviceTimeZone,
    localTime,
  }
}

const initialDeviceState = {
  data: null,
  error: null,
  isLoading: false,
  fetchedAt: null,
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

  const [deviceState, setDeviceState] = useState(initialDeviceState)
  const [statusState, setStatusState] = useState(initialStatusState)
  const [refreshIndex, setRefreshIndex] = useState(0)
  const [channelState, setChannelState] = useState(initialChannelState)
  const [isApiEnabled, setIsApiEnabled] = useState(
    Boolean(config.retailPlayerDevicesEnabled),
  )

  const baseDevice = useMemo(() => {
    const ensureMatchingDevice = (device) => {
      if (!device) {
        return null
      }

      if (!normalizedSlugKey) {
        return device
      }

      const candidateKey =
        device.slugKey ||
        deviceSlugKey(device.slug || device.name || device.id)
      if (!candidateKey) {
        return null
      }

      return candidateKey === normalizedSlugKey ? device : null
    }

    const mappedDevice = ensureMatchingDevice(deviceState.data)
    if (mappedDevice) {
      return mappedDevice
    }

    const payloadDevice = statusState.data?.device
    if (payloadDevice) {
      return ensureMatchingDevice(mapRetailPlayerDevice(payloadDevice))
    }

    return null
  }, [deviceState.data, normalizedSlugKey, statusState.data])

  const refresh = useCallback(() => {
    setRefreshIndex((previous) => previous + 1)
  }, [])

  useEffect(() => {
    if (!slugParam) {
      setDeviceState(initialDeviceState)
      setStatusState(initialStatusState)
      return undefined
    }

    const url = buildStatusUrl(slugParam)
    if (!url) {
      setDeviceState(initialDeviceState)
      setStatusState(initialStatusState)
      return undefined
    }

    const abortController = new AbortController()
    setDeviceState((previous) => ({ ...previous, isLoading: true, error: null }))
    setStatusState((previous) => ({ ...previous, isLoading: true, error: null }))

    httpClient(url, { signal: abortController.signal })
      .then(({ json }) => {
        if (abortController.signal.aborted) {
          return
        }

        const mappedDevice = mapRetailPlayerDevice(json?.device) || null

        setIsApiEnabled(true)
        setDeviceState({
          data: mappedDevice,
          error: null,
          isLoading: false,
          fetchedAt: new Date(),
        })
        setStatusState({
          data: json,
          error: null,
          isLoading: false,
          fetchedAt: new Date(),
        })
      })
      .catch((err) => {
        if (abortController.signal.aborted) {
          return
        }

        if (isIntegrationDisabledError(err)) {
          setIsApiEnabled(false)
          setDeviceState(initialDeviceState)
          setStatusState(initialStatusState)
          return
        }

        setIsApiEnabled(true)
        const nextState = {
          data: null,
          error: err,
          isLoading: false,
          fetchedAt: new Date(),
        }
        setDeviceState(nextState)
        setStatusState(nextState)
      })

    return () => {
      abortController.abort()
    }
  }, [slugParam, refreshIndex])

  useEffect(() => {
    if (!isApiEnabled) {
      setChannelState(initialChannelState)
      return undefined
    }

    if (deviceState.isLoading) {
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
  }, [baseDevice?.channelList, deviceState.isLoading, isApiEnabled])

  const normalizedDevice = useMemo(
    () => mapStatusPayloadToDevice(baseDevice, statusState.data, channelState.data),
    [baseDevice, channelState.data, statusState.data],
  )

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

    const coverArtParams = {
      size: 300,
      square: true,
    }

    if (backendArtworkId) {
      coverArtParams.artworkId = backendArtworkId
    } else {
      if (!statusState.data) {
        setArtworkUrl(null)
        return undefined
      }

      if (!metadataTitle) {
        setArtworkUrl(null)
        return undefined
      }

      coverArtParams.title = metadataTitle
      if (metadataArtist) {
        coverArtParams.artist = metadataArtist
      }
    }

    const abortController = new AbortController()
    let isCancelled = false

    const fetchArtwork = async () => {
      try {
        const url = buildRetailPlayerCoverArtPath(coverArtParams)
        const response = await httpClient(url, { signal: abortController.signal })
        if (isCancelled || abortController.signal.aborted) {
          return
        }
        const imageUrl = normalizeValue(response?.json?.url)
        setArtworkUrl(imageUrl || null)
      } catch (err) {
        if (isCancelled || abortController.signal.aborted) {
          return
        }
        setArtworkUrl(null)
      }
    }

    fetchArtwork()

    return () => {
      isCancelled = true
      abortController.abort()
    }
  }, [
    artworkSignature,
    backendArtworkId,
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

  const rawDevicesError = deviceState.error
  const devicesError =
    rawDevicesError && rawDevicesError.status === 404 ? null : rawDevicesError

  const notFound =
    Boolean(normalizedSlugKey) &&
    isApiEnabled &&
    !deviceState.isLoading &&
    (!baseDevice || (statusState.error && statusState.error.status === 404))

  const statusError =
    statusState.error && statusState.error.status !== 404
      ? statusState.error
      : null
  const error = statusError || devicesError || channelState.error || null

  return {
    device: deviceWithArtwork,
    baseDevice,
    refresh,
    isLoading: Boolean(
      deviceState.isLoading || statusState.isLoading || channelState.isLoading,
    ),
    isDeviceListLoading: deviceState.isLoading,
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
