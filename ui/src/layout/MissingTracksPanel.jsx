import React, { useCallback, useState } from 'react'
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
  makeStyles,
} from '@material-ui/core'
import { MdOutlineNotifications } from 'react-icons/md'
import { useTranslate, useNotify } from 'react-admin'
import { httpClient } from '../dataProvider'

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

  const open = Boolean(anchorEl)

  const fetchEntries = useCallback(() => {
    setLoading(true)
    httpClient('/api/notifications/missing-tracks')
      .then(({ json }) => {
        const list = Array.isArray(json) ? json : []
        setEntries(list)
      })
      .catch((error) => {
        notify('ra.notification.http_error', 'warning', {
          messageArgs: { error: error.message || 'Unknown error' },
        })
        setEntries([])
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

  const formatLine = useCallback(
    (track) => {
      const title = track?.title || translate('notifications.missingTracksUnknownTitle')
      const artist = track?.artist || translate('notifications.missingTracksUnknownArtist')
      return `${title} — ${artist}`
    },
    [translate],
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
                {entries.map((track, index) => (
                  <ListItem key={`${track.title || 'track'}-${track.artist || index}-${index}`}>
                    <ListItemText primary={formatLine(track)} />
                  </ListItem>
                ))}
              </List>
            )}
          </CardContent>
        </Card>
      </Popover>
    </div>
  )
}

export default MissingTracksPanel
