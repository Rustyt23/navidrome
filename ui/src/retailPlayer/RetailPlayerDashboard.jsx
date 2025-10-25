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
import { Link as RouterLink, useParams } from 'react-router-dom'
import { BiDislike } from 'react-icons/bi'
import { MdSkipNext } from 'react-icons/md'
import RetailPlayerMockService from './RetailPlayerMockService'

const combineClasses = (...classNames) => classNames.filter(Boolean).join(' ')

const clamp = (value, min, max) => Math.min(Math.max(value, min), max)

const formatTime = (date) =>
  date
    .toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    })
    .replace(/^24:/, '00:')

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
    list: {
      borderRadius: theme.shape.borderRadius,
      backgroundColor: theme.palette.background.paper,
      overflow: 'hidden',
      border: `1px solid ${theme.palette.divider}`,
    },
    contentGrid: {
      display: 'grid',
      gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)',
      gap: theme.spacing(5),
      alignItems: 'flex-start',
      width: '100%',
      [theme.breakpoints.down('md')]: {
        gridTemplateColumns: 'minmax(0, 1fr)',
      },
    },
    nowPlayingCard: {
      borderRadius: theme.shape.borderRadius,
      backgroundColor: theme.palette.background.paper,
      border: `1px solid ${theme.palette.divider}`,
      padding: theme.spacing(4),
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: theme.spacing(3),
      [theme.breakpoints.down('sm')]: {
        padding: theme.spacing(3),
        gap: theme.spacing(2.5),
      },
    },
    nowPlayingStack: {
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      gap: theme.spacing(2),
      width: '100%',
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
      width: 'min(180px, 100%)',
      maxWidth: 200,
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
      fontSize: theme.typography.pxToRem(36),
      fontWeight: theme.typography.fontWeightBold,
      textAlign: 'center',
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(30),
      },
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(22),
      },
    },
    nowPlayingArtist: {
      fontSize: theme.typography.pxToRem(18),
      textAlign: 'center',
      color: theme.palette.text.secondary,
      [theme.breakpoints.down('sm')]: {
        fontSize: theme.typography.pxToRem(15),
      },
    },
    controlsRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: theme.spacing(4),
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
      alignSelf: 'center',
    },
    volumeLabelRow: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'space-between',
      color: theme.palette.text.secondary,
      textTransform: 'lowercase',
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
    dislikeMessage: {
      marginTop: theme.spacing(1),
      textAlign: 'center',
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(14),
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
  }
})

const dummyTracks = [
  {
    title: 'Neon Skyline',
    artist: 'City Echo',
    artworkUrl: null,
  },
  {
    title: 'Golden Hours',
    artist: 'Harbor Lights',
    artworkUrl:
      'https://images.unsplash.com/photo-1526285840434-67ff3760c121?auto=format&fit=crop&w=400&q=80',
  },
  {
    title: 'Velvet Static',
    artist: 'Analog Dream',
    artworkUrl: null,
  },
]

