import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import { Button, ButtonBase, Slider, Typography } from '@material-ui/core'
import { Title } from 'react-admin'
import LinkIcon from '@material-ui/icons/Link'
import SignalWifi4BarIcon from '@material-ui/icons/SignalWifi4Bar'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import VolumeUpIcon from '@material-ui/icons/VolumeUp'
import DescriptionIcon from '@material-ui/icons/Description'
import GetAppIcon from '@material-ui/icons/GetApp'
import CachedIcon from '@material-ui/icons/Cached'
import { Link as RouterLink, useParams } from 'react-router-dom'
import {
  getDevice,
  getDeviceStatus,
  postDeviceCommand,
} from '../services/retailPlayerService'

const combineClasses = (...classNames) => classNames.filter(Boolean).join(' ')

const formatTime = (date) =>
  date
    .toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    })
    .replace(/^24:/, '00:')

const uuidRegex = /^[0-9a-fA-F-]{36}$/

const clampVolume = (value) => {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) {
    return 0
  }
  return Math.min(Math.max(Math.round(numeric), 0), 100)
}

const toStringValue = (value) => {
  if (value === null || value === undefined) {
    return null
  }
  if (typeof value === 'string') {
    const trimmed = value.trim()
    return trimmed || null
  }
  if (typeof value === 'number' && Number.isFinite(value)) {
    return String(value)
  }
  if (typeof value === 'boolean') {
    return value ? 'true' : 'false'
  }
  return String(value)
}

const toBooleanValue = (value) => {
  if (typeof value === 'boolean') {
    return value
  }
  if (typeof value === 'number') {
    if (Number.isNaN(value)) {
      return undefined
    }
    return value !== 0
  }
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (!normalized) {
      return undefined
    }
    if (['true', 'yes', 'on', '1'].includes(normalized)) {
      return true
    }
    if (['false', 'no', 'off', '0'].includes(normalized)) {
      return false
    }
  }
  return undefined
}

const getValue = (source, path) => {
  if (!source || !path) {
    return undefined
  }
  const segments = Array.isArray(path) ? path : String(path).split('.')
  let current = source
  for (const segment of segments) {
    if (current === null || current === undefined) {
      return undefined
    }
    current = current[segment]
  }
  return current
}

const firstValue = (source, paths) => {
  if (!source) {
    return undefined
  }
  for (const path of paths) {
    const value = getValue(source, path)
    if (value !== undefined && value !== null) {
      return value
    }
  }
  return undefined
}

const extractArtworkUrl = (value) => {
  const stringValue = toStringValue(value)
  if (stringValue) {
    return stringValue
  }
  if (value && typeof value === 'object') {
    const nested =
      toStringValue(firstValue(value, [['url'], ['href'], ['link'], ['source']])) || null
    return nested
  }
  return null
}

