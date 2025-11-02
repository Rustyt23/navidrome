import { useCallback, useEffect, useMemo, useState } from 'react'
import httpClient from '../dataProvider/httpClient'
import useRetailPlayerDevices from './useRetailPlayerDevices'
import { deviceSlugKey, normalizeValue } from './deviceUtils'

const ensureArray = (value) => (Array.isArray(value) ? value : [])

const parseVolume = (value) => {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.min(Math.max(Math.round(value), 0), 100)
  }
  if (typeof value === 'string') {
    const parsed = Number.parseFloat(value)
    if (Number.isFinite(parsed)) {
      return Math.min(Math.max(Math.round(parsed), 0), 100)
    }
  }
  return null
}

const mapChannelList = (channels) =>
  ensureArray(channels)
    .map((channel, index) => {
      if (!channel || typeof channel !== 'object') {
        return null
      }

      const id = normalizeValue(channel.id)
      const name = normalizeValue(channel.name)
      const fallbackName = name || id || `Channel ${index + 1}`

      return {
        key: deviceSlugKey(id || fallbackName) || `channel-${index + 1}`,
        label: fallbackName,
        channelId: id,
      }
    })
    .filter(Boolean)

const mapStreamMetadata = (streamMetadata) =>
  ensureArray(streamMetadata)
    .map((item, index) => {
      if (!item || typeof item !== 'object') {
        return null
      }

      const metadata =
        item.metadata && typeof item.metadata === 'object' ? item.metadata : {}
      const channelId =
        normalizeValue(metadata.channelId) ||
        normalizeValue(item.channelId) ||
        normalizeValue(item.id)
      const channelName =
        normalizeValue(item.channelName) || normalizeValue(metadata.channelName)
      const label = channelName || normalizeValue(metadata.title)

      return {
        key:
          deviceSlugKey(channelId || channelName || label) ||
          `channel-${index + 1}`,
        label: label || channelName || channelId || `Channel ${index + 1}`,
        artist: normalizeValue(metadata.artist),
        channelId: channelId || '',
        metadata,
        isActive: Boolean(item.isActive) || Boolean(metadata.isActive),
        volume: parseVolume(item.volume ?? metadata.volume),
      }
    })
    .filter(Boolean)

const pickFirst = (candidates) =>
  candidates.map((value) => normalizeValue(value)).find((value) => value)

const deriveActiveChannelId = (baseDevice, status, metadataSchedules) => {
  const statusMap = status && typeof status === 'object' ? status : {}
  const metadataActive = metadataSchedules.find((schedule) => schedule.isActive)

  return (
    pickFirst([
      statusMap.channelId,
      statusMap.channel,
      statusMap.activeChannel,
      statusMap.activeStream,
      statusMap.activeStreamName,
      metadataActive?.channelId,
      baseDevice?.channel,
    ]) || ''
  )
}

const buildSchedules = (baseDevice, channelList, streamMetadata, status) => {
  const metadataSchedules = mapStreamMetadata(streamMetadata)
  const metadataById = new Map()
  metadataSchedules.forEach((schedule) => {
    if (schedule.channelId) {
      metadataById.set(schedule.channelId, schedule)
    }
  })

  const listSchedules = channelList.length ? channelList : metadataSchedules
  const activeChannelId = deriveActiveChannelId(baseDevice, status, metadataSchedules)
  const fallbackArtist =
    normalizeValue(baseDevice?.organization) || baseDevice?.name || ''

  let schedules = listSchedules.map((schedule, index) => {
    const metadata = metadataById.get(schedule.channelId) || schedule.metadata || {}
    const label = normalizeValue(schedule.label) || `Channel ${index + 1}`
    const channelId = normalizeValue(schedule.channelId)
    const artist =
      normalizeValue(schedule.artist) ||
      normalizeValue(metadata.artist) ||
      fallbackArtist
    const key = deviceSlugKey(channelId || label) || `channel-${index + 1}`
    const isActive = channelId
      ? channelId === activeChannelId
      : key && activeChannelId && key === deviceSlugKey(activeChannelId)

    return {
      key,
      label,
      channelId,
      artist,
      metadata,
      isActive,
    }
  })

  if (!schedules.length && baseDevice) {
    schedules = [
      {
        key: deviceSlugKey(baseDevice.id) || baseDevice.id || 'default',
        label: normalizeValue(baseDevice.channel) || baseDevice.name,
        channelId: normalizeValue(baseDevice.channel),
        artist: fallbackArtist,
        metadata: {},
        isActive: true,
      },
    ]
  }

  if (!schedules.some((schedule) => schedule.isActive) && schedules.length) {
    schedules = schedules.map((schedule, index) => ({
      ...schedule,
      isActive: index === 0,
    }))
  }

  return schedules
}

