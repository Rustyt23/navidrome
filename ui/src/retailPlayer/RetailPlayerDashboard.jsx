import React, { useEffect, useMemo, useState } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import {
  Button,
  Card,
  CardContent,
  CircularProgress,
  Divider,
  List,
  ListItem,
  ListItemText,
  Slider,
  Typography,
} from '@material-ui/core'
import { Title } from 'react-admin'
import { useParams } from 'react-router-dom'
import RefreshIcon from '@material-ui/icons/Refresh'
import SwapHorizIcon from '@material-ui/icons/SwapHoriz'
import ThumbDownIcon from '@material-ui/icons/ThumbDown'
import useRetailPlayerDeviceStatus from './useRetailPlayerDeviceStatus'
import { normalizeValue } from './deviceUtils'
import httpClient from '../dataProvider/httpClient'

const useStyles = makeStyles((theme) => ({
  root: {
    maxWidth: 960,
    margin: '0 auto',
    padding: theme.spacing(4),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
  },
  header: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    flexWrap: 'wrap',
    gap: theme.spacing(2),
  },
  headerActions: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(1),
  },
  card: {
    width: '100%',
  },
  helperText: {
    marginTop: theme.spacing(1),
    color: theme.palette.text.secondary,
  },
  error: {
    color: theme.palette.error.main,
  },
  feedback: {
    color: theme.palette.success.main,
  },
  loading: {
    display: 'flex',
    justifyContent: 'center',
    padding: theme.spacing(4),
  },
  list: {
    marginTop: theme.spacing(1),
  },
}))