const normalizeDevice = (deviceData, statusData, fallbackId) => {
  const idPaths = [
    ['id'],
    ['deviceId'],
    ['device', 'id'],
    ['device', 'deviceId'],
  ]
  const namePaths = [
    ['name'],
    ['displayName'],
    ['deviceName'],
    ['title'],
    ['device', 'name'],
    ['device', 'displayName'],
  ]
  const connectedPaths = [
    ['isConnected'],
    ['connected'],
    ['online'],
    ['isOnline'],
    ['status', 'connected'],
    ['device', 'isConnected'],
    ['device', 'connected'],
    ['device', 'online'],
  ]
  const signalPaths = [
    ['hasSignal'],
    ['signal'],
    ['signalStatus'],
    ['device', 'hasSignal'],
    ['network', 'hasSignal'],
  ]
  const mutedPaths = [
    ['isMuted'],
    ['muted'],
    ['audio', 'muted'],
    ['device', 'isMuted'],
    ['device', 'muted'],
  ]
  const volumePaths = [
    ['volume'],
    ['device', 'volume'],
    ['audio', 'volume'],
    ['settings', 'volume'],
  ]
  const activeKeyPaths = [
    ['activeChannelKey'],
    ['currentChannelKey'],
    ['channelKey'],
    ['channel', 'key'],
    ['channel', 'id'],
    ['currentChannel', 'key'],
    ['currentChannel', 'id'],
    ['activeChannel', 'key'],
    ['activeChannel', 'id'],
  ]
  const activeNamePaths = [
    ['currentChannel', 'name'],
    ['channel', 'name'],
    ['activeChannel', 'name'],
    ['currentChannelName'],
    ['channelName'],
  ]
  const schedulePaths = [
    ['schedules'],
    ['channels'],
    ['channelList'],
    ['availableChannels'],
    ['device', 'schedules'],
    ['device', 'channels'],
    ['status', 'channels'],
  ]
  const nowPlayingPaths = [
    ['nowPlaying'],
    ['currentTrack'],
    ['track'],
    ['playback', 'nowPlaying'],
  ]

  const resolvedId =
    toStringValue(firstValue(deviceData, idPaths)) ||
    toStringValue(firstValue(statusData, idPaths)) ||
    toStringValue(fallbackId)
  if (!resolvedId) {
    return null
  }

  const resolvedName =
    toStringValue(firstValue(deviceData, namePaths)) ||
    toStringValue(firstValue(statusData, namePaths)) ||
    resolvedId

  const isConnected =
    toBooleanValue(firstValue(statusData, connectedPaths)) ?? false
  const hasSignal =
    toBooleanValue(firstValue(statusData, signalPaths)) ?? isConnected
  const isMuted = toBooleanValue(firstValue(statusData, mutedPaths)) ?? false

  const volumeRaw = firstValue(statusData, volumePaths)
  const volume =
    typeof volumeRaw === 'number'
      ? clampVolume(volumeRaw)
      : clampVolume(Number(volumeRaw))

  const activeKey = toStringValue(firstValue(statusData, activeKeyPaths))
  const activeName = toStringValue(firstValue(statusData, activeNamePaths))

  const scheduleCandidates = []
  for (const path of schedulePaths) {
    const fromDevice = getValue(deviceData, path)
    if (Array.isArray(fromDevice)) {
      scheduleCandidates.push(fromDevice)
    }
    const fromStatus = getValue(statusData, path)
    if (Array.isArray(fromStatus)) {
      scheduleCandidates.push(fromStatus)
    }
  }

  const normalizedSchedules = []
  const seenKeys = new Set()
  scheduleCandidates.forEach((collection) => {
    collection.forEach((item, index) => {
      if (item === null || item === undefined) {
        return
      }

      let key = null
      let label = null
      let isActive = false

      if (typeof item === 'string') {
        key = item
        label = item
      } else if (typeof item === 'object') {
        key =
          toStringValue(
            firstValue(item, [
              ['key'],
              ['id'],
              ['uuid'],
              ['channelId'],
              ['channel', 'id'],
              ['channel', 'key'],
            ]),
          ) || `schedule-${normalizedSchedules.length}`
        label =
          toStringValue(
            firstValue(item, [
              ['label'],
              ['name'],
              ['title'],
              ['description'],
            ]),
          ) || key || `Channel ${normalizedSchedules.length + 1}`
        const activeCandidate =
          toBooleanValue(firstValue(item, [['isActive'], ['active'], ['selected']])) ?? false
        isActive =
          activeCandidate ||
          (key && activeKey && key === activeKey) ||
          (label && activeName && label === activeName)
      } else {
        key = `schedule-${normalizedSchedules.length}`
        label = toStringValue(item) || key
      }

      const resolvedKey = key || `schedule-${normalizedSchedules.length}`
      const resolvedLabel = label || resolvedKey

      if (seenKeys.has(resolvedKey)) {
        const existing = normalizedSchedules.find((schedule) => schedule.key === resolvedKey)
        if (existing) {
          existing.isActive =
            existing.isActive ||
            isActive ||
            (resolvedKey && activeKey && resolvedKey === activeKey) ||
            (resolvedLabel && activeName && resolvedLabel === activeName)
        }
        return
      }

      normalizedSchedules.push({
        key: resolvedKey,
        label: resolvedLabel,
        isActive:
          isActive ||
          (resolvedKey && activeKey && resolvedKey === activeKey) ||
          (resolvedLabel && activeName && resolvedLabel === activeName),
      })
      seenKeys.add(resolvedKey)
    })
  })

  const nowPlayingSource = firstValue(statusData, nowPlayingPaths)
  let nowPlaying = ''
  let artworkUrl =
    extractArtworkUrl(
      firstValue(statusData, [
        ['artworkUrl'],
        ['artwork'],
        ['channel', 'artworkUrl'],
        ['channel', 'artwork'],
      ]),
    ) || null

  if (typeof nowPlayingSource === 'string') {
    nowPlaying = nowPlayingSource.trim()
  } else if (nowPlayingSource && typeof nowPlayingSource === 'object') {
    const nowPlayingTitle =
      toStringValue(
        firstValue(nowPlayingSource, [
          ['title'],
          ['name'],
          ['track'],
          ['song'],
          ['description'],
        ]),
      ) || ''
    const nowPlayingArtist =
      toStringValue(
        firstValue(nowPlayingSource, [
          ['artist'],
          ['artistName'],
          ['artist', 'name'],
          ['artists', 0, 'name'],
          ['artists', 0],
        ]),
      ) || ''
    const combined = [nowPlayingTitle, nowPlayingArtist].filter(Boolean).join(' | ')
    nowPlaying = combined || nowPlayingTitle || nowPlayingArtist || ''
    artworkUrl =
      extractArtworkUrl(
        firstValue(nowPlayingSource, [
          ['artworkUrl'],
          ['artwork'],
          ['artwork', 'url'],
          ['image'],
          ['imageUrl'],
        ]),
      ) || artworkUrl
  }

  return {
    id: resolvedId,
    name: resolvedName,
    isConnected,
    hasSignal,
    isMuted,
    volume,
    schedules: normalizedSchedules,
    nowPlaying,
    activeChannelKey: activeKey || null,
    artworkUrl,
  }
}

