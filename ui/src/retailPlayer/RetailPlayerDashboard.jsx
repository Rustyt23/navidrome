import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import { ButtonBase, Slider, Typography } from '@material-ui/core'
import { Title } from 'react-admin'
import LinkIcon from '@material-ui/icons/Link'
import SignalWifi4BarIcon from '@material-ui/icons/SignalWifi4Bar'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import VolumeUpIcon from '@material-ui/icons/VolumeUp'
import DescriptionIcon from '@material-ui/icons/Description'
import CachedIcon from '@material-ui/icons/Cached'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import { useParams } from 'react-router-dom'
import { BiDislike } from 'react-icons/bi'
import { MdSkipNext } from 'react-icons/md'
import useRetailPlayerDeviceStatus from './useRetailPlayerDeviceStatus'
import { normalizeValue } from './deviceUtils'
import httpClient from '../dataProvider/httpClient'

const combineClasses = (...classNames) => classNames.filter(Boolean).join(' ')

const clamp = (value, min, max) => Math.min(Math.max(value, min), max)

const formatTime = (date, timeZone) => {
  if (!(date instanceof Date) || Number.isNaN(date.getTime())) {
    return '--:--'
  }

  try {
    return new Intl.DateTimeFormat([], {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
      ...(timeZone ? { timeZone } : {}),
    })
      .format(date)
      .replace(/^24:/, '00:')
  } catch (err) {
    return date
      .toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
        hour12: false,
      })
      .replace(/^24:/, '00:')
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
  const sliderMain =
    (theme.palette.secondary && theme.palette.secondary.main) || theme.palette.primary.main
  const accentColor =
    (theme.palette.secondary && theme.palette.secondary.main) || '#ff6f9f'
  const disabledBackground =
    (theme.palette.action && theme.palette.action.disabledBackground) ||
    theme.palette.background.paper

  return {
    root: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(3.5),
      padding: `${theme.spacing(1)}px ${theme.spacing(4)}px`,
      width: '100%',
      maxWidth: '50vw',
      margin: '0 auto',
      boxSizing: 'border-box',
      minHeight: '100vh',
      alignItems: 'center',
      [theme.breakpoints.down('md')]: {
        padding: `${theme.spacing(4)}px ${theme.spacing(3)}px`,
        gap: theme.spacing(3),
        maxWidth: '100%',
      },
      [theme.breakpoints.down('sm')]: {
        padding: `${theme.spacing(3)}px ${theme.spacing(2.5)}px`,
        gap: theme.spacing(2.5),
      },
    },
    header: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      flexDirection: 'column',
      gap: theme.spacing(2),
      flexWrap: 'wrap',
      width: '100%',
      maxWidth: 960,
      textAlign: 'center',
      paddingLeft: 0,
    },
    title: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(52),
      letterSpacing: '-0.01em',
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(40),
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
      justifyContent: 'center',
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
    list: {
      borderRadius: theme.shape.borderRadius * 1.5,
      backgroundColor: theme.palette.background.paper,
      overflow: 'hidden',
      border: `1px solid ${theme.palette.divider}`,
      width: '100%',
      maxWidth: 960,
      boxShadow: '0 22px 45px rgba(0, 0, 0, 0.28)',
      transition: theme.transitions.create(['box-shadow'], {
        duration: theme.transitions.duration.shorter,
      }),
      '&:hover, &:focus-within': {
        boxShadow: '0 30px 60px rgba(0, 0, 0, 0.36)',
      },
    },
    mainContent: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(3),
      width: '100%',
      maxWidth: 960,
      margin: '0 auto',
      alignItems: 'center',
    },
    nowPlayingCard: {
      borderRadius: theme.shape.borderRadius * 1.5,
      backgroundColor: theme.palette.background.paper,
      border: `1px solid ${theme.palette.divider}`,
      padding: theme.spacing(3),
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: theme.spacing(3),
      width: '100%',
      maxWidth: 960,
      boxShadow: '0 26px 55px rgba(0, 0, 0, 0.32)',
      transition: theme.transitions.create(['box-shadow', 'transform'], {
        duration: theme.transitions.duration.shorter,
        easing: theme.transitions.easing.easeInOut,
      }),
      '&:hover, &:focus-within': {
        boxShadow: '0 36px 70px rgba(0, 0, 0, 0.38)',
        transform: 'translateY(-2px)',
      },
      [theme.breakpoints.down('sm')]: {
        padding: theme.spacing(2.5),
      },
    },
    locationLabel: {
      width: '100%',
      textAlign: 'center',
      fontSize: theme.typography.pxToRem(20),
      fontWeight: theme.typography.fontWeightMedium,
      color: accentColor,
      letterSpacing: 0.5,
    },
    nowPlayingBody: {
      display: 'flex',
      flexDirection: 'column',
      justifyContent: 'center',
      alignItems: 'center',
      textAlign: 'center',
      gap: theme.spacing(3),
      flex: 1,
      minWidth: 0,
      width: '100%',
    },
    nowPlayingHeader: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: theme.spacing(1),
      width: '100%',
      textAlign: 'center',
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
      padding: `${theme.spacing(2.25)}px ${theme.spacing(3)}px`,
      borderBottom: `1px solid ${theme.palette.divider}`,
      transition: theme.transitions.create(['background-color'], {
        duration: theme.transitions.duration.shortest,
      }),
    },
    listIcon: {
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(24),
    },
    playlistLabel: {
      flex: 1,
      minWidth: 0,
    },
    playlistLabelActive: {
      color: accentColor,
    },
    playlistLabelInactive: {
      color: theme.palette.text.secondary,
    },
    playlistIconActive: {
      color: accentColor,
    },
    playlistIconInactive: {
      color: theme.palette.text.secondary,
    },
    dropdownWrapper: {
      display: 'flex',
      flexDirection: 'column',
      width: '100%',
      backgroundColor: theme.palette.background.paper,
    },
    dropdownTriggerButton: {
      borderBottom: `1px solid ${theme.palette.divider}`,
      display: 'flex',
      alignItems: 'center',
      width: '100%',
      transition: theme.transitions.create(['background-color'], {
        duration: theme.transitions.duration.shorter,
      }),
    },
    dropdownCaret: {
      marginLeft: 'auto',
      transition: theme.transitions.create(['transform'], {
        duration: theme.transitions.duration.shortest,
      }),
      color: accentColor,
    },
    dropdownCaretOpen: {
      transform: 'rotate(180deg)',
    },
    dropdownMenu: {
      display: 'grid',
      gridAutoRows: 'min-content',
      backgroundColor: theme.palette.background.paper,
      transition: theme.transitions.create(['max-height', 'opacity'], {
        duration: theme.transitions.duration.short,
        easing: theme.transitions.easing.easeInOut,
      }),
      maxHeight: 0,
      opacity: 0,
      pointerEvents: 'none',
      overflow: 'hidden',
      boxShadow: '0 20px 40px rgba(0, 0, 0, 0.3)',
      borderTop: `1px solid ${theme.palette.divider}`,
      justifyItems: 'center',
    },
    dropdownMenuOpen: {
      maxHeight: 320,
      opacity: 1,
      pointerEvents: 'auto',
    },
    dropdownOptionButton: {
      textAlign: 'center',
      '&:last-child $listItem': {
        borderBottom: 'none',
      },
    },
    dropdownOptionContent: {
      justifyContent: 'center',
      textAlign: 'center',
    },
    listText: {
      fontSize: theme.typography.pxToRem(18),
      fontWeight: theme.typography.fontWeightMedium,
      letterSpacing: 0.2,
    },
    dropdownOptionLabel: {
      textAlign: 'center',
    },
    artworkWrapper: {
      width: 'clamp(120px, 20vw, 180px)',
      maxWidth: '100%',
      flexShrink: 0,
      alignSelf: 'center',
      [theme.breakpoints.down('md')]: {
        width: 'clamp(120px, 32vw, 200px)',
      },
      [theme.breakpoints.down('sm')]: {
        width: 'min(160px, 70%)',
      },
    },
    artworkCircle: {
      position: 'relative',
      width: '100%',
      paddingTop: '100%',
      borderRadius: '50%',
      overflow: 'hidden',
      backgroundColor:
        (theme.palette.action && theme.palette.action.disabledBackground) ||
        theme.palette.action.hover,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      boxShadow: 'inset 0 0 0 2px rgba(255, 255, 255, 0.04)',
    },
    artworkContent: {
      position: 'absolute',
      inset: 0,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
    },
    artworkImage: {
      width: '100%',
      height: '100%',
      objectFit: 'cover',
      borderRadius: '50%',
      display: 'block',
    },
    discSvg: {
      width: '80%',
      height: '80%',
      maxWidth: 180,
      maxHeight: 180,
    },
    discOuter: {
      fill:
        (theme.palette.action && theme.palette.action.disabled) ||
        theme.palette.grey[400],
    },
    discInner: {
      fill:
        (theme.palette.background && theme.palette.background.paper) ||
        theme.palette.common.white,
    },
    discHighlight: {
      fill:
        (theme.palette.primary && theme.palette.primary.main) ||
        theme.palette.text.primary,
      opacity: 0.2,
    },
    nowPlayingTitle: {
      fontSize: theme.typography.pxToRem(32),
      fontWeight: 600,
      textAlign: 'center',
      width: '100%',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(28),
      },
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(22),
      },
    },
    nowPlayingArtist: {
      fontSize: theme.typography.pxToRem(18),
      textAlign: 'center',
      color: theme.palette.text.secondary,
      width: '100%',
      overflow: 'hidden',
      textOverflow: 'ellipsis',
      whiteSpace: 'nowrap',
      letterSpacing: 0.2,
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(15),
      },
    },
    nowPlayingFooter: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      gap: theme.spacing(2.5),
      flexWrap: 'wrap',
      width: '100%',
    },
    controlsRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: theme.spacing(3),
      flexWrap: 'wrap',
    },
    controlButton: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      width: 52,
      height: 52,
      borderRadius: '50%',
      transition: theme.transitions.create(['background-color', 'color'], {
        duration: theme.transitions.duration.shortest,
      }),
      color: theme.palette.text.primary,
      '&:hover, &:focus-visible': {
        backgroundColor: theme.palette.action.hover,
      },
    },
    controlButtonMuted: {
      color: dangerMain,
    },
    controlIcon: {
      fontSize: theme.typography.pxToRem(28),
      display: 'inline-flex',
    },
    volumeSection: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(1.5),
      width: '100%',
      maxWidth: 360,
      minWidth: 0,
      alignSelf: 'center',
      margin: '0 auto',
      position: 'relative',
      [theme.breakpoints.down('md')]: {
        maxWidth: 420,
      },
      [theme.breakpoints.down('sm')]: {
        width: '100%',
        minWidth: 'auto',
      },
      '&:hover $volumeLabelRow, &:focus-within $volumeLabelRow': {
        opacity: 1,
        transform: 'translateY(0)',
      },
    },
    volumeLabelRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      color: theme.palette.text.secondary,
      textTransform: 'lowercase',
      opacity: 0,
      pointerEvents: 'none',
      transform: 'translateY(-6px)',
      transition: theme.transitions.create(['opacity', 'transform'], {
        duration: theme.transitions.duration.shortest,
      }),
    },
    slider: {
      color: sliderMain,
      width: '100%',
      margin: '0 auto',
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
    dislikeMessage: {
      marginTop: theme.spacing(1),
      textAlign: 'center',
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(14),
      width: '100%',
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
    refreshButton: {
      borderRadius: theme.shape.borderRadius * 2,
      padding: theme.spacing(0.75),
      border: `1px solid ${theme.palette.divider}`,
      '&:hover, &:focus-visible': {
        backgroundColor: theme.palette.action.hover,
      },
    },
  }
})

