import { useCallback, useEffect, useMemo, useState } from 'react'
import { useDataProvider } from 'react-admin'
import subsonic from '../subsonic'
import httpClient from '../dataProvider/httpClient'
import { baseUrl } from '../utils'
import config from '../config'
import RetailPlayerMockService from './RetailPlayerMockService'
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

const mapResponseDevice = (device) => {
  if (!device || typeof device !== 'object') {
    return null
  }

  const rawId = normalizeValue(device.id)
  const fallbackId =
    rawId || normalizeValue(device.macAddress) || normalizeValue(device.ordinal)

  const resolvedId = fallbackId || normalizeValue(device.name)

  if (!resolvedId) {
    return null
  }

  const slugSource = buildDeviceSlug({ ...device, id: resolvedId }) || resolvedId
  const name = normalizeValue(device.name) || resolvedId

  return {
    id: resolvedId,
    apiId: rawId || resolvedId,
    name,
    slug: slugSource,
    slugKey: deviceSlugKey(slugSource),
    channel: normalizeValue(device.channel),
    channelList: normalizeValue(device.channelList),
    organization: normalizeValue(device.organization),
    timeZone: normalizeValue(device.timeZone),
  }
}

const selectDeviceBySlug = (devices, slugKey) => {
  if (!Array.isArray(devices) || !devices.length) {
    return null
  }

  if (!slugKey) {
    return devices[0]
  }

  return (
    devices.find((device) => device.slugKey === slugKey) ||
    devices.find((device) => deviceSlugKey(device.slug || device.name) === slugKey) ||
    devices.find((device) => deviceSlugKey(device.apiId) === slugKey) ||
    devices.find((device) => deviceSlugKey(device.id) === slugKey) ||
    null
  )
}

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
  const isApiEnabled = Boolean(config.retailPlayerDevicesEnabled)

  const deviceIdentifier = useMemo(() => {
    if (!slugParam) {
      return ''
    }
    try {
      return decodeURIComponent(slugParam)
    } catch (err) {
      return slugParam
    }
  }, [slugParam])

  const normalizedSlugKey = useMemo(() => deviceSlugKey(deviceIdentifier), [deviceIdentifier])

  const [baseDevice, setBaseDevice] = useState(() => {
    if (!isApiEnabled) {
      const devices = RetailPlayerMockService.listDevices()
      return selectDeviceBySlug(devices, normalizedSlugKey)
    }
    return null
  })
  const [devicesError, setDevicesError] = useState(null)
  const [devicesLoading, setDevicesLoading] = useState(false)

  const [statusState, setStatusState] = useState(initialStatusState)
  const [refreshIndex, setRefreshIndex] = useState(0)
  const [channelState, setChannelState] = useState(initialChannelState)

  const refresh = useCallback(() => {
    setRefreshIndex((previous) => previous + 1)
  }, [])

  useEffect(() => {
    if (!isApiEnabled) {
      setDevicesLoading(false)
      setDevicesError(null)

      const devices = RetailPlayerMockService.listDevices()
      const selectedDevice = selectDeviceBySlug(devices, normalizedSlugKey)
      setBaseDevice(selectedDevice || null)

      if (!selectedDevice) {
        setStatusState(initialStatusState)
        setChannelState(initialChannelState)
        return undefined
      }

      const mockDevice = RetailPlayerMockService.getDevice(selectedDevice.apiId)
      if (!mockDevice) {
        setStatusState(initialStatusState)
        setChannelState(initialChannelState)
        return undefined
      }

      const resolvedChannel =
        normalizeValue(mockDevice.channel) || normalizeValue(selectedDevice.channel)

      const statusPayload = {
        status: {
          channel: resolvedChannel,
          activeStreamName: resolvedChannel,
          volume: mockDevice.volume,
          isMuted: mockDevice.isMuted,
          isConnected: mockDevice.isConnected,
          hasSignal: mockDevice.hasSignal,
        },
        streamMetadata: [],
      }

      const channelPayload = Array.isArray(mockDevice.schedules)
        ? mockDevice.schedules
            .map((schedule) => {
              if (!schedule || typeof schedule !== 'object') {
                return null
              }
              const id = normalizeValue(schedule.key) || normalizeValue(schedule.label)
              const name = normalizeValue(schedule.label) || normalizeValue(schedule.key)
              if (!id && !name) {
                return null
              }
              return { id: id || name, name: name || id }
            })
            .filter(Boolean)
        : []

      setStatusState({
        data: statusPayload,
        error: null,
        isLoading: false,
        fetchedAt: new Date(),
      })
      setChannelState({
        data: channelPayload,
        error: null,
        isLoading: false,
        fetchedAt: new Date(),
      })

      return undefined
    }

    const identifier = normalizeValue(deviceIdentifier)
    if (!identifier) {
      setBaseDevice(null)
      setStatusState(initialStatusState)
      setChannelState(initialChannelState)
      setDevicesLoading(false)
      setDevicesError(null)
      return undefined
    }

    const url = buildStatusUrl(identifier)
    if (!url) {
      setBaseDevice(null)
      setStatusState(initialStatusState)
      setChannelState(initialChannelState)
      setDevicesLoading(false)
      setDevicesError(null)
      return undefined
    }

    const abortController = new AbortController()
    setDevicesLoading(true)
    setDevicesError(null)
    setChannelState({ data: [], error: null, isLoading: true, fetchedAt: null })
    setStatusState((previous) => ({ ...previous, isLoading: true, error: null }))

    httpClient(url, { signal: abortController.signal })
      .then(({ json }) => {
        if (abortController.signal.aborted) {
          return
        }

        const mappedDevice = mapResponseDevice(json?.device)
        const fallbackDevice =
          mappedDevice ||
          mapResponseDevice({
            id: identifier,
            name: identifier,
            channel: '',
            channelList: '',
            organization: '',
            timeZone: '',
          })

        setBaseDevice(fallbackDevice)
        setStatusState({
          data: json,
          error: null,
          isLoading: false,
          fetchedAt: new Date(),
        })
        setDevicesLoading(false)
        setDevicesError(null)
      })
      .catch((err) => {
        if (abortController.signal.aborted) {
          return
        }

        setBaseDevice(null)
        setChannelState(initialChannelState)
        setStatusState({
          data: null,
          error: err,
          isLoading: false,
          fetchedAt: new Date(),
        })
        setDevicesLoading(false)
        if (err?.status && err.status !== 404) {
          setDevicesError(err)
        } else {
          setDevicesError(null)
        }
      })

    return () => {
      abortController.abort()
      setDevicesLoading(false)
    }
  }, [deviceIdentifier, isApiEnabled, normalizedSlugKey, refreshIndex])

  useEffect(() => {
    if (!isApiEnabled) {
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
  }, [baseDevice?.channelList, devicesLoading, isApiEnabled])

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