const useStyles = makeStyles((theme) => {
  const successMain =
    (theme.palette.success && theme.palette.success.main) ||
    (theme.palette.secondary && theme.palette.secondary.main) ||
    theme.palette.primary.main
  const successContrast =
    (theme.palette.success && theme.palette.success.contrastText) ||
    theme.palette.getContrastText(successMain)
  const dangerMain =
    (theme.palette.error && theme.palette.error.main) ||
    (theme.palette.secondary && theme.palette.secondary.main) ||
    theme.palette.primary.main
  const accentMain =
    (theme.palette.primary && theme.palette.primary.main) ||
    (theme.palette.secondary && theme.palette.secondary.main) ||
    theme.palette.text.primary
  const infoMain =
    (theme.palette.info && theme.palette.info.main) || accentMain
  const sliderMain =
    (theme.palette.secondary && theme.palette.secondary.main) || theme.palette.primary.main
  const disabledBackground =
    (theme.palette.action && theme.palette.action.disabledBackground) ||
    theme.palette.background.paper

  return {
    root: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(5),
      padding: theme.spacing(5),
      maxWidth: 960,
      width: '100%',
      margin: '0 auto',
      [theme.breakpoints.down('md')]: {
        padding: theme.spacing(4),
        gap: theme.spacing(4),
      },
      [theme.breakpoints.down('sm')]: {
        padding: theme.spacing(2.5),
        gap: theme.spacing(3),
      },
    },
    header: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      gap: theme.spacing(2),
      flexWrap: 'wrap',
    },
    backLinkTop: {
      alignSelf: 'flex-start',
      fontSize: theme.typography.pxToRem(14),
      textTransform: 'uppercase',
      letterSpacing: 1,
    },
    title: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(60),
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(44),
      },
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(28),
      },
    },
    statusGroup: {
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(2),
      flexWrap: 'wrap',
      justifyContent: 'flex-end',
    },
    statusIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: theme.typography.pxToRem(32),
      '& svg': {
        fontSize: 'inherit',
      },
    },
    statusIconSuccess: {
      color: successMain,
    },
    statusIconDanger: {
      color: dangerMain,
    },
    statusIconNeutral: {
      color: theme.palette.text.secondary,
    },
    timePill: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      padding: `${theme.spacing(0.5)}px ${theme.spacing(2)}px`,
      borderRadius: theme.shape.borderRadius * 2,
      backgroundColor: successMain,
      color: successContrast,
      fontWeight: theme.typography.fontWeightMedium,
      fontSize: theme.typography.pxToRem(18),
    },
    inlineError: {
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(1.5),
      padding: `${theme.spacing(1)}px ${theme.spacing(1.5)}px`,
      borderRadius: theme.shape.borderRadius,
      backgroundColor: theme.palette.action.hover,
      color: theme.palette.error.main,
      fontSize: theme.typography.pxToRem(14),
    },
    inlineErrorButton: {
      padding: `${theme.spacing(0.25)}px ${theme.spacing(1.5)}px`,
    },
    compactErrorWrapper: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(1.5),
      padding: theme.spacing(3),
      maxWidth: 480,
      width: '100%',
      margin: '0 auto',
    },
    compactErrorMessage: {
      color: theme.palette.error.main,
      fontWeight: theme.typography.fontWeightMedium,
    },
    list: {
      borderRadius: theme.shape.borderRadius,
      backgroundColor: theme.palette.background.paper,
      overflow: 'hidden',
      border: `1px solid ${theme.palette.divider}`,
    },
    listItemButton: {
      display: 'block',
      width: '100%',
      textAlign: 'left',
      '&:hover $listItem, &:focus-visible $listItem': {
        backgroundColor: theme.palette.action.hover,
      },
      '&:last-child $listItem': {
        borderBottom: 'none',
      },
    },
    listItem: {
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(2),
      padding: `${theme.spacing(2)}px ${theme.spacing(3)}px`,
      borderBottom: `1px solid ${theme.palette.divider}`,
      transition: theme.transitions.create(['background-color'], {
        duration: theme.transitions.duration.shortest,
      }),
    },
    listItemActive: {
      backgroundColor: theme.palette.action.selected,
    },
    listItemInactive: {
      backgroundColor: disabledBackground,
    },
    listIcon: {
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(24),
    },
    listIconInactive: {
      color: theme.palette.text.disabled,
    },
    listText: {
      fontSize: theme.typography.pxToRem(20),
      fontWeight: theme.typography.fontWeightMedium,
    },
    listTextInactive: {
      color: theme.palette.text.disabled,
    },
    artworkWrapper: {
      alignSelf: 'center',
      width: 200,
      maxWidth: '100%',
    },
    artworkPlaceholder: {
      width: '100%',
      paddingTop: '100%',
      borderRadius: theme.shape.borderRadius,
      backgroundColor: theme.palette.action.hover,
      border: `1px solid ${theme.palette.divider}`,
    },
    nowPlaying: {
      fontSize: theme.typography.pxToRem(48),
      fontWeight: theme.typography.fontWeightBold,
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(36),
      },
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(26),
      },
    },
    controls: {
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(4),
      flexWrap: 'wrap',
      justifyContent: 'space-between',
      width: '100%',
    },
    controlButton: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      borderRadius: theme.shape.borderRadius * 2,
      padding: theme.spacing(1.5),
      transition: theme.transitions.create(['background-color', 'color'], {
        duration: theme.transitions.duration.shortest,
      }),
    },
    controlIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: theme.typography.pxToRem(64),
    },
    controlButtonMuted: {
      color: dangerMain,
    },
    controlButtonUnmuted: {
      color: successMain,
    },
    controlIconDownload: {
      color: infoMain,
    },
    volumeControl: {
      display: 'flex',
      flexDirection: 'column',
      flex: 1,
      minWidth: 240,
      gap: theme.spacing(1),
      maxWidth: 480,
    },
    volumeLabel: {
      textTransform: 'lowercase',
      fontSize: theme.typography.pxToRem(16),
      color: theme.palette.text.secondary,
    },
    volumeSliderRow: {
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(2),
    },
    slider: {
      color: sliderMain,
    },
    sliderTrack: {
      backgroundColor: sliderMain,
    },
    sliderThumb: {
      backgroundColor: sliderMain,
    },
    sliderRail: {
      backgroundColor: theme.palette.action.disabled,
    },
    volumeValue: {
      minWidth: 32,
      textAlign: 'right',
      fontVariantNumeric: 'tabular-nums',
      fontWeight: theme.typography.fontWeightMedium,
    },
    notFoundWrapper: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(2),
      padding: theme.spacing(5),
      maxWidth: 720,
      width: '100%',
      margin: '0 auto',
      [theme.breakpoints.down('sm')]: {
        padding: theme.spacing(3),
      },
    },
    notFoundTitle: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(36),
    },
    notFoundMessage: {
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(18),
    },
    backLink: {
      color:
        (theme.palette.primary && theme.palette.primary.main) ||
        theme.palette.text.primary,
      fontWeight: theme.typography.fontWeightMedium,
      textDecoration: 'none',
      display: 'inline-flex',
      alignItems: 'center',
      gap: theme.spacing(1),
    },
    refreshButton: {
      borderRadius: theme.shape.borderRadius * 2,
      padding: theme.spacing(0.75),
      border: `1px solid ${theme.palette.divider}`,
      '&:hover, &:focus-visible': {
        backgroundColor: theme.palette.action.hover,
      },
    },
    artworkImage: {
      width: '100%',
      height: 'auto',
      borderRadius: theme.shape.borderRadius,
      display: 'block',
    },
  }
})

