import React, { useEffect, useMemo, useState } from 'react'
import {
  Card,
  CardActions,
  CardContent,
  Grid,
  IconButton,
  MenuItem,
  Typography,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { Title } from 'react-admin'
import Select from '@material-ui/core/Select'
import Slider from '@material-ui/core/Slider'
import FiberManualRecordIcon from '@material-ui/icons/FiberManualRecord'
import PlayArrowIcon from '@material-ui/icons/PlayArrow'
import PauseIcon from '@material-ui/icons/Pause'
import SkipNextIcon from '@material-ui/icons/SkipNext'
import SkipPreviousIcon from '@material-ui/icons/SkipPrevious'

const useStyles = makeStyles((theme) => ({
  root: {
    marginTop: theme.spacing(2),
  },
  card: {
    height: '100%',
  },
  header: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: theme.spacing(1),
  },
  status: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color: theme.palette.text.secondary,
  },
  statusIconOnline: {
    color: theme.palette.success.main,
  },
  statusIconOffline: {
    color: theme.palette.action.disabled,
  },
  controls: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    width: '100%',
  },
  channelSelect: {
    minWidth: 160,
  },
  slider: {
    flex: 1,
    marginLeft: theme.spacing(2),
    marginRight: theme.spacing(2),
  },
}))

const createMockService = () => {
  const channels = [
    { id: 'channel-1', name: 'Main Floor' },
    { id: 'channel-2', name: 'Outdoor Patio' },
    { id: 'channel-3', name: 'Seasonal Playlist' },
    { id: 'channel-4', name: 'Announcements' },
  ]

  const devices = [
    {
      id: 'device-1',
      name: 'Lobby Speaker',
      online: true,
      channelId: channels[0].id,
      volume: 65,
    },
    {
      id: 'device-2',
      name: 'Warehouse Receiver',
      online: false,
      channelId: channels[1].id,
      volume: 30,
    },
  ]

  return {
    getDevices: () => Promise.resolve(devices.map((device) => ({ ...device }))),
    getChannels: () => Promise.resolve(channels.map((channel) => ({ ...channel }))),
    setDeviceChannel: (deviceId, channelId) => {
      console.log(`[RetailPlayer] setDeviceChannel`, { deviceId, channelId })
    },
    setDeviceVolume: (deviceId, volume) => {
      console.log(`[RetailPlayer] setDeviceVolume`, { deviceId, volume })
    },
    controlPlayback: (deviceId, action) => {
      console.log(`[RetailPlayer] controlPlayback`, { deviceId, action })
    },
  }
}

const mockService = createMockService()

const RetailPlayerDashboard = () => {
  const classes = useStyles()
  const [devices, setDevices] = useState([])
  const [channels, setChannels] = useState([])

  useEffect(() => {
    let isMounted = true
    const loadData = async () => {
      const [deviceList, channelList] = await Promise.all([
        mockService.getDevices(),
        mockService.getChannels(),
      ])
      if (isMounted) {
        setDevices(deviceList)
        setChannels(channelList)
      }
    }
    loadData()
    return () => {
      isMounted = false
    }
  }, [])

  const channelMap = useMemo(
    () =>
      channels.reduce((acc, channel) => {
        acc[channel.id] = channel
        return acc
      }, {}),
    [channels],
  )

  const handleChannelChange = (deviceId) => (event) => {
    const channelId = event.target.value
    setDevices((prevDevices) =>
      prevDevices.map((device) =>
        device.id === deviceId ? { ...device, channelId } : device,
      ),
    )
    mockService.setDeviceChannel(deviceId, channelId)
  }

  const handleVolumeChange = (deviceId) => (event, value) => {
    const volume = Array.isArray(value) ? value[0] : value
    setDevices((prevDevices) =>
      prevDevices.map((device) =>
        device.id === deviceId ? { ...device, volume } : device,
      ),
    )
    mockService.setDeviceVolume(deviceId, volume)
  }

  const handlePlayback = (deviceId, action) => () => {
    mockService.controlPlayback(deviceId, action)
  }

  const renderDevice = (device) => {
    const isOnline = device.online
    const nowPlaying = channelMap[device.channelId]?.name || '—'
    return (
      <Grid item xs={12} md={6} key={device.id}>
        <Card className={classes.card}>
          <CardContent>
            <div className={classes.header}>
              <Typography variant="h6">{device.name}</Typography>
              <div className={classes.status}>
                <FiberManualRecordIcon
                  fontSize="small"
                  className={
                    isOnline ? classes.statusIconOnline : classes.statusIconOffline
                  }
                />
                <Typography variant="body2">
                  {isOnline ? 'Online' : 'Offline'}
                </Typography>
              </div>
            </div>
            <Typography variant="subtitle2" color="textSecondary">
              Now Playing
            </Typography>
            <Typography variant="body1" gutterBottom>
              {nowPlaying}
            </Typography>
            <div className={classes.controls}>
              <Select
                value={device.channelId}
                onChange={handleChannelChange(device.id)}
                className={classes.channelSelect}
                disabled={!isOnline}
                variant="outlined"
              >
                {channels.map((channel) => (
                  <MenuItem value={channel.id} key={channel.id}>
                    {channel.name}
                  </MenuItem>
                ))}
              </Select>
              <Slider
                value={device.volume}
                onChange={handleVolumeChange(device.id)}
                aria-labelledby={`${device.id}-volume`}
                step={1}
                min={0}
                max={100}
                className={classes.slider}
                disabled={!isOnline}
              />
              <Typography variant="body2">{device.volume}</Typography>
            </div>
          </CardContent>
          <CardActions>
            <IconButton
              aria-label="previous"
              onClick={handlePlayback(device.id, 'previous')}
              disabled={!isOnline}
            >
              <SkipPreviousIcon />
            </IconButton>
            <IconButton
              aria-label="play"
              onClick={handlePlayback(device.id, 'play')}
              disabled={!isOnline}
            >
              <PlayArrowIcon />
            </IconButton>
            <IconButton
              aria-label="pause"
              onClick={handlePlayback(device.id, 'pause')}
              disabled={!isOnline}
            >
              <PauseIcon />
            </IconButton>
            <IconButton
              aria-label="next"
              onClick={handlePlayback(device.id, 'next')}
              disabled={!isOnline}
            >
              <SkipNextIcon />
            </IconButton>
          </CardActions>
        </Card>
      </Grid>
    )
  }

  return (
    <div className={classes.root}>
      <Title title="Retail Player" />
      <Grid container spacing={2}>
        {devices.map(renderDevice)}
      </Grid>
    </div>
  )
}

export default RetailPlayerDashboard