const RetailPlayerDashboard = () => {
  const classes = useStyles()
  const { deviceSlug } = useParams()
  const {
    device: resolvedDevice,
    baseDevice,
    error: integrationError,
    statusError,
    devicesError,
    isLoading: retailLoading,
    isStatusLoading: statusLoading,
    refresh: refreshStatus,
    notFound,
    isApiEnabled,
  } = useRetailPlayerDeviceStatus(deviceSlug)
  const [device, setDevice] = useState(resolvedDevice)
  const [deviceTime, setDeviceTime] = useState(() => new Date())
  const [isMuted, setIsMuted] = useState(false)
  const [volume, setVolume] = useState(50)
  const [displayVolume, setDisplayVolume] = useState(50)
  const volumeTimeoutRef = useRef(null)
  const previousVolumeRef = useRef(50)
  const volumeSyncReadyRef = useRef(false)
  const dislikeTimeoutRef = useRef(null)
  const dislikeRequestControllerRef = useRef(null)
  const channelRequestControllerRef = useRef(null)
  const [showDislikeMessage, setShowDislikeMessage] = useState(false)
  const [previousNowPlaying, setPreviousNowPlaying] = useState(null)
  const [currentTrackIndex, setCurrentTrackIndex] = useState(0)
  const [isScheduleMenuOpen, setScheduleMenuOpen] = useState(false)
  const scheduleDropdownRef = useRef(null)
  const isBusy = retailLoading || statusLoading
  const combinedError = integrationError || statusError || devicesError

  const deviceTrackKey = useMemo(() => {
    if (!device) {
      return ''
    }

    return (
      normalizeValue(device.apiId) ||
      normalizeValue(device.id) ||
      normalizeValue(device.slug) ||
      normalizeValue(device.macAddress)
    )
  }, [device])

  useEffect(() => {
    setPreviousNowPlaying(null)
  }, [deviceTrackKey])

  useEffect(() => {
    if (!isApiEnabled) {
      return undefined
    }

    const intervalId = window.setInterval(() => {
      refreshStatus()
    }, 3000)

    return () => {
      window.clearInterval(intervalId)
    }
  }, [isApiEnabled, refreshStatus])

  const deviceApiId = useMemo(() => {
    if (resolvedDevice?.apiId) {
      return resolvedDevice.apiId
    }
    if (resolvedDevice?.id) {
      return resolvedDevice.id
    }
    if (baseDevice?.apiId) {
      return baseDevice.apiId
    }
    if (baseDevice?.id) {
      return baseDevice.id
    }
    if (device?.apiId) {
      return device.apiId
    }
    if (device?.id) {
      return device.id
    }
    return ''
  }, [
    baseDevice?.apiId,
    baseDevice?.id,
    device?.apiId,
    device?.id,
    resolvedDevice?.apiId,
    resolvedDevice?.id,
  ])

  const canControlDevice = useMemo(
    () => Boolean(isApiEnabled && deviceApiId),
    [deviceApiId, isApiEnabled],
  )

  const resolveDeviceTime = useCallback((sourceDevice) => {
    if (!sourceDevice) {
      return new Date()
    }

    const directLocalTime =
      typeof sourceDevice.localTime === 'string' ? sourceDevice.localTime : null
    if (directLocalTime) {
      const parsedDirect = new Date(directLocalTime)
      if (!Number.isNaN(parsedDirect.getTime())) {
        return parsedDirect
      }
    }

    const status =
      sourceDevice.status && typeof sourceDevice.status === 'object'
        ? sourceDevice.status
        : {}

    const localTimeValue =
      typeof status.localTime === 'string' ? status.localTime : null
    if (localTimeValue) {
      const parsedLocal = new Date(localTimeValue)
      if (!Number.isNaN(parsedLocal.getTime())) {
        return parsedLocal
      }
    }

    const systemTimeValue =
      typeof status.systemTime === 'string' ? status.systemTime : null
    if (systemTimeValue) {
      const parsedSystem = new Date(systemTimeValue)
      if (!Number.isNaN(parsedSystem.getTime())) {
        return parsedSystem
      }
    }

    return new Date()
  }, [])

  useEffect(() => {
    setDevice(resolvedDevice || null)
    setDeviceTime(resolveDeviceTime(resolvedDevice))
  }, [resolvedDevice, resolveDeviceTime])

  useEffect(() => {
    const intervalId = window.setInterval(() => {
      setDeviceTime((previous) => {
        if (!(previous instanceof Date) || Number.isNaN(previous.getTime())) {
          return new Date()
        }
        return new Date(previous.getTime() + 60000)
      })
    }, 60000)

    return () => window.clearInterval(intervalId)
  }, [])

  const schedules = useMemo(() => device?.schedules || [], [device])

  const activeChannelKey = useMemo(() => {
    const activeSchedule = schedules.find((schedule) => schedule.isActive)
    return activeSchedule ? activeSchedule.key : null
  }, [schedules])

  const [pendingActiveChannelKey, setPendingActiveChannelKey] = useState(null)

  useEffect(() => {
    if (pendingActiveChannelKey && activeChannelKey === pendingActiveChannelKey) {
      setPendingActiveChannelKey(null)
    }
  }, [activeChannelKey, pendingActiveChannelKey])

  const effectiveActiveChannelKey = pendingActiveChannelKey || activeChannelKey

  const availableSchedules = useMemo(
    () => schedules.filter((schedule) => schedule.key !== effectiveActiveChannelKey),
    [effectiveActiveChannelKey, schedules],
  )
  const availableSchedulesCount = availableSchedules.length

  useEffect(() => {
    if (!isScheduleMenuOpen) {
      return undefined
    }

    const handleClickOutside = (event) => {
      if (
        scheduleDropdownRef.current &&
        !scheduleDropdownRef.current.contains(event.target)
      ) {
        setScheduleMenuOpen(false)
      }
    }

    const handleEscape = (event) => {
      if (event.key === 'Escape') {
        setScheduleMenuOpen(false)
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    document.addEventListener('keydown', handleEscape)

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      document.removeEventListener('keydown', handleEscape)
    }
  }, [isScheduleMenuOpen])

  useEffect(() => {
    setScheduleMenuOpen(false)
  }, [activeChannelKey])

  useEffect(() => {
    if (availableSchedulesCount === 0) {
      setScheduleMenuOpen(false)
    }
  }, [availableSchedulesCount])

  const activeSchedule = useMemo(() => {
    if (!schedules.length) {
      return null
    }
    const matched = schedules.find((schedule) => schedule.key === effectiveActiveChannelKey)
    return matched || schedules[0]
  }, [effectiveActiveChannelKey, schedules])

  const sendDislikeNotification = useCallback(() => {
    if (!isApiEnabled || !deviceApiId) {
      return
    }

    const nowPlaying = device?.nowPlaying && typeof device.nowPlaying === 'object' ? device.nowPlaying : {}
    const metadata =
      nowPlaying?.metadata && typeof nowPlaying.metadata === 'object'
        ? nowPlaying.metadata
        : {}

    const titleCandidates = [
      typeof nowPlaying?.title === 'string' ? nowPlaying.title.trim() : '',
      typeof metadata?.title === 'string' ? metadata.title.trim() : '',
    ]
    const trackTitle = titleCandidates.find((value) => value) || ''

    const playlistName =
      typeof activeSchedule?.label === 'string' ? activeSchedule.label.trim() : ''

    if (!trackTitle && !playlistName) {
      return
    }

    if (dislikeRequestControllerRef.current) {
      dislikeRequestControllerRef.current.abort()
    }

    const abortController = new AbortController()
    dislikeRequestControllerRef.current = abortController

    const headers = new Headers({ 'Content-Type': 'application/json' })

    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/dislike`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ trackTitle, playlistName }),
      signal: abortController.signal,
    })
      .catch((err) => {
        if (err?.name !== 'AbortError') {
          // eslint-disable-next-line no-console
          console.error('Failed to send dislike notification', err)
        }
      })
      .finally(() => {
        if (dislikeRequestControllerRef.current === abortController) {
          dislikeRequestControllerRef.current = null
        }
      })
  }, [activeSchedule?.label, device?.nowPlaying, deviceApiId, isApiEnabled])

  const dropdownLabel = activeSchedule ? activeSchedule.label : 'No playlists available'

  useEffect(() => {
    const initialVolume =
      typeof device?.volume === 'number' && !Number.isNaN(device.volume)
        ? device.volume
        : 50
    setIsMuted(Boolean(device?.isMuted) || initialVolume === 0)
    setVolume(initialVolume)
    setDisplayVolume(initialVolume)
    if (initialVolume > 0) {
      previousVolumeRef.current = initialVolume
    }
    volumeSyncReadyRef.current = false
  }, [device])

  useEffect(() => {
    volumeSyncReadyRef.current = false
  }, [deviceApiId, isApiEnabled])

  useEffect(() => {
    if (!canControlDevice) {
      return undefined
    }

    if (typeof volume !== 'number' || Number.isNaN(volume)) {
      return undefined
    }

    if (!deviceApiId) {
      return undefined
    }

    if (!volumeSyncReadyRef.current) {
      volumeSyncReadyRef.current = true
      return undefined
    }

    const abortController = new AbortController()
    const headers = new Headers({ 'Content-Type': 'application/json' })

    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/volume`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ volume }),
      signal: abortController.signal,
    }).catch((err) => {
      if (err?.name !== 'AbortError') {
        // eslint-disable-next-line no-console
        console.error('Failed to update retail player volume', err)
      }
    })

    return () => {
      abortController.abort()
    }
  }, [canControlDevice, deviceApiId, volume])

  const normalizedDeviceTrack = useMemo(() => {
    if (!device?.nowPlaying) {
      return null
    }

    if (typeof device.nowPlaying === 'object' && device.nowPlaying !== null) {
      const nowPlaying = device.nowPlaying
      const normalizedTitle = normalizeValue(nowPlaying.title)
      const normalizedArtist = normalizeValue(nowPlaying.artist)
      const normalizedArtwork = normalizeValue(nowPlaying.artworkUrl)
      const isLoading = Boolean(nowPlaying.isLoading)

      return {
        title:
          normalizedTitle ||
          (isLoading ? 'Loading' : device.channel || nowPlaying.streamName || 'Now Playing'),
        artist:
          normalizedArtist || (isLoading ? '' : device.channel || 'Retail Player'),
        artworkUrl: normalizedArtwork || null,
        isLoading,
      }
    }

    if (typeof device.nowPlaying === 'string') {
      const [titlePart, artistPart] = device.nowPlaying.split('|')
      return {
        title: titlePart ? titlePart.trim() : device.nowPlaying,
        artist: artistPart ? artistPart.trim() : device.channel || 'Retail Player',
        artworkUrl: null,
        isLoading: false,
      }
    }

    return null
  }, [device])

  useEffect(() => {
    if (!normalizedDeviceTrack || normalizedDeviceTrack.isLoading) {
      return
    }

    const nextTrack = {
      title: normalizedDeviceTrack.title || 'Now Playing',
      artist: normalizedDeviceTrack.artist || device?.channel || 'Retail Player',
      artworkUrl: normalizedDeviceTrack.artworkUrl || null,
    }

    setPreviousNowPlaying((previous) => {
      if (
        previous &&
        previous.title === nextTrack.title &&
        previous.artist === nextTrack.artist &&
        previous.artworkUrl === nextTrack.artworkUrl
      ) {
        return previous
      }
      return nextTrack
    })
  }, [device?.channel, normalizedDeviceTrack])

  const effectiveNowPlaying = useMemo(() => {
    if (normalizedDeviceTrack) {
      if (normalizedDeviceTrack.isLoading) {
        if (previousNowPlaying) {
          return previousNowPlaying
        }
        return {
          title: normalizedDeviceTrack.title || 'Loading',
          artist: normalizedDeviceTrack.artist || '',
          artworkUrl: normalizedDeviceTrack.artworkUrl || null,
        }
      }
      return normalizedDeviceTrack
    }

    return previousNowPlaying
  }, [normalizedDeviceTrack, previousNowPlaying])

