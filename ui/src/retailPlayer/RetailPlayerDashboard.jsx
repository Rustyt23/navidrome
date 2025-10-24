import React from 'react'
import {
  Box,
  Divider,
  Grid,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Typography,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { Title } from 'react-admin'
import Slider from '@material-ui/core/Slider'
import FiberManualRecordIcon from '@material-ui/icons/FiberManualRecord'
import SignalCellularAltIcon from '@material-ui/icons/SignalCellularAlt'
import WifiIcon from '@material-ui/icons/Wifi'
import BatteryFullIcon from '@material-ui/icons/BatteryFull'
import MicOffIcon from '@material-ui/icons/MicOff'
import GetAppIcon from '@material-ui/icons/GetApp'

const useStyles = makeStyles((theme) => ({
  root: {
    padding: theme.spacing(4),
    backgroundColor: theme.palette.background.default,
    color: theme.palette.text.primary,
    minHeight: '100vh',
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: theme.spacing(4),
  },
  headerTitle: {
    fontSize: '1.75rem',
    fontWeight: 600,
  },
  headerIcons: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    color: theme.palette.success.main,
  },
  headerIcon: {
    fontSize: '1.75rem',
  },
  content: {
    marginTop: theme.spacing(2),
  },
  channelList: {
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius,
    paddingTop: theme.spacing(1),
    paddingBottom: theme.spacing(1),
  },
  listItem: {
    paddingTop: theme.spacing(1),
    paddingBottom: theme.spacing(1),
  },
  listItemIcon: {
    minWidth: theme.spacing(4),
  },
  channelIcon: {
    fontSize: '0.75rem',
    color: theme.palette.text.secondary,
  },
  listItemText: {
    '& .MuiTypography-root': {
      fontSize: '0.95rem',
    },
  },
  playerPanel: {
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius,
    padding: theme.spacing(4),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
    height: '100%',
  },
  playerHeader: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(3),
  },
  playerStatusIcon: {
    fontSize: '4rem',
    color: theme.palette.error.main,
  },
  trackInfo: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(0.5),
  },
  trackTitle: {
    fontSize: '1.5rem',
    fontWeight: 500,
  },
  trackSubtitle: {
    fontSize: '0.9rem',
    color: theme.palette.text.secondary,
  },
  volumeSection: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(2),
  },
  volumeLabel: {
    textTransform: 'lowercase',
    fontSize: '0.95rem',
    color: theme.palette.text.secondary,
  },
  sliderRow: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(2),
  },
  slider: {
    flex: 1,
  },
  sliderValue: {
    minWidth: 32,
    textAlign: 'right',
    fontWeight: 600,
  },
  downloadIcon: {
    alignSelf: 'flex-end',
    fontSize: '3rem',
    color: theme.palette.info.main,
  },
}))

const channelNames = [
  'ThompsonChicago_LobbyEarly',
  'ThompsonChicago_LobbyLate',
  'ThompsonChicago_LobbyMid',
]

const RetailPlayerDashboard = () => {
  const classes = useStyles()
  const volumeValue = 40

  return (
    <Box className={classes.root}>
      <Title title="Retail Player" />
      <div className={classes.header}>
        <Typography className={classes.headerTitle}>
          ThompsonChicago_Lobby
        </Typography>
        <div className={classes.headerIcons}>
          <SignalCellularAltIcon className={classes.headerIcon} />
          <WifiIcon className={classes.headerIcon} />
          <BatteryFullIcon className={classes.headerIcon} />
        </div>
      </div>
      <Grid container spacing={4} className={classes.content}>
        <Grid item xs={12} md={4}>
          <List disablePadding className={classes.channelList}>
            {channelNames.map((name) => (
              <ListItem key={name} className={classes.listItem}>
                <ListItemIcon className={classes.listItemIcon}>
                  <FiberManualRecordIcon className={classes.channelIcon} />
                </ListItemIcon>
                <ListItemText primary={name} className={classes.listItemText} />
              </ListItem>
            ))}
          </List>
        </Grid>
        <Grid item xs={12} md={8}>
          <Box className={classes.playerPanel}>
            <div className={classes.playerHeader}>
              <MicOffIcon className={classes.playerStatusIcon} />
              <div className={classes.trackInfo}>
                <Typography className={classes.trackTitle}>
                  The Kids | Dog Trainer
                </Typography>
              </div>
            </div>
            <Divider />
            <div className={classes.volumeSection}>
              <Typography className={classes.volumeLabel}>volume</Typography>
              <div className={classes.sliderRow}>
                <Slider
                  value={volumeValue}
                  disabled
                  className={classes.slider}
                  aria-label="volume"
                />
                <Typography className={classes.sliderValue}>
                  {volumeValue}
                </Typography>
              </div>
            </div>
            <GetAppIcon className={classes.downloadIcon} />
          </Box>
        </Grid>
      </Grid>
    </Box>
  )
}

export default RetailPlayerDashboard