const RetailPlayerDashboard = () => {
  const classes = useStyles()
  const { deviceId } = useParams()
  const deviceInfoRef = useRef(null)
  const [device, setDevice] = useState(null)
  const [currentTime, setCurrentTime] = useState(() => formatTime(new Date()))
  const [isLoading, setIsLoading] = useState(true)
  const [loadError, setLoadError] = useState(null)

  const isValidDeviceId = useMemo(
    () => Boolean(deviceId) && uuidRegex.test(deviceId),
    [deviceId],
  )

  useEffect(() => {
    console.log('[RetailPlayerDashboard] deviceId:', deviceId)
  }, [deviceId])

  const refreshStatusOnly = useCallback(async () => {
    if (!isValidDeviceId) {
      return
    }
    try {
      const statusResponse = await getDeviceStatus(deviceId)
      setDevice((previous) => {
        const normalized = normalizeDevice(deviceInfoRef.current, statusResponse, deviceId)
        return normalized ?? previous
      })
      setCurrentTime(formatTime(new Date()))
      setLoadError(null)
    } catch (error) {
      console.error(`[RetailPlayerDashboard] Failed to refresh status for ${deviceId}`, error)
      setLoadError("Couldn't load device details.")
    }
  }, [deviceId, isValidDeviceId])

  const refreshDevice = useCallback(async () => {
    if (!isValidDeviceId) {
      return
    }
    console.log(`[RetailPlayerDashboard] Refreshing device ${deviceId}`)
    setIsLoading(true)
    setLoadError(null)
    try {
      const [deviceResponse, statusResponse] = await Promise.all([
        getDevice(deviceId),
        getDeviceStatus(deviceId),
      ])
      deviceInfoRef.current = deviceResponse || null
      const normalized = normalizeDevice(deviceResponse, statusResponse, deviceId)
      setDevice(normalized)
      setCurrentTime(formatTime(new Date()))
    } catch (error) {
      console.error(`[RetailPlayerDashboard] Failed to load device ${deviceId}`, error)
      deviceInfoRef.current = null
      setDevice(null)
      setLoadError("Couldn't load device details.")
    } finally {
      setIsLoading(false)
    }
  }, [deviceId, isValidDeviceId])

  useEffect(() => {
    if (!isValidDeviceId) {
      deviceInfoRef.current = null
      setDevice(null)
      setIsLoading(false)
      setLoadError(null)
      return
    }
    refreshDevice()
  }, [isValidDeviceId, refreshDevice])

  useEffect(() => {
    const updateTime = () => setCurrentTime(formatTime(new Date()))
    const intervalId = window.setInterval(updateTime, 60000)
    return () => window.clearInterval(intervalId)
  }, [])

  const schedules = useMemo(() => device?.schedules || [], [device])

  const activeChannelKey = useMemo(() => {
    if (device?.activeChannelKey) {
      return device.activeChannelKey
    }
    const activeSchedule = schedules.find((schedule) => schedule.isActive)
    return activeSchedule ? activeSchedule.key : null
  }, [device, schedules])

  const isMuted = Boolean(device?.isMuted)
  const volume = typeof device?.volume === 'number' ? device.volume : 0
  const nowPlaying = device?.nowPlaying || ''
  const artworkUrl = device?.artworkUrl || null

  const statusItems = useMemo(() => {
    if (!device) {
      return []
    }

    return [
      {
        key: 'connected',
        icon: LinkIcon,
        intent: device.isConnected ? 'success' : 'danger',
        label: 'Connected',
      },
      { key: 'time', label: currentTime, labelForAria: 'Time' },
      {
        key: 'signal',
        icon: SignalWifi4BarIcon,
        intent: device.hasSignal ? 'success' : 'danger',
        label: 'Signal',
      },
      {
        key: 'muted',
        icon: isMuted ? VolumeOffIcon : VolumeUpIcon,
        intent: isMuted ? 'danger' : 'success',
        label: isMuted ? 'Muted' : 'Audio Enabled',
      },
    ]
  }, [currentTime, device, isMuted])

  const sendCommand = useCallback(
    async (payload) => {
      if (!isValidDeviceId) {
        return
      }
      try {
        await postDeviceCommand(deviceId, payload)
        await refreshStatusOnly()
      } catch (error) {
        console.error(`[RetailPlayerDashboard] Command failed for ${deviceId}`, error)
      }
    },
    [deviceId, isValidDeviceId, refreshStatusOnly],
  )

  const handleSelectChannel = useCallback(
    (schedule) => {
      if (!device || !schedule) {
        return
      }
      setDevice((previous) => {
        if (!previous) {
          return previous
        }
        return {
          ...previous,
          activeChannelKey: schedule.key,
          schedules: previous.schedules.map((item) => ({
            ...item,
            isActive: item.key === schedule.key,
          })),
        }
      })
      void sendCommand({ type: 'channel', channel: schedule.key })
    },
    [device, sendCommand],
  )

  const handleToggleMute = useCallback(() => {
    if (!device) {
      return
    }
    const nextMuted = !device.isMuted
    setDevice((previous) => (previous ? { ...previous, isMuted: nextMuted } : previous))
    void sendCommand({ type: 'mute', value: nextMuted })
  }, [device, sendCommand])

  const handleVolumeChange = useCallback(
    (_, newValue) => {
      if (!device) {
        return
      }

      const resolvedValue = Array.isArray(newValue) ? newValue[0] : newValue
      if (typeof resolvedValue !== 'number' || Number.isNaN(resolvedValue)) {
        return
      }

      const clampedValue = clampVolume(resolvedValue)
      setDevice((previous) =>
        previous ? { ...previous, volume: clampedValue } : previous,
      )
      void sendCommand({ type: 'volume', value: clampedValue })
    },
    [device, sendCommand],
  )

  const handleRefresh = useCallback(() => {
    if (!isValidDeviceId) {
      return
    }
    void refreshDevice()
  }, [isValidDeviceId, refreshDevice])

  const handleAdjustVolume = useCallback(
    (delta) => {
      if (!device) {
        return
      }
      const nextVolume = clampVolume((device.volume ?? 0) + delta)
      setDevice((previous) => (previous ? { ...previous, volume: nextVolume } : previous))
      void sendCommand({ type: 'volume', value: nextVolume })
    },
    [device, sendCommand],
  )

  const handleShortcutChannel = useCallback(
    (index) => {
      const schedule = schedules[index]
      if (!schedule) {
        return
      }
      handleSelectChannel(schedule)
    },
    [handleSelectChannel, schedules],
  )

  useEffect(() => {
    const handleKeyDown = (event) => {
      if (!device) {
        return
      }

      const target = event.target
      const tagName = target && target.tagName
      if (
        tagName === 'INPUT' ||
        tagName === 'TEXTAREA' ||
        (target && target.isContentEditable)
      ) {
        return
      }

      switch (event.key) {
        case 'm':
        case 'M':
          event.preventDefault()
          handleToggleMute()
          break
        case '+':
        case '=':
          event.preventDefault()
          handleAdjustVolume(5)
          break
        case '-':
          event.preventDefault()
          handleAdjustVolume(-5)
          break
        case '1':
          event.preventDefault()
          handleShortcutChannel(0)
          break
        case '2':
          event.preventDefault()
          handleShortcutChannel(1)
          break
        case '3':
          event.preventDefault()
          handleShortcutChannel(2)
          break
        default:
          break
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [device, handleAdjustVolume, handleShortcutChannel, handleToggleMute])

  if (!isValidDeviceId) {
    return (
      <div className={classes.compactErrorWrapper}>
        <Title title="Retail Player" />
        <Typography className={classes.compactErrorMessage}>Invalid device id</Typography>
        <RouterLink to="/retailplayer/devices" className={classes.backLink}>
          ← Back to devices
        </RouterLink>
      </div>
    )
  }

  if (!device && isLoading) {
    return (
      <div className={classes.notFoundWrapper}>
        <Title title="Retail Player" />
        <Typography className={classes.notFoundMessage}>Loading device…</Typography>
      </div>
    )
  }

  if (!device) {
    if (loadError) {
      return (
        <div className={classes.compactErrorWrapper}>
          <Title title="Retail Player" />
          <Typography className={classes.compactErrorMessage}>{loadError}</Typography>
          <Button
            className={classes.inlineErrorButton}
            variant="outlined"
            size="small"
            onClick={handleRefresh}
            disabled={isLoading}
          >
            Retry
          </Button>
          <RouterLink to="/retailplayer/devices" className={classes.backLink}>
            ← Back to devices
          </RouterLink>
        </div>
      )
    }

    return (
      <div className={classes.notFoundWrapper}>
        <Title title="Retail Player" />
        <Typography component="h1" className={classes.notFoundTitle}>
          Device not found
        </Typography>
        <Typography className={classes.notFoundMessage}>
          The device you are looking for is unavailable. Choose a device from the list to
          continue.
        </Typography>
        <RouterLink to="/retailplayer/devices" className={classes.backLink}>
          ← Back to devices
        </RouterLink>
      </div>
    )
  }

  return (
    <div className={classes.root}>
      <Title title="Retail Player" />
      <RouterLink to="/retailplayer/devices" className={`${classes.backLink} ${classes.backLinkTop}`}>
        ← Back to Devices
      </RouterLink>
      {loadError && (
        <div className={classes.inlineError} role="alert">
          <span>{loadError}</span>
          <Button
            className={classes.inlineErrorButton}
            variant="outlined"
            size="small"
            onClick={handleRefresh}
            disabled={isLoading}
          >
            Retry
          </Button>
        </div>
      )}
      <header className={classes.header}>
        <Typography component="h1" className={classes.title}>
          {device.name}
        </Typography>
        <div className={classes.statusGroup}>
          <ButtonBase
            className={combineClasses(classes.statusIcon, classes.statusIconNeutral, classes.refreshButton)}
            onClick={handleRefresh}
            aria-label="Refresh"
            focusRipple
            disabled={isLoading}
          >
            <CachedIcon fontSize="inherit" />
          </ButtonBase>
          {statusItems.map((statusItem) => {
            if (statusItem.key === 'time') {
              return (
                <span
                  key={statusItem.key}
                  className={classes.timePill}
                  aria-label={statusItem.labelForAria || 'Time'}
                >
                  {statusItem.label}
                </span>
              )
            }

            const StatusIcon = statusItem.icon
            return (
              <span
                key={statusItem.key}
                className={combineClasses(
                  classes.statusIcon,
                  statusItem.intent === 'success'
                    ? classes.statusIconSuccess
                    : classes.statusIconDanger,
                )}
                aria-label={statusItem.label}
                role="img"
              >
                <StatusIcon fontSize="inherit" />
              </span>
            )
          })}
        </div>
      </header>

      <section className={classes.list} aria-label="Available schedules">
        {device.schedules.map((schedule) => {
          const isActive = schedule.key === activeChannelKey
          return (
            <ButtonBase
              key={schedule.key}
              className={classes.listItemButton}
              onClick={() => handleSelectChannel(schedule)}
              focusRipple
              aria-label={`Select channel: ${schedule.label}`}
              aria-pressed={isActive}
            >
              <div
                className={combineClasses(
                  classes.listItem,
                  isActive ? classes.listItemActive : classes.listItemInactive,
                )}
              >
                <DescriptionIcon
                  className={combineClasses(
                    classes.listIcon,
                    !isActive ? classes.listIconInactive : null,
                  )}
                  aria-hidden="true"
                />
                <Typography
                  className={combineClasses(
                    classes.listText,
                    !isActive ? classes.listTextInactive : null,
                  )}
                >
                  {schedule.label}
                </Typography>
              </div>
            </ButtonBase>
          )
        })}
      </section>

      <div className={classes.artworkWrapper}>
        {artworkUrl ? (
          <img
            src={artworkUrl}
            alt="Album artwork"
            className={classes.artworkImage}
          />
        ) : (
          <div className={classes.artworkPlaceholder} role="img" aria-label="Album artwork placeholder" />
        )}
      </div>

      <Typography component="h2" className={classes.nowPlaying}>
        {nowPlaying || '—'}
      </Typography>

      <section className={classes.controls}>
        <ButtonBase
          className={combineClasses(
            classes.controlButton,
            isMuted ? classes.controlButtonMuted : classes.controlButtonUnmuted,
          )}
          aria-label="Mute/Unmute"
          onClick={handleToggleMute}
          focusRipple
        >
          <span className={classes.controlIcon} role="img" aria-hidden="true">
            {isMuted ? <VolumeOffIcon fontSize="inherit" /> : <VolumeUpIcon fontSize="inherit" />}
          </span>
        </ButtonBase>
        <div className={classes.volumeControl}>
          <Typography className={classes.volumeLabel}>volume</Typography>
          <div className={classes.volumeSliderRow}>
            <Slider
              classes={{
                root: classes.slider,
                track: classes.sliderTrack,
                thumb: classes.sliderThumb,
                rail: classes.sliderRail,
              }}
              value={volume}
              min={0}
              max={100}
              aria-label="Volume"
              onChange={handleVolumeChange}
            />
            <Typography className={classes.volumeValue} aria-live="polite">
              {volume}
            </Typography>
          </div>
        </div>
        <span
          className={`${classes.controlIcon} ${classes.controlIconDownload}`}
          aria-label="Download"
          role="img"
        >
          <GetAppIcon fontSize="inherit" />
        </span>
      </section>
    </div>
  )
}

export default RetailPlayerDashboard
