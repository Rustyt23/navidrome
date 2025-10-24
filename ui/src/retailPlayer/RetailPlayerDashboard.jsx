import React from 'react'
import { Box, Card, Divider, Slider, Typography } from '@material-ui/core'
import { makeStyles, useTheme } from '@material-ui/core/styles'
import { Title } from 'react-admin'
import LinkIcon from '@material-ui/icons/Link'
import WifiIcon from '@material-ui/icons/Wifi'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import DescriptionIcon from '@material-ui/icons/Description'
import GetAppIcon from '@material-ui/icons/GetApp'
import clsx from 'clsx'

const RETAIL_PLAYER_DATA = {
  title: 'ThompsonChicago_Lobby',
  statuses: [
    { id: 'connection', type: 'icon', icon: LinkIcon, color: 'success.main' },
    { id: 'time', type: 'pill', label: '16:40', color: 'success.main' },
    { id: 'signal', type: 'icon', icon: WifiIcon, color: 'success.main' },
    { id: 'muted', type: 'icon', icon: VolumeOffIcon, color: 'error.main' },
  ],
  schedules: [
    { id: 'early', label: 'ThompsonChicago_LobbyEarly', active: true },
    { id: 'late', label: 'ThompsonChicago_LobbyLate', active: false },
    { id: 'mid', label: 'ThompsonChicago_LobbyMid', active: false },
  ],
  nowPlaying: 'The Kids | Dog Trainer',
  volume: 75,
}

const useStyles = makeStyles((theme) => ({
  root: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(5),
    padding: theme.spacing(4, 6, 6),
    [theme.breakpoints.down('sm')]: {
      padding: theme.spacing(3, 2, 4),
      gap: theme.spacing(4),
    },
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    flexWrap: 'wrap',
    gap: theme.spacing(2.5),
  },
  title: {
    fontWeight: 600,
  },
  statuses: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
  },
  statusIcon: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: theme.spacing(4.5),
    height: theme.spacing(4.5),
  },
  timePill: {
    padding: theme.spacing(0.5, 2),
    borderRadius: theme.spacing(2),
    fontWeight: 600,
    textTransform: 'uppercase',
  },
  listCard: {
    backgroundColor: theme.palette.background.paper,
    overflow: 'hidden',
  },
  scheduleCard: {
    padding: theme.spacing(1.75, 3),
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
    transition: theme.transitions.create(['background-color', 'border-color'], {
      duration: theme.transitions.duration.shorter,
    }),
  },
  scheduleActive: {
    backgroundColor: theme.palette.action.selected,
    borderLeft: `4px solid ${theme.palette.secondary.main}`,
  },
  scheduleDisabled: {
    opacity: 0.4,
  },
  scheduleLabel: {
    flex: 1,
    fontWeight: 500,
  },
  nowPlayingCard: {
    padding: theme.spacing(2.5, 3),
    backgroundColor: theme.palette.background.paper,
  },
  nowPlayingText: {
    fontWeight: 600,
    letterSpacing: '0.02em',
  },
  controlBar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(4),
    flexWrap: 'wrap',
    padding: theme.spacing(2.5, 3),
    backgroundColor: theme.palette.background.paper,
  },
  controlIcon: {
    fontSize: '3rem',
  },
  mutedIcon: {
    color: theme.palette.error.main,
  },
  downloadIcon: {
    color: theme.palette.secondary.main,
  },
  volumeSection: {
    display: 'flex',
    flexDirection: 'column',
    flex: 1,
    minWidth: theme.spacing(32),
    gap: theme.spacing(1),
  },
  sliderRoot: {
    padding: theme.spacing(0.5, 1, 0),
  },
}))

const RetailPlayerDashboard = () => {
  const classes = useStyles()
  const theme = useTheme()
  const { title, statuses, schedules, nowPlaying, volume } = RETAIL_PLAYER_DATA

  const resolveColor = (token) => {
    if (!token) {
      return undefined
    }

    const [paletteKey, shadeKey] = token.split('.')
    const palette = theme.palette[paletteKey]
    if (!palette) {
      return undefined
    }

    if (shadeKey && palette[shadeKey]) {
      return palette[shadeKey]
    }

    return palette.main ?? palette
  }

  return (
    <Box className={classes.root}>
      <Title title="Retail Player" />

      <Box className={classes.header}>
        <Typography variant="h3" className={classes.title}>
          {title}
        </Typography>

        <Box className={classes.statuses}>
          {statuses.map((status) => {
            const resolvedColor = resolveColor(status.color)

            if (status.type === 'pill') {
              return (
                <Box
                  key={status.id}
                  className={classes.timePill}
                  style={{
                    backgroundColor: resolvedColor,
                    color: resolvedColor
                      ? theme.palette.getContrastText(resolvedColor)
                      : undefined,
                  }}
                >
                  <Typography component="span">{status.label}</Typography>
                </Box>
              )
            }

            const IconComponent = status.icon
            return (
              <Box
                key={status.id}
                className={classes.statusIcon}
                style={{
                  color: resolvedColor,
                }}
              >
                <IconComponent style={{ color: resolvedColor }} />
              </Box>
            )
          })}
        </Box>
      </Box>

      <Card className={classes.listCard} elevation={4}>
        {schedules.map((schedule, index) => {
          const scheduleClasses = clsx(classes.scheduleCard, {
            [classes.scheduleActive]: schedule.active,
            [classes.scheduleDisabled]: !schedule.active,
          })

          return (
            <React.Fragment key={schedule.id}>
              {index > 0 && <Divider />}
              <Box className={scheduleClasses}>
                <DescriptionIcon color={schedule.active ? 'inherit' : 'disabled'} />
                <Typography
                  variant="h6"
                  className={classes.scheduleLabel}
                  color={schedule.active ? 'textPrimary' : 'textSecondary'}
                >
                  {schedule.label}
                </Typography>
              </Box>
            </React.Fragment>
          )
        })}
      </Card>

      <Card className={classes.nowPlayingCard} elevation={4}>
        <Typography variant="h2" className={classes.nowPlayingText}>
          {nowPlaying}
        </Typography>
      </Card>

      <Card className={classes.controlBar} elevation={4}>
        <VolumeOffIcon className={`${classes.controlIcon} ${classes.mutedIcon}`} />

        <Box className={classes.volumeSection}>
          <Typography variant="subtitle1">volume</Typography>
          <Slider
            value={volume}
            min={0}
            max={100}
            classes={{ root: classes.sliderRoot }}
            color="secondary"
            aria-label="volume"
          />
        </Box>

        <GetAppIcon className={`${classes.controlIcon} ${classes.downloadIcon}`} />
      </Card>
    </Box>
  )
}

export default RetailPlayerDashboard
