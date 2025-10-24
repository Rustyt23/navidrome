import React from 'react'
import { makeStyles } from '@material-ui/core/styles'
import { Slider, Typography } from '@material-ui/core'
import { Title } from 'react-admin'
import LinkIcon from '@material-ui/icons/Link'
import SignalWifi4BarIcon from '@material-ui/icons/SignalWifi4Bar'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import DescriptionIcon from '@material-ui/icons/Description'
import GetAppIcon from '@material-ui/icons/GetApp'

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
  const sliderMain =
    (theme.palette.secondary && theme.palette.secondary.main) || theme.palette.primary.main

  return {
    root: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(4),
      padding: theme.spacing(4),
      [theme.breakpoints.down('sm')]: {
        padding: theme.spacing(2),
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
    title: {
      fontWeight: theme.typography.fontWeightBold,
      fontSize: theme.typography.pxToRem(48),
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(36),
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
    listItem: {
      display: 'flex',
      alignItems: 'center',
      gap: theme.spacing(2),
      padding: `${theme.spacing(2)}px ${theme.spacing(3)}px`,
      borderBottom: `1px solid ${theme.palette.divider}`,
      '&:last-child': {
        borderBottom: 'none',
      },
    },
    listIcon: {
      color: theme.palette.text.secondary,
      fontSize: theme.typography.pxToRem(24),
    },
    listText: {
      fontSize: theme.typography.pxToRem(20),
      fontWeight: theme.typography.fontWeightMedium,
    },
    listTextDisabled: {
      color: theme.palette.text.disabled,
    },
    nowPlaying: {
      fontSize: theme.typography.pxToRem(40),
      fontWeight: theme.typography.fontWeightBold,
      [theme.breakpoints.down('md')]: {
        fontSize: theme.typography.pxToRem(32),
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
    },
    controlIcon: {
      display: 'inline-flex',
      alignItems: 'center',
      justifyContent: 'center',
      fontSize: theme.typography.pxToRem(64),
    },
    controlIconMuted: {
      color: dangerMain,
    },
    controlIconDownload: {
      color: accentMain,
    },
    volumeControl: {
      display: 'flex',
      flexDirection: 'column',
      flex: 1,
      minWidth: 240,
      gap: theme.spacing(1),
    },
    volumeLabel: {
      textTransform: 'lowercase',
      fontSize: theme.typography.pxToRem(16),
      color: theme.palette.text.secondary,
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
  }
})

const RetailPlayerDashboard = () => {
  const classes = useStyles()

  return (
    <div className={classes.root}>
      <Title title="Retail Player" />
      <header className={classes.header}>
        <Typography component="h1" className={classes.title}>
          ThompsonChicago_Lobby
        </Typography>
        <div className={classes.statusGroup}>
          <span
            className={`${classes.statusIcon} ${classes.statusIconSuccess}`}
            aria-label="Connected"
            role="img"
          >
            <LinkIcon fontSize="inherit" />
          </span>
          <span className={classes.timePill} aria-label="Time">
            16:40
          </span>
          <span
            className={`${classes.statusIcon} ${classes.statusIconSuccess}`}
            aria-label="Signal"
            role="img"
          >
            <SignalWifi4BarIcon fontSize="inherit" />
          </span>
          <span
            className={`${classes.statusIcon} ${classes.statusIconDanger}`}
            aria-label="Muted"
            role="img"
          >
            <VolumeOffIcon fontSize="inherit" />
          </span>
        </div>
      </header>

      <section className={classes.list} aria-label="Available schedules">
        <div className={classes.listItem}>
          <DescriptionIcon className={classes.listIcon} aria-hidden="true" />
          <Typography className={classes.listText}>
            ThompsonChicago_LobbyEarly
          </Typography>
        </div>
        <div className={classes.listItem}>
          <DescriptionIcon className={classes.listIcon} aria-hidden="true" />
          <Typography className={`${classes.listText} ${classes.listTextDisabled}`}>
            ThompsonChicago_LobbyLate
          </Typography>
        </div>
        <div className={classes.listItem}>
          <DescriptionIcon className={classes.listIcon} aria-hidden="true" />
          <Typography className={`${classes.listText} ${classes.listTextDisabled}`}>
            ThompsonChicago_LobbyMid
          </Typography>
        </div>
      </section>

      <Typography component="h2" className={classes.nowPlaying}>
        The Kids | Dog Trainer
      </Typography>

      <section className={classes.controls}>
        <span
          className={`${classes.controlIcon} ${classes.controlIconMuted}`}
          aria-label="Muted"
          role="img"
        >
          <VolumeOffIcon fontSize="inherit" />
        </span>
        <div className={classes.volumeControl}>
          <Typography className={classes.volumeLabel}>volume</Typography>
          <Slider
            classes={{
              root: classes.slider,
              track: classes.sliderTrack,
              thumb: classes.sliderThumb,
              rail: classes.sliderRail,
            }}
            defaultValue={75}
            aria-label="Volume"
          />
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