const buildNowPlaying = (baseDevice, status, schedules, artwork) => {
  const activeSchedule = schedules.find((schedule) => schedule.isActive) || null
  const metadata = activeSchedule?.metadata || {}
  const statusMap = status && typeof status === 'object' ? status : {}

  const title =
    pickFirst([
      metadata.trackTitle,
      metadata.track_title,
      metadata.title,
      statusMap.trackTitle,
      statusMap.nowPlayingTitle,
      statusMap.title,
      statusMap.activeStreamName,
      statusMap.activeStream,
      baseDevice?.channel,
      baseDevice?.name,
      'Now Playing',
    ]) || 'Now Playing'

  const artist =
    pickFirst([
      metadata.trackArtist,
      metadata.track_artist,
      metadata.artist,
      statusMap.trackArtist,
      statusMap.artist,
      baseDevice?.organization,
      'Retail Player',
    ]) || 'Retail Player'

  return {
    title,
    artist,
    album: pickFirst([metadata.album, statusMap.album]),
    artworkUrl: normalizeValue(metadata.artworkUrl),
    streamName: pickFirst([
      statusMap.activeStreamName,
      statusMap.activeStream,
      metadata.streamName,
    ]),
    artworkId: normalizeValue(artwork?.artworkId),
    mediaFileId: normalizeValue(artwork?.mediaFileId),
    metadata,
  }
}

const mapStatusPayloadToDevice = (baseDevice, payload, channelList) => {
  if (!baseDevice) {
    return null
  }

  const status = payload && typeof payload === 'object' ? payload.status || {} : {}
  const schedules = buildSchedules(baseDevice, channelList, payload?.streamMetadata, status)
  const nowPlaying = buildNowPlaying(baseDevice, status, schedules, payload?.artwork)
  const activeSchedule = schedules.find((schedule) => schedule.isActive) || null
  const activeVolume = parseVolume(activeSchedule?.metadata?.volume)
  const statusVolume = parseVolume(status.volume)
  const volume = statusVolume ?? activeVolume ?? 50

  return {
    ...baseDevice,
    schedules,
    nowPlaying,
    volume,
    isMuted: volume === 0,
    isConnected: Boolean(
      pickFirst([status.activeStream, status.activeStreamName, status.ipAddress]),
    ),
    hasSignal: Boolean(
      pickFirst([status.activeStream, status.activeStreamName, status.scheduleStatus]),
    ),
    status,
    streamMetadata: ensureArray(payload?.streamMetadata),
    timeZone: pickFirst([baseDevice.timeZone, status.timeZone]),
    localTime: pickFirst([status.localTime, status.systemTime]),
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
      devices.find((device) => device.slugKey === normalizedSlugKey) ||
      devices.find((device) => deviceSlugKey(device.name) === normalizedSlugKey) ||
      devices.find((device) => deviceSlugKey(device.apiId) === normalizedSlugKey) ||
      devices.find((device) => deviceSlugKey(device.id) === normalizedSlugKey) ||
      null
    )
  }, [devices, normalizedSlugKey])

  const [statusState, setStatusState] = useState(initialStatusState)
  const [channelState, setChannelState] = useState(initialChannelState)
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

    const deviceId = normalizeValue(baseDevice?.apiId || baseDevice?.id)
    if (!deviceId) {
      setStatusState(initialStatusState)
      return undefined
    }

    const url = `/api/retailplayer/devices/${encodeURIComponent(deviceId)}/status`
    const abortController = new AbortController()

    setStatusState((previous) => ({
      ...previous,
      isLoading: true,
      error: null,
    }))

    httpClient(url, { signal: abortController.signal })
      .then(({ json }) => {
        if (abortController.signal.aborted) {
          return
        }
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
        setStatusState({
          data: null,
          error: err,
          isLoading: false,
          fetchedAt: new Date(),
        })
      })

    return () => {
      abortController.abort()
    }
  }, [baseDevice?.apiId, baseDevice?.id, devicesLoading, isApiEnabled, refreshIndex])

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

    const url = `/api/retailplayer/channel-lists/${encodeURIComponent(
      channelListId,
    )}/channels`
    const abortController = new AbortController()

    setChannelState((previous) => ({
      ...previous,
      isLoading: true,
      error: null,
    }))

    httpClient(url, { signal: abortController.signal })
      .then(({ json }) => {
        if (abortController.signal.aborted) {
          return
        }
        setChannelState({
          data: mapChannelList(json?.channels || json?.data),
          error: null,
          isLoading: false,
        })
      })
      .catch((err) => {
        if (abortController.signal.aborted) {
          return
        }
        setChannelState({ data: [], error: err, isLoading: false })
      })

    return () => {
      abortController.abort()
    }
  }, [baseDevice?.channelList, devicesLoading, isApiEnabled, refreshIndex])

  const normalizedDevice = useMemo(
    () => mapStatusPayloadToDevice(baseDevice, statusState.data, channelState.data),
    [baseDevice, channelState.data, statusState.data],
  )

  const notFound =
    Boolean(normalizedSlugKey) &&
    !devicesLoading &&
    (!baseDevice || (statusState.error && statusState.error.status === 404))

  const statusError =
    statusState.error && statusState.error.status !== 404 ? statusState.error : null
  const error = statusError || devicesError || channelState.error || null

  return {
    device: normalizedDevice,
    baseDevice,
    refresh,
    isLoading: Boolean(
      devicesLoading || statusState.isLoading || channelState.isLoading,
    ),
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
