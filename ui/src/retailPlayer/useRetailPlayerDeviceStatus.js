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

const mapStatusPayloadToDevice = (baseDevice, payload) => {
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

  const normalizedSchedules = streamMetadata
    .map((item, index) => {
      const metadata = item.metadata && typeof item.metadata === 'object' ? item.metadata : {}
      const keySource =
        normalizeValue(item.channelName) ||
        normalizeValue(item.activeResource) ||
        normalizeValue(item.filename) ||
        `channel-${index + 1}`
      const key = keySource || `channel-${index + 1}`
      const label =
        normalizeValue(item.channelName) ||
        normalizeValue(metadata.title) ||
        normalizeValue(item.filename) ||
        `Channel ${index + 1}`
      const artist =
        normalizeValue(metadata.artist) ||
        normalizeValue(baseDevice.channel) ||
        normalizeValue(baseDevice.organization) ||
        baseDevice.name
      const volume = parseVolume(item.volume)

      return {
        key,
        label: label || key,
        artist: artist || baseDevice.name,
        isActive: false,
        metadata: {
          ...metadata,
          channelName: normalizeValue(item.channelName),
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
  const streamName = activeStreamName || normalizeValue(status.activeStream)

  const schedulesWithActive = normalizedSchedules.map((schedule, index) => {
    const matchesResource =
      activeResource && normalizeValue(schedule.metadata?.activeResource) === activeResource
    const matchesChannel =
      activeStreamName && normalizeValue(schedule.metadata?.channelName) === activeStreamName
    return {
      ...schedule,
      isActive:
        matchesResource || matchesChannel || (!activeResource && !activeStreamName && index === 0),
    }
  })

  const schedules = schedulesWithActive.length
    ? schedulesWithActive
    : [
        {
          key: buildDeviceSlug(baseDevice) || baseDevice.id,
          label: normalizeValue(baseDevice.channel) || baseDevice.name,
          artist: normalizeValue(baseDevice.organization) || baseDevice.name,
          isActive: true,
          metadata: {},
        },
      ]

  const activeSchedule = schedules.find((schedule) => schedule.isActive) || schedules[0]

  const metadata = activeSchedule?.metadata || {}
  const nowPlayingTitle =
    normalizeValue(metadata.title) ||
    activeSchedule?.label ||
    normalizeValue(status.activeStreamName) ||
    normalizeValue(status.activeStream) ||
    baseDevice.name
  const nowPlayingArtist =
    normalizeValue(metadata.artist) ||
    activeSchedule?.artist ||
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

  const normalizedDevice = useMemo(
    () => mapStatusPayloadToDevice(baseDevice, statusState.data),
    [baseDevice, statusState.data],
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
  const nowPlayingTitle = normalizeValue(nowPlaying.title)
  const nowPlayingArtist = normalizeValue(nowPlaying.artist)
  const existingArtwork = normalizeValue(nowPlaying.artworkUrl)
  const streamName = normalizeValue(nowPlaying.streamName)
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

    if (!streamName && !nowPlayingTitle && !metadataTitle) {
      setArtworkUrl(null)
      return undefined
    }

    let isCancelled = false

    const sanitizedStream = streamName ? streamName.replace(/\\/g, '/') : ''
    const fileName = sanitizedStream ? sanitizedStream.split('/').pop() : ''
    const baseWithoutExt = fileName ? fileName.replace(/\.[^/.]+$/, '') : ''

    let derivedTitle = metadataTitle || ''
    let derivedArtist = metadataArtist || ''

    if (!derivedTitle && baseWithoutExt) {
      if (baseWithoutExt.includes(' - ')) {
        const parts = baseWithoutExt.split(' - ')
        derivedArtist = derivedArtist || parts.shift()?.trim() || ''
        derivedTitle = parts.join(' - ').trim()
      } else {
        derivedTitle = baseWithoutExt.trim()
      }
    }

    if (!derivedTitle && nowPlayingTitle) {
      derivedTitle = nowPlayingTitle
    }

    if (!derivedArtist && nowPlayingArtist) {
      derivedArtist = nowPlayingArtist
    }

    const filter = {}
    if (derivedTitle) {
      filter.title = derivedTitle
    } else if (streamName) {
      filter.title = streamName
    }

    if (derivedArtist) {
      filter.artist = derivedArtist
    }

    if (!Object.keys(filter).length) {
      setArtworkUrl(null)
      return undefined
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
    nowPlayingArtist,
    nowPlayingTitle,
    normalizedDevice,
    streamName,
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
  const error = statusError || devicesError || null

  return {
    device: deviceWithArtwork,
    baseDevice,
    refresh,
    isLoading: Boolean(devicesLoading || statusState.isLoading),
    isDeviceListLoading: devicesLoading,
    isStatusLoading: statusState.isLoading,
    error,
    statusError: statusState.error,
    devicesError,
    notFound,
    isApiEnabled,
    lastUpdated: statusState.fetchedAt,
  }
}

export default useRetailPlayerDeviceStatus