const RetailPlayerDashboard = () => {
  const classes = useStyles()
  const { slug } = useParams()
  const {
    device,
    baseDevice,
    refresh,
    isLoading,
    isStatusLoading,
    isChannelListLoading,
    error,
    notFound,
    isApiEnabled,
    lastUpdated,
  } = useRetailPlayerDeviceStatus(slug)

  const deviceName = device?.name || baseDevice?.name || 'Retail Player'
  const deviceIdentifier = useMemo(
    () =>
      normalizeValue(device?.apiId) ||
      normalizeValue(device?.id) ||
      normalizeValue(baseDevice?.apiId) ||
      normalizeValue(baseDevice?.id) ||
      '',
    [baseDevice?.apiId, baseDevice?.id, device?.apiId, device?.id],
  )

  const canControlDevice = Boolean(deviceIdentifier && isApiEnabled)
  const schedules = device?.schedules || []
  const nowPlaying = device?.nowPlaying || {
    title: 'Now Playing',
    artist: 'Retail Player',
  }

  const [localVolume, setLocalVolume] = useState(device?.volume ?? 50)
  const [feedback, setFeedback] = useState('')
  const [isUpdatingVolume, setIsUpdatingVolume] = useState(false)
  const [isChangingChannel, setIsChangingChannel] = useState(false)
  const [isTogglingChannel, setIsTogglingChannel] = useState(false)
  const [isDisliking, setIsDisliking] = useState(false)

  useEffect(() => {
    setLocalVolume(device?.volume ?? 50)
  }, [device?.volume])

  const clearFeedback = () => setFeedback('')

  const handleRefresh = () => {
    clearFeedback()
    refresh()
  }

  const handleSliderChange = (_, newValue) => {
    const value = Array.isArray(newValue) ? newValue[0] : newValue
    if (typeof value === 'number' && !Number.isNaN(value)) {
      setLocalVolume(value)
    }
  }

  const handleVolumeCommit = (_, newValue) => {
    if (!canControlDevice) {
      return
    }

    const value = Array.isArray(newValue) ? newValue[0] : newValue
    if (typeof value !== 'number' || Number.isNaN(value)) {
      return
    }

    if (value === device?.volume) {
      return
    }

    setIsUpdatingVolume(true)
    clearFeedback()

    const headers = new Headers({ 'Content-Type': 'application/json' })
    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceIdentifier)}/volume`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ volume: value }),
    })
      .then(() => {
        setFeedback('Volume updated')
        refresh()
      })
      .catch((err) => {
        const message = err?.message ? `Unable to update volume: ${err.message}` : 'Unable to update volume'
        setFeedback(message)
      })
      .finally(() => {
        setIsUpdatingVolume(false)
      })
  }

  const handleSelectSchedule = (schedule) => {
    if (!canControlDevice || !schedule?.channelId) {
      return
    }

    setIsChangingChannel(true)
    clearFeedback()

    const headers = new Headers({ 'Content-Type': 'application/json' })
    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceIdentifier)}/channel`, {
      method: 'POST',
      headers,
      body: JSON.stringify({ channel: schedule.channelId }),
    })
      .then(() => {
        setFeedback(`Switched to ${schedule.label}`)
        refresh()
      })
      .catch((err) => {
        const message = err?.message
          ? `Unable to change channel: ${err.message}`
          : 'Unable to change channel'
        setFeedback(message)
      })
      .finally(() => {
        setIsChangingChannel(false)
      })
  }

  const handleToggleChannel = () => {
    if (!canControlDevice) {
      return
    }

    setIsTogglingChannel(true)
    clearFeedback()

    const payload = {}
    const currentChannel = normalizeValue(device?.channel)
    const channelListId =
      normalizeValue(device?.channelList) || normalizeValue(baseDevice?.channelList)

    if (currentChannel) {
      payload.channel = currentChannel
    }
    if (channelListId) {
      payload.channelList = channelListId
    }

    const headers = new Headers({ 'Content-Type': 'application/json' })
    const options = { method: 'POST', headers }
    if (Object.keys(payload).length) {
      options.body = JSON.stringify(payload)
    }

    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceIdentifier)}/channel/toggle`, options)
      .then(() => {
        setFeedback('Requested alternate channel')
        refresh()
      })
      .catch((err) => {
        const message = err?.message
          ? `Unable to toggle channel: ${err.message}`
          : 'Unable to toggle channel'
        setFeedback(message)
      })
      .finally(() => {
        setIsTogglingChannel(false)
      })
  }

  const handleDislike = () => {
    if (!canControlDevice) {
      return
    }

    setIsDisliking(true)
    clearFeedback()

    httpClient(`/api/retailplayer/devices/${encodeURIComponent(deviceIdentifier)}/dislike`, {
      method: 'POST',
    })
      .then(() => {
        setFeedback('Marked current track as disliked')
      })
      .catch((err) => {
        const message = err?.message
          ? `Unable to send dislike: ${err.message}`
          : 'Unable to send dislike'
        setFeedback(message)
      })
      .finally(() => {
        setIsDisliking(false)
      })
  }

  const lastUpdatedLabel = useMemo(() => {
    if (!(lastUpdated instanceof Date) || Number.isNaN(lastUpdated.getTime())) {
      return ''
    }
    return lastUpdated.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
  }, [lastUpdated])

  const actionDisabled = isStatusLoading || isChannelListLoading

  return (
    <div className={classes.root}>
      <Title title="Retail Player" />
      <header className={classes.header}>
        <Typography component="h1" variant="h4">
          {deviceName}
        </Typography>
        <div className={classes.headerActions}>
          <Button
            variant="outlined"
            startIcon={<RefreshIcon />}
            onClick={handleRefresh}
            disabled={actionDisabled}
          >
            Refresh
          </Button>
          <Button
            variant="outlined"
            startIcon={<SwapHorizIcon />}
            onClick={handleToggleChannel}
            disabled={!canControlDevice || isTogglingChannel}
          >
            Toggle channel
          </Button>
          <Button
            variant="outlined"
            startIcon={<ThumbDownIcon />}
            onClick={handleDislike}
            disabled={!canControlDevice || isDisliking}
          >
            Dislike
          </Button>
        </div>
      </header>

      {!isApiEnabled ? (
        <Typography className={classes.helperText}>
          Retail player integration is disabled.
        </Typography>
      ) : null}

      {isLoading ? (
        <div className={classes.loading}>
          <CircularProgress size={32} />
        </div>
      ) : null}

      {notFound ? (
        <Typography className={classes.error}>
          We could not find this device.
        </Typography>
      ) : null}

      {error ? (
        <Typography className={classes.error}>
          {error.message || 'An unexpected error occurred while loading the device.'}
        </Typography>
      ) : null}

      {feedback ? <Typography className={classes.feedback}>{feedback}</Typography> : null}

      {device ? (
        <>
          <Card className={classes.card}>
            <CardContent>
              <Typography variant="h6">Now playing</Typography>
              <Typography variant="subtitle1">{nowPlaying.title}</Typography>
              <Typography color="textSecondary">{nowPlaying.artist}</Typography>
              <Divider className={classes.helperText} />
              <Typography className={classes.helperText}>
                Volume: {localVolume}%
              </Typography>
              <Slider
                value={localVolume}
                onChange={handleSliderChange}
                onChangeCommitted={handleVolumeCommit}
                min={0}
                max={100}
                disabled={!canControlDevice || isUpdatingVolume}
                aria-label="Volume"
              />
            </CardContent>
          </Card>

          <Card className={classes.card}>
            <CardContent>
              <Typography variant="h6">Schedules</Typography>
              {schedules.length ? (
                <List dense className={classes.list}>
                  {schedules.map((schedule) => (
                    <ListItem
                      key={schedule.key}
                      button={Boolean(canControlDevice && schedule.channelId)}
                      selected={schedule.isActive}
                      onClick={() => handleSelectSchedule(schedule)}
                      disabled={
                        !canControlDevice || isChangingChannel || !schedule.channelId
                      }
                    >
                      <ListItemText
                        primary={schedule.label}
                        secondary={
                          schedule.isActive
                            ? 'Active channel'
                            : schedule.artist || undefined
                        }
                      />
                    </ListItem>
                  ))}
                </List>
              ) : (
                <Typography className={classes.helperText}>
                  No schedules available for this device.
                </Typography>
              )}
            </CardContent>
          </Card>

          {lastUpdatedLabel ? (
            <Typography className={classes.helperText}>
              Last updated at {lastUpdatedLabel}
            </Typography>
          ) : null}
        </>
      ) : null}
    </div>
  )
}

export default RetailPlayerDashboard
