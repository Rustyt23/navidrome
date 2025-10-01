import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Badge,
  Box,
  Chip,
  CircularProgress,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Popover,
  Tooltip,
  Typography,
} from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { MdNotificationsActive } from 'react-icons/md'
import { useNotify, useTranslate } from 'react-admin'
import { useSelector } from 'react-redux'
import { httpClient } from '../dataProvider'
import { REST_URL } from '../consts'
import { useInterval } from '../common'

const useStyles = makeStyles(
  (theme) => ({
    button: {
      color: theme.palette.text.secondary,
    },
    buttonActive: {
      color: theme.palette.warning.main,
    },
    popoverContent: {
      width: 360,
      maxWidth: '90vw',
    },
    header: {
      padding: theme.spacing(2, 3, 1, 3),
    },
    listContainer: {
      maxHeight: 320,
      overflowY: 'auto',
    },
    listItem: {
      alignItems: 'flex-start',
    },
    secondaryLine: {
      display: 'flex',
      flexDirection: 'column',
      gap: theme.spacing(0.5),
      marginTop: theme.spacing(0.5),
    },
    playlists: {
      display: 'flex',
      flexWrap: 'wrap',
      gap: theme.spacing(1),
    },
    emptyState: {
      padding: theme.spacing(2, 3, 3, 3),
      color: theme.palette.text.secondary,
    },
    detectedAt: {
      color: theme.palette.text.secondary,
      fontSize: theme.typography.caption.fontSize,
    },
    loadingBox: {
      padding: theme.spacing(3),
      display: 'flex',
      justifyContent: 'center',
      alignItems: 'center',
    },
  }),
  { name: 'NDMissingNotificationsPanel' },
)

const formatDetectedAt = (value) => {
  if (!value) {
    return ''
  }
  try {
    const date = new Date(value)
    return date.toLocaleString()
  } catch (err) {
    return value
  }
}

const MissingNotificationsPanel = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const scanStatus = useSelector((state) => state.activity?.scanStatus)
  const streamReconnected = useSelector(
    (state) => state.activity?.streamReconnected,
  )
  const [anchorEl, setAnchorEl] = useState(null)
  const [notifications, setNotifications] = useState([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const open = Boolean(anchorEl)
  const count = notifications.length

  const fetchNotifications = useCallback(() => {
    setLoading(true)
    return httpClient(`${REST_URL}/missing/notifications`)
      .then(({ json }) => {
        setNotifications(json?.data || [])
        setError(null)
      })
      .catch((err) => {
        setError(err)
        notify('notifications.missing.load_error', {
          type: 'warning',
          messageArgs: { error: err?.message },
        })
      })
      .finally(() => setLoading(false))
  }, [notify])

  useEffect(() => {
    fetchNotifications()
  }, [fetchNotifications])

  useEffect(() => {
    if (scanStatus && !scanStatus.scanning) {
      fetchNotifications()
    }
  }, [scanStatus?.scanning, fetchNotifications])

  useEffect(() => {
    if (streamReconnected) {
      fetchNotifications()
    }
  }, [streamReconnected, fetchNotifications])

  useEffect(() => {
    if (open) {
      fetchNotifications()
    }
  }, [open, fetchNotifications])

  useInterval(() => {
    if (!loading) {
      fetchNotifications()
    }
  }, open ? 10000 : 60000)

  const handleOpen = useCallback(
    (event) => {
      setAnchorEl(event.currentTarget)
    },
    [],
  )

  const handleClose = useCallback(() => {
    setAnchorEl(null)
  }, [])

  const buttonClass = useMemo(
    () => (count > 0 ? classes.buttonActive : classes.button),
    [classes.button, classes.buttonActive, count],
  )

  const countLabel = useMemo(() => {
    if (count === 0) {
      return translate('notifications.missing.none')
    }
    return translate('notifications.missing.count', { smart_count: count })
  }, [count, translate])

  const listContent = useMemo(() => {
    if (loading) {
      return (
        <Box className={classes.loadingBox}>
          <CircularProgress size={24} />
        </Box>
      )
    }
    if (error) {
      return (
        <Typography className={classes.emptyState} variant="body2">
          {translate('notifications.missing.load_error', {
            error: error?.message || 'unknown',
          })}
        </Typography>
      )
    }
    if (notifications.length === 0) {
      return null
    }
    return (
      <List className={classes.listContainer} dense>
        {notifications.map((item) => {
          const { mediaFile = {}, playlistNames = [], detectedAt, songTitle } = item
          const title =
            mediaFile.title || songTitle || translate('notifications.missing.untitled')
          const artist = mediaFile.artist || translate('resources.song.fields.artist')
          const album = mediaFile.album
          return (
            <ListItem key={item.mediaFileId} className={classes.listItem} divider>
              <ListItemText
                primary={title}
                secondary={
                  <div className={classes.secondaryLine}>
                    <Typography variant="body2" color="textSecondary">
                      {artist}
                    </Typography>
                    {album && (
                      <Typography variant="body2" color="textSecondary">
                        {album}
                      </Typography>
                    )}
                    <Typography className={classes.detectedAt} variant="caption">
                      {translate('notifications.missing.detected', {
                        value: formatDetectedAt(detectedAt),
                      })}
                    </Typography>
                    <Box className={classes.playlists}>
                      {playlistNames.length > 0 ? (
                        playlistNames.map((name) => (
                          <Chip key={name} label={name} size="small" />
                        ))
                      ) : (
                        <Typography variant="caption" color="textSecondary">
                          {translate('notifications.missing.no_playlists')}
                        </Typography>
                      )}
                    </Box>
                  </div>
                }
              />
            </ListItem>
          )
        })}
      </List>
    )
  }, [
    notifications,
    loading,
    error,
    classes.listContainer,
    classes.listItem,
    classes.secondaryLine,
    classes.playlists,
    classes.detectedAt,
    classes.emptyState,
    classes.loadingBox,
    translate,
  ])

  return (
    <div>
      <Tooltip title={translate('notifications.missing.title')}>
        <IconButton className={buttonClass} onClick={handleOpen}>
          <Badge
            color="secondary"
            badgeContent={count}
            invisible={count === 0}
          >
            <MdNotificationsActive size={20} />
          </Badge>
        </IconButton>
      </Tooltip>
      <Popover
        id="panel-missing-notifications"
        anchorEl={anchorEl}
        open={open}
        onClose={handleClose}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
      >
        <div className={classes.popoverContent}>
          <Box className={classes.header}>
            <Typography variant="subtitle1">
              {translate('notifications.missing.title')}
            </Typography>
            <Typography variant="caption" color="textSecondary">
              {countLabel}
            </Typography>
          </Box>
          {listContent}
        </div>
      </Popover>
    </div>
  )
}

export default MissingNotificationsPanel
