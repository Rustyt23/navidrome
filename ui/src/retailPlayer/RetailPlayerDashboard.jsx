import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import { ButtonBase, Slider, Typography } from '@material-ui/core'
import { Title } from 'react-admin'
import LinkIcon from '@material-ui/icons/Link'
import SignalWifi4BarIcon from '@material-ui/icons/SignalWifi4Bar'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import VolumeUpIcon from '@material-ui/icons/VolumeUp'
import DescriptionIcon from '@material-ui/icons/Description'
import GetAppIcon from '@material-ui/icons/GetApp'
import CachedIcon from '@material-ui/icons/Cached'
import { Link as RouterLink, useParams } from 'react-router-dom'
import RetailPlayerMockService from './RetailPlayerMockService'

const combineClasses = (...classNames) => classNames.filter(Boolean).join(' ')

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
  const [device, setDevice] = useState(null)
  const [artworkUrl, setArtworkUrl] = useState(null)
  const [currentTime, setCurrentTime] = useState(() => formatTime(new Date()))

  const refreshDevice = useCallback(() => {
    if (!deviceId) {
      setDevice(null)
      setArtworkUrl(null)
      return
    }

    const nextDevice = RetailPlayerMockService.getDevice(deviceId)
    setDevice(nextDevice)
    setArtworkUrl(RetailPlayerMockService.getArtwork(deviceId))
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

  const isMuted = Boolean(device?.isMuted)
  const volume = typeof device?.volume === 'number' ? device.volume : 0
  const nowPlaying = device?.nowPlaying || ''

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

  const handleToggleMute = useCallback(() => {
    if (!device) {
      return
    }
    RetailPlayerMockService.setMute(deviceId, !device.isMuted)
    refreshDevice()
  }, [device, deviceId, refreshDevice])

  const handleVolumeChange = useCallback(
    (_, newValue) => {
      if (!device) {
        return
      }

      const resolvedValue = Array.isArray(newValue) ? newValue[0] : newValue
      if (typeof resolvedValue !== 'number' || Number.isNaN(resolvedValue)) {
        return
      }

      RetailPlayerMockService.setVolume(deviceId, resolvedValue)
      refreshDevice()
    },
    [device, deviceId, refreshDevice],
  )

  const handleRefresh = useCallback(() => {
    refreshDevice()
  }, [refreshDevice])

  const handleAdjustVolume = useCallback(
    (delta) => {
      if (!device) {
        return
      }
      const nextVolume = device.volume + delta
      RetailPlayerMockService.setVolume(deviceId, nextVolume)
      refreshDevice()
    },
    [device, deviceId, refreshDevice],
  )

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
          RetailPlayerMockService.setMute(deviceId, !device.isMuted)
          refreshDevice()
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
  }, [device, deviceId, handleAdjustVolume, handleShortcutChannel, refreshDevice])

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
        {nowPlaying}
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
