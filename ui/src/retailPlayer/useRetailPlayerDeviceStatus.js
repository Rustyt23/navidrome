import { useCallback, useEffect, useMemo, useState } from 'react'
import { useDataProvider } from 'react-admin'
import subsonic from '../subsonic'
import httpClient from '../dataProvider/httpClient'
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
      metadata,
    },
    status,
    streamMetadata,
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
    if (!baseDevice?.id) {
      setStatusState(initialStatusState)
      return undefined
    }

    const url = buildStatusUrl(baseDevice.id)
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
  }, [baseDevice?.id, refreshIndex])

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
    return [normalizedDevice.id, normalizedDevice.slug, normalizedDevice.nowPlaying?.title].join('::')
  }, [normalizedDevice])

  const nowPlayingTitle = normalizeValue(normalizedDevice?.nowPlaying?.title)
  const nowPlayingArtist = normalizeValue(normalizedDevice?.nowPlaying?.artist)
  const existingArtwork = normalizeValue(normalizedDevice?.nowPlaying?.artworkUrl)

  useEffect(() => {
    if (!normalizedDevice) {
      setArtworkUrl(null)
      return undefined
    }

    if (existingArtwork) {
      setArtworkUrl(existingArtwork)
      return undefined
    }

    if (!nowPlayingTitle) {
      setArtworkUrl(null)
      return undefined
    }

    let isCancelled = false
    const filters = nowPlayingArtist
      ? { title: nowPlayingTitle, artist: nowPlayingArtist }
      : { title: nowPlayingTitle }

    dataProvider
      .getList('song', {
        pagination: { page: 1, perPage: 1 },
        sort: { field: 'id', order: 'ASC' },
        filter: filters,
      })
      .then((response) => {
        if (isCancelled) {
          return
        }
        const songs = Array.isArray(response?.data) ? response.data : []
        if (songs.length > 0) {
          setArtworkUrl(subsonic.getCoverArtUrl(songs[0], 300, true))
        } else {
          setArtworkUrl(null)
        }
      })
      .catch(() => {
        if (!isCancelled) {
          setArtworkUrl(null)
        }
      })

    return () => {
      isCancelled = true
    }
  }, [artworkSignature, dataProvider, existingArtwork, nowPlayingArtist, nowPlayingTitle, normalizedDevice])

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