const trackPool = useMemo(() => {
  return effectiveNowPlaying ? [effectiveNowPlaying] : []
}, [effectiveNowPlaying])

  useEffect(() => {
    setCurrentTrackIndex(0)
  }, [effectiveNowPlaying])

  const currentTrack = useMemo(() => {
    if (!trackPool.length) {
      return { title: 'Now Playing', artist: 'Retail Player', artworkUrl: null }
    }
    const index = ((currentTrackIndex % trackPool.length) + trackPool.length) % trackPool.length
    return trackPool[index]
  }, [currentTrackIndex, trackPool])

  const artworkUrl =
    (effectiveNowPlaying && effectiveNowPlaying.artworkUrl) ||
    normalizeValue(device?.nowPlaying?.artworkUrl) ||
    null
  const resolvedArtworkUrl = artworkUrl || currentTrack?.artworkUrl || null

  const deviceTimeZone = useMemo(() => {
    if (device && typeof device.timeZone === 'string') {
      const trimmed = device.timeZone.trim()
      if (trimmed) {
        return trimmed
      }
    }

    const statusZone =
      device?.status && typeof device.status === 'object' && typeof device.status.timeZone === 'string'
        ? device.status.timeZone.trim()
        : ''

    return statusZone
  }, [device])

  const currentTimeLabel = useMemo(
    () => formatTime(deviceTime, deviceTimeZone || undefined),
    [deviceTime, deviceTimeZone],
  )

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
      { key: 'time', label: currentTimeLabel, labelForAria: 'Time' },
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
  }, [currentTimeLabel, device, isMuted])

  const handleToggleScheduleMenu = useCallback(() => {
    if (!availableSchedulesCount) {
      return
    }
    setScheduleMenuOpen((prev) => !prev)
  }, [availableSchedulesCount])

  const handleSelectChannel = useCallback(
    (schedule) => {
      if (!schedule) {
        return
      }

      const metadata =
        schedule && typeof schedule === 'object' && schedule.metadata && typeof schedule.metadata === 'object'
          ? schedule.metadata
          : {}

      const rawSchedule = schedule && typeof schedule === 'object' && schedule.raw && typeof schedule.raw === 'object'
        ? schedule.raw
        : {}

      const channelIdCandidates = [
        metadata.channelId,
        metadata.channel_id,
        metadata.channel,
        metadata.id,
        schedule.channelId,
        schedule.channel_id,
        schedule.id,
        rawSchedule.id,
        rawSchedule.channelId,
        rawSchedule.channel_id,
      ]

      const selectedChannelId = channelIdCandidates
        .map((candidate) => normalizeValue(candidate))
        .find((value) => value)

      if (!selectedChannelId) {
        // eslint-disable-next-line no-console
        console.error('Unable to determine channel id for selection', schedule)
        return
      }

      if (!canControlDevice || !deviceApiId) {
        return
      }

      if (channelRequestControllerRef.current) {
        channelRequestControllerRef.current.abort()
      }

      const abortController = new AbortController()
      channelRequestControllerRef.current = abortController

      const headers = new Headers({ 'Content-Type': 'application/json' })
      const selectedKey = schedule.key

      setPendingActiveChannelKey(selectedKey)

      httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/channel`, {
        method: 'POST',
        headers,
        body: JSON.stringify({ channel: selectedChannelId }),
        signal: abortController.signal,
      })
        .then(() => {
          refreshStatus()
        })
        .catch((err) => {
          if (err?.name !== 'AbortError') {
            // eslint-disable-next-line no-console
            console.error('Failed to update retail player channel', err)
          }
          setPendingActiveChannelKey(null)
        })
        .finally(() => {
          if (channelRequestControllerRef.current === abortController) {
            channelRequestControllerRef.current = null
          }
        })
    },
    [canControlDevice, deviceApiId, refreshStatus],
  )

  const handleSelectFromDropdown = useCallback(
    (schedule) => {
      setScheduleMenuOpen(false)
      handleSelectChannel(schedule)
    },
    [handleSelectChannel],
  )

  const clearVolumeTimeout = useCallback(() => {
    if (volumeTimeoutRef.current) {
      window.clearTimeout(volumeTimeoutRef.current)
      volumeTimeoutRef.current = null
    }
  }, [])

  const updateVolume = useCallback(
    (nextValue) => {
      setDisplayVolume((previous) => {
        const rawNext = typeof nextValue === 'function' ? nextValue(previous) : nextValue
        const clamped = clamp(Math.round(rawNext), 0, 100)
        clearVolumeTimeout()
        setIsMuted(clamped === 0)
        if (clamped > 0) {
          previousVolumeRef.current = clamped
        }
        volumeTimeoutRef.current = window.setTimeout(() => {
          setVolume(clamped)
          volumeTimeoutRef.current = null
        }, 150)
        return clamped
      })
    },
    [clearVolumeTimeout, setIsMuted],
  )

  const handleToggleMute = useCallback(() => {
    if (isMuted) {
      const restoredVolume =
        previousVolumeRef.current > 0 ? previousVolumeRef.current : 50
      updateVolume(restoredVolume)
      return
    }

    updateVolume((current) => {
      if (current > 0) {
        previousVolumeRef.current = current
      }
      return 0
    })
  }, [isMuted, updateVolume])

  const handleVolumeChange = useCallback((_, newValue) => {
    const resolvedValue = Array.isArray(newValue) ? newValue[0] : newValue
    if (typeof resolvedValue !== 'number' || Number.isNaN(resolvedValue)) {
      return
    }
    updateVolume(resolvedValue)
  }, [updateVolume])

  const handleRefresh = useCallback(() => {
    refreshStatus()
    setDeviceTime(new Date())
  }, [refreshStatus])

  const handleAdjustVolume = useCallback(
    (delta) => {
      updateVolume((prev) => prev + delta)
    },
    [updateVolume],
  )

  const handleDislike = useCallback(() => {
    sendDislikeNotification()
    setShowDislikeMessage(true)
    if (dislikeTimeoutRef.current) {
      window.clearTimeout(dislikeTimeoutRef.current)
    }
    dislikeTimeoutRef.current = window.setTimeout(() => {
      setShowDislikeMessage(false)
      dislikeTimeoutRef.current = null
    }, 2000)
  }, [sendDislikeNotification])

  const handleSkip = useCallback(() => {
    if (!trackPool.length) {
      return
    }

    setCurrentTrackIndex((previous) => (previous + 1) % trackPool.length)

    if (!canControlDevice || !deviceApiId) {
      return
    }

    const activeMetadata =
      activeSchedule && typeof activeSchedule === 'object' && activeSchedule.metadata
        ? activeSchedule.metadata
        : {}

    const channelIdCandidates = [
      normalizeValue(activeMetadata?.channelId),
      normalizeValue(activeMetadata?.channel_id),
      normalizeValue(activeMetadata?.channel),
      normalizeValue(device?.channel),
    ]
    const channelListCandidates = [
      normalizeValue(baseDevice?.channelList),
      normalizeValue(device?.channelList),
    ]

    const resolvedChannelId = channelIdCandidates.find((candidate) => candidate) || ''
    const resolvedChannelListId = channelListCandidates.find((candidate) => candidate) || ''

    if (!resolvedChannelId && !resolvedChannelListId) {
      return
    }

    if (channelRequestControllerRef.current) {
      channelRequestControllerRef.current.abort()
    }

    const abortController = new AbortController()
    channelRequestControllerRef.current = abortController

    const headers = new Headers({ 'Content-Type': 'application/json' })
    const body = {}

    if (resolvedChannelId) {
      body.channel = resolvedChannelId
    }
    if (resolvedChannelListId) {
      body.channelList = resolvedChannelListId
    }

    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceApiId)}/channel/toggle`, {
      method: 'POST',
      headers,
      body: JSON.stringify(body),
      signal: abortController.signal,
    })
      .then(() => {
        refreshStatus()
      })
      .catch((err) => {
        if (err?.name !== 'AbortError') {
          // eslint-disable-next-line no-console
          console.error('Failed to toggle retail player channel', err)
        }
      })
      .finally(() => {
        if (channelRequestControllerRef.current === abortController) {
          channelRequestControllerRef.current = null
        }
      })
  }, [
    activeSchedule,
    baseDevice?.channelList,
    canControlDevice,
    device?.channel,
    device?.channelList,
    deviceApiId,
    refreshStatus,
    trackPool,
  ])

  const handleShortcutChannel = useCallback(
    (index) => {
      const schedule = availableSchedules[index]
      if (!schedule) {
        return
      }
      handleSelectChannel(schedule)
    },
    [availableSchedules, handleSelectChannel],
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

  useEffect(() => () => {
    clearVolumeTimeout()
    if (dislikeTimeoutRef.current) {
      window.clearTimeout(dislikeTimeoutRef.current)
    }
    if (dislikeRequestControllerRef.current) {
      dislikeRequestControllerRef.current.abort()
      dislikeRequestControllerRef.current = null
    }
    if (channelRequestControllerRef.current) {
      channelRequestControllerRef.current.abort()
      channelRequestControllerRef.current = null
    }
  }, [clearVolumeTimeout])

  if (!device) {
    const heading = notFound
      ? 'Device not found'
      : isBusy
        ? 'Loading device…'
        : 'Unable to load device'
    const message = notFound
      ? 'The device you are looking for is unavailable. Choose a device from the list to continue.'
      : isBusy
        ? 'Fetching the latest device details. This will only take a moment.'
        : 'We could not load this device right now. Please refresh and try again.'

    return (
      <div className={classes.notFoundWrapper}>
        <Title title="Retail Player" />
        <Typography component="h1" className={classes.notFoundTitle}>
          {heading}
        </Typography>
        <Typography className={classes.notFoundMessage}>{message}</Typography>
        {combinedError && !isBusy ? (
          <Typography className={classes.notFoundMessage} component="p">
            {combinedError.message || String(combinedError)}
          </Typography>
        ) : null}
      </div>
    )
  }

  return (
    <div className={classes.root}>
      <Title title="Retail Player" />

      <div className={classes.mainContent}>
        <section className={classes.nowPlayingCard} aria-label="Now playing">
          <Typography
            component="h1"
            className={classes.locationLabel}
            noWrap
            title={device.name}
          >
            {device.name}
          </Typography>
          <div className={classes.artworkWrapper} aria-label="Artwork">
            <div className={classes.artworkCircle}>
              <div className={classes.artworkContent}>
                {resolvedArtworkUrl ? (
                  <img
                    src={resolvedArtworkUrl}
                    alt={`Artwork for ${currentTrack.title}`}
                    className={classes.artworkImage}
                  />
                ) : (
                  <svg
                    viewBox="0 0 200 200"
                    className={classes.discSvg}
                    role="img"
                    aria-hidden="true"
                  >
                    <circle cx="100" cy="100" r="98" className={classes.discOuter} />
                    <circle cx="100" cy="100" r="48" className={classes.discInner} />
                    <path
                      d="M150 50c-18-14-40-22-62-20"
                      className={classes.discHighlight}
                    />
                  </svg>
                )}
              </div>
            </div>
          </div>

          <div className={classes.nowPlayingBody}>
            <div className={classes.nowPlayingHeader}>
              <Typography
                component="h2"
                className={classes.nowPlayingTitle}
                noWrap
                title={currentTrack.title}
              >
                {currentTrack.title}
              </Typography>
              <Typography
                className={classes.nowPlayingArtist}
                noWrap
                title={currentTrack.artist}
              >
                {currentTrack.artist}
              </Typography>
            </div>

            <div className={classes.nowPlayingFooter}>
              <section className={classes.controlsRow} aria-label="Now playing controls">
                <ButtonBase
                  className={classes.controlButton}
                  aria-label="Dislike"
                  onClick={handleDislike}
                  focusRipple
                >
                  <span className={classes.controlIcon} role="img" aria-hidden="true">
                    <BiDislike fontSize="inherit" />
                  </span>
                </ButtonBase>
                <ButtonBase
                  className={combineClasses(
                    classes.controlButton,
                    isMuted ? classes.controlButtonMuted : null,
                  )}
                  aria-label="Mute/Unmute"
                  onClick={handleToggleMute}
                  focusRipple
                >
                  <span className={classes.controlIcon} role="img" aria-hidden="true">
                    {isMuted ? <VolumeOffIcon fontSize="inherit" /> : <VolumeUpIcon fontSize="inherit" />}
                  </span>
                </ButtonBase>
                <ButtonBase
                  className={classes.controlButton}
                  aria-label="Skip"
                  onClick={handleSkip}
                  focusRipple
                >
                  <span className={classes.controlIcon} role="img" aria-hidden="true">
                    <MdSkipNext fontSize="inherit" />
                  </span>
                </ButtonBase>
              </section>

              <section className={classes.volumeSection} aria-label="Volume">
                <div className={classes.volumeLabelRow}>
                  <Typography component="span">volume</Typography>
                  <Typography className={classes.volumeValue} aria-live="polite">
                    {displayVolume}
                  </Typography>
                </div>
                <Slider
                  classes={{
                    root: classes.slider,
                    track: classes.sliderTrack,
                    thumb: classes.sliderThumb,
                    rail: classes.sliderRail,
                  }}
                  value={displayVolume}
                  min={0}
                  max={100}
                  aria-label="Volume"
                  onChange={handleVolumeChange}
                />
              </section>
            </div>

            {showDislikeMessage ? (
              <Typography className={classes.dislikeMessage} aria-live="polite">
                Marked as disliked
              </Typography>
            ) : null}
          </div>
        </section>

        <section
          className={classes.list}
          aria-label="Available schedules"
          ref={scheduleDropdownRef}
        >
          <div className={classes.dropdownWrapper}>
            <ButtonBase
              className={combineClasses(
                classes.listItemButton,
                classes.dropdownTriggerButton,
              )}
              onClick={handleToggleScheduleMenu}
              focusRipple
              aria-haspopup="listbox"
              aria-expanded={isScheduleMenuOpen && Boolean(availableSchedulesCount)}
              aria-controls="schedule-menu"
              disabled={!availableSchedulesCount}
            >
              <div className={classes.listItem}>
                <DescriptionIcon
                  className={combineClasses(
                    classes.listIcon,
                    activeSchedule
                      ? classes.playlistIconActive
                      : classes.playlistIconInactive,
                  )}
                  aria-hidden="true"
                />
                <Typography
                  className={combineClasses(
                    classes.listText,
                    classes.playlistLabel,
                    activeSchedule
                      ? classes.playlistLabelActive
                      : classes.playlistLabelInactive,
                  )}
                  noWrap
                >
                  {dropdownLabel}
                </Typography>
                <ExpandMoreIcon
                  className={combineClasses(
                    classes.dropdownCaret,
                    isScheduleMenuOpen ? classes.dropdownCaretOpen : null,
                  )}
                  aria-hidden="true"
                />
              </div>
            </ButtonBase>
            <div
              className={combineClasses(
                classes.dropdownMenu,
                isScheduleMenuOpen ? classes.dropdownMenuOpen : null,
              )}
              role="listbox"
              id="schedule-menu"
              aria-hidden={!isScheduleMenuOpen}
            >
              {availableSchedules.map((schedule) => {
                const isActive = schedule.key === effectiveActiveChannelKey
                return (
                  <ButtonBase
                    key={schedule.key}
                    className={combineClasses(
                      classes.listItemButton,
                      classes.dropdownOptionButton,
                    )}
                    onClick={() => handleSelectFromDropdown(schedule)}
                    focusRipple
                    role="option"
                    aria-selected={isActive}
                  >
                    <div
                      className={combineClasses(
                        classes.listItem,
                        classes.dropdownOptionContent,
                      )}
                    >
                      <DescriptionIcon
                        className={combineClasses(
                          classes.listIcon,
                          isActive
                            ? classes.playlistIconActive
                            : classes.playlistIconInactive,
                        )}
                        aria-hidden="true"
                      />
                      <Typography
                        className={combineClasses(
                          classes.listText,
                          classes.playlistLabel,
                          classes.dropdownOptionLabel,
                          isActive
                            ? classes.playlistLabelActive
                            : classes.playlistLabelInactive,
                        )}
                        noWrap
                      >
                        {schedule.label}
                      </Typography>
                    </div>
                  </ButtonBase>
                )
              })}
            </div>
          </div>
        </section>
      </div>
    </div>
  )
}

export default RetailPlayerDashboard