const RetailPlayerDashboard = () => {
  const classes = useStyles()
  const { deviceId } = useParams()
  const [device, setDevice] = useState(null)
  const [currentTime, setCurrentTime] = useState(() => formatTime(new Date()))
  const [isMuted, setIsMuted] = useState(false)
  const [, setVolume] = useState(50)
  const [displayVolume, setDisplayVolume] = useState(50)
  const volumeTimeoutRef = useRef(null)
  const dislikeTimeoutRef = useRef(null)
  const [showDislikeMessage, setShowDislikeMessage] = useState(false)
  const [currentTrackIndex, setCurrentTrackIndex] = useState(0)

  const refreshDevice = useCallback(() => {
    if (!deviceId) {
      setDevice(null)
      return
    }

    const nextDevice = RetailPlayerMockService.getDevice(deviceId)
    setDevice(nextDevice)
    setCurrentTime(formatTime(new Date()))
  }, [deviceId])

  useEffect(() => {
    refreshDevice()
  }, [refreshDevice])

  useEffect(() => {
    const updateTime = () => setCurrentTime(formatTime(new Date()))
    const intervalId = window.setInterval(updateTime, 60000)
    return () => window.clearInterval(intervalId)
  }, [])

  const schedules = useMemo(() => device?.schedules || [], [device])

  const activeChannelKey = useMemo(() => {
    const activeSchedule = schedules.find((schedule) => schedule.isActive)
    return activeSchedule ? activeSchedule.key : null
  }, [schedules])

  useEffect(() => {
    setIsMuted(Boolean(device?.isMuted))
    const initialVolume =
      typeof device?.volume === 'number' && !Number.isNaN(device.volume)
        ? device.volume
        : 50
    setVolume(initialVolume)
    setDisplayVolume(initialVolume)
  }, [device])

  const normalizedDeviceTrack = useMemo(() => {
    if (!device?.nowPlaying) {
      return null
    }
    if (typeof device.nowPlaying === 'object' && device.nowPlaying !== null) {
      return {
        title: device.nowPlaying.title || 'Now Playing',
        artist: device.nowPlaying.artist || device.channel || 'Retail Player',
        artworkUrl: device.nowPlaying.artworkUrl || null,
      }
    }
    if (typeof device.nowPlaying === 'string') {
      const [titlePart, artistPart] = device.nowPlaying.split('|')
      return {
        title: titlePart ? titlePart.trim() : device.nowPlaying,
        artist: artistPart ? artistPart.trim() : device.channel || 'Retail Player',
        artworkUrl: null,
      }
    }
    return null
  }, [device])

  const trackPool = useMemo(() => {
    if (normalizedDeviceTrack) {
      return [normalizedDeviceTrack, ...dummyTracks]
    }
    return dummyTracks
  }, [normalizedDeviceTrack])

  useEffect(() => {
    setCurrentTrackIndex(0)
  }, [normalizedDeviceTrack])

  const currentTrack = useMemo(() => {
    if (!trackPool.length) {
      return { title: 'Now Playing', artist: 'Retail Player', artworkUrl: null }
    }
    const index = ((currentTrackIndex % trackPool.length) + trackPool.length) % trackPool.length
    return trackPool[index]
  }, [currentTrackIndex, trackPool])

  const artworkUrl = device?.nowPlaying?.artworkUrl || null
  const resolvedArtworkUrl = artworkUrl || currentTrack?.artworkUrl || null

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

  const handleSelectChannel = useCallback(
    (schedule) => {
      if (!device) {
        return
      }
      RetailPlayerMockService.setActiveChannel(deviceId, schedule.key)
      refreshDevice()
    },
    [device, deviceId, refreshDevice],
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
        volumeTimeoutRef.current = window.setTimeout(() => {
          setVolume(clamped)
          volumeTimeoutRef.current = null
        }, 150)
        return clamped
      })
    },
    [clearVolumeTimeout],
  )

  const handleToggleMute = useCallback(() => {
    setIsMuted((prev) => !prev)
  }, [])

  const handleVolumeChange = useCallback((_, newValue) => {
    const resolvedValue = Array.isArray(newValue) ? newValue[0] : newValue
    if (typeof resolvedValue !== 'number' || Number.isNaN(resolvedValue)) {
      return
    }
    updateVolume(resolvedValue)
  }, [updateVolume])

  const handleRefresh = useCallback(() => {
    refreshDevice()
  }, [refreshDevice])

  const handleAdjustVolume = useCallback(
    (delta) => {
      updateVolume((prev) => prev + delta)
    },
    [updateVolume],
  )

  const handleDislike = useCallback(() => {
    setShowDislikeMessage(true)
    if (dislikeTimeoutRef.current) {
      window.clearTimeout(dislikeTimeoutRef.current)
    }
    dislikeTimeoutRef.current = window.setTimeout(() => {
      setShowDislikeMessage(false)
      dislikeTimeoutRef.current = null
    }, 2000)
  }, [])

  const handleSkip = useCallback(() => {
    if (!trackPool.length) {
      return
    }
    setCurrentTrackIndex((previous) => (previous + 1) % trackPool.length)
  }, [trackPool])

  const handleShortcutChannel = useCallback(
    (index) => {
      const schedule = schedules[index]
      if (!schedule) {
        return
      }
      RetailPlayerMockService.setActiveChannel(deviceId, schedule.key)
      refreshDevice()
    },
    [deviceId, refreshDevice, schedules],
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
  }, [clearVolumeTimeout])

  if (!device) {
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

      <div className={classes.contentGrid}>
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

        <section className={classes.nowPlayingCard} aria-label="Now playing">
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

          <div className={classes.nowPlayingStack}>
            <Typography component="h2" className={classes.nowPlayingTitle}>
              {currentTrack.title}
            </Typography>
            <Typography className={classes.nowPlayingArtist}>{currentTrack.artist}</Typography>
          </div>

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

          {showDislikeMessage ? (
            <Typography className={classes.dislikeMessage} aria-live="polite">
              Marked as disliked
            </Typography>
          ) : null}

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
        </section>
      </div>
    </div>
  )
}

export default RetailPlayerDashboard
