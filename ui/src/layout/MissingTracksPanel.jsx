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
  listItem: {
    paddingTop: theme.spacing(0.5),
    paddingBottom: theme.spacing(0.5),
    paddingLeft: theme.spacing(1),
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

  const open = Boolean(anchorEl)

  const fetchEntries = useCallback(() => {
    setLoading(true)
    httpClient('/api/notifications/missing-tracks')
      .then(({ json }) => {
        if (Array.isArray(json)) {
          setEntries(json)
        } else {
          setEntries([])
        }
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
                {entries.map((entry, index) => (
                  <ListItem
                    key={`${entry.playlist_id || 'playlist'}-${entry.track_path || 'path'}-${entry.created_at || index}-${index}`}
                    className={classes.listItem}
                  >
                    <ListItemText
                      primary={`${entry.playlist_id || '-'} - ${
                        entry.track_path || ''
                      } (${entry.created_at || ''})`}
                    />
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
