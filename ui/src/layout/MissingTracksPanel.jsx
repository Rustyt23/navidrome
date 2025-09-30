import React, { useCallback, useMemo, useState } from 'react'
import {
  Card,
  CardContent,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Popover,
  Tooltip,
  Typography,
  CircularProgress,
  Collapse,
  makeStyles,
} from '@material-ui/core'
import { MdOutlineNotifications } from 'react-icons/md'
import { useTranslate, useNotify } from 'react-admin'
import { httpClient } from '../dataProvider'
import ExpandLessIcon from '@material-ui/icons/ExpandLess'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import { formatDuration } from '../utils'

const useStyles = makeStyles((theme) => ({
  button: { color: 'inherit' },
  card: { padding: 0 },
  cardContent: {
    padding: `${theme.spacing(1)}px !important`,
    '&:last-child': {
      paddingBottom: `${theme.spacing(1)}px !important`,
    },
  },
  list: {
    width: '30em',
    maxHeight: '20em',
    overflowY: 'auto',
    padding: 0,
  },
  nestedList: {
    paddingLeft: theme.spacing(2),
  },
  summaryItem: {
    paddingLeft: theme.spacing(1),
    paddingRight: theme.spacing(1),
  },
  nestedItem: {
    paddingTop: theme.spacing(0.5),
    paddingBottom: theme.spacing(0.5),
    paddingLeft: theme.spacing(2),
    paddingRight: theme.spacing(1),
  },
  empty: {
    padding: theme.spacing(1, 2),
  },
  progressWrapper: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing(2),
  },
}))

const MissingTracksPanel = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const [anchorEl, setAnchorEl] = useState(null)
  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(false)
  const [expanded, setExpanded] = useState({})

  const open = Boolean(anchorEl)

  const fetchEntries = useCallback(() => {
    setLoading(true)
    httpClient('/api/notifications/missing-tracks')
      .then(({ json }) => {
        const list = Array.isArray(json) ? json : []
        setEntries(list)
        setExpanded({})
      })
      .catch((error) => {
        notify('ra.notification.http_error', 'warning', {
          messageArgs: { error: error.message || 'Unknown error' },
        })
        setEntries([])
        setExpanded({})
      })
      .finally(() => setLoading(false))
  }, [notify])

  const handleOpen = useCallback(
    (event) => {
      setAnchorEl(event.currentTarget)
      fetchEntries()
    },
    [fetchEntries],
  )

  const handleClose = useCallback(() => {
    setAnchorEl(null)
  }, [])

  const togglePlaylist = useCallback((playlistId) => {
    setExpanded((prev) => ({
      ...prev,
      [playlistId]: !prev[playlistId],
    }))
  }, [])

  const getPlaylistName = useCallback(
    (entry) => {
      if (!entry) {
        return ''
      }
      if (entry.playlist_name) {
        return entry.playlist_name
      }
      if (!entry.playlist_id) {
        return translate('notifications.missingTracksUnknownPlaylist')
      }
      const parts = entry.playlist_id.split(/[\\/]/)
      const raw = parts[parts.length - 1] || entry.playlist_id
      return raw.replace(/\.m3u8?$/i, '')
    },
    [translate],
  )

  const getTrackPrimary = useCallback(
    (track) => {
      if (!track) {
        return ''
      }
      if (track.title) {
        return track.title
      }
      const parts = (track.track_path || '').split(/[\\/]/)
      const fallback = parts[parts.length - 1]
      return fallback || translate('notifications.missingTracksUnknownTitle')
    },
    [translate],
  )

  const getTrackSecondary = useCallback((track) => {
    if (!track) {
      return ''
    }
    const details = []
    if (track.artist) {
      details.push(track.artist)
    }
    if (track.duration_seconds && track.duration_seconds > 0) {
      details.push(formatDuration(track.duration_seconds))
    }
    if (details.length === 0 && track.title) {
      return ''
    }
    if (details.length === 0) {
      const parts = (track.track_path || '').split(/[\\/]/)
      return parts[parts.length - 1] || ''
    }
    return details.join(' • ')
  }, [])

  const renderedEntries = useMemo(
    () =>
      entries.map((entry, index) => {
        const playlistId = entry.playlist_id || `playlist-${index}`
        const summaryCount = entry.missing_count || (entry.tracks ? entry.tracks.length : 0)
        const summaryText = translate('notifications.missingTracksSummary', {
          smart_count: summaryCount,
          playlist: getPlaylistName(entry),
        })
        const isExpanded = !!expanded[playlistId]
        const tracks = Array.isArray(entry.tracks) ? entry.tracks : []

        return (
          <div key={playlistId}>
            <ListItem
              button
              onClick={() => togglePlaylist(playlistId)}
              className={classes.summaryItem}
            >
              <ListItemText primary={summaryText} />
              {isExpanded ? <ExpandLessIcon fontSize="small" /> : <ExpandMoreIcon fontSize="small" />}
            </ListItem>
            <Collapse in={isExpanded} timeout="auto" unmountOnExit>
              <List disablePadding className={classes.nestedList}>
                {tracks.map((track, index) => (
                  <ListItem
                    key={`${playlistId}-${track.track_path || index}-${track.created_at || index}-${index}`}
                    className={classes.nestedItem}
                  >
                    <ListItemText
                      primary={getTrackPrimary(track)}
                      secondary={getTrackSecondary(track)}
                    />
                  </ListItem>
                ))}
              </List>
            </Collapse>
          </div>
        )
      }),
    [classes.nestedItem, classes.nestedList, classes.summaryItem, entries, expanded, getPlaylistName, getTrackPrimary, getTrackSecondary, togglePlaylist, translate],
  )

  return (
    <div>
      <Tooltip title={translate('notifications.missingTracks')}>
        <IconButton
          className={classes.button}
          onClick={handleOpen}
          aria-label={translate('notifications.missingTracks')}
          aria-haspopup="true"
        >
          <MdOutlineNotifications size={20} />
        </IconButton>
      </Tooltip>
      <Popover
        id="panel-missing-tracks"
        anchorEl={anchorEl}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        open={open}
        onClose={handleClose}
      >
        <Card className={classes.card}>
          <CardContent className={classes.cardContent}>
            {loading ? (
              <div className={classes.progressWrapper}>
                <CircularProgress size={24} />
              </div>
            ) : entries.length === 0 ? (
              <Typography className={classes.empty}>
                {translate('notifications.missingTracksEmpty')}
              </Typography>
            ) : (
              <List className={classes.list} dense>
                {renderedEntries}
              </List>
            )}
          </CardContent>
        </Card>
      </Popover>
    </div>
  )
}

export default MissingTracksPanel
