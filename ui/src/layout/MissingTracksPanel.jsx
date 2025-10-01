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
  button: (props) => ({
    color: props.open ? theme.palette.secondary.main : 'inherit',
  }),
  card: { padding: 0 },
  cardContent: {
    padding: `${theme.spacing(1)}px !important`,
    '&:last-child': {
      paddingBottom: `${theme.spacing(1)}px !important`,
    },
  },
  list: {
    width: '36em',
    maxHeight: '28em',
    overflowY: 'auto',
    padding: 0,
  },
  listItem: {
    paddingLeft: theme.spacing(1.5),
    paddingRight: theme.spacing(1.5),
  },
  header: {
    padding: theme.spacing(0, 1.5, 1),
    fontWeight: theme.typography.fontWeightMedium,
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
  const translate = useTranslate()
  const notify = useNotify()
  const [anchorEl, setAnchorEl] = useState(null)
  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(false)

  const open = Boolean(anchorEl)
  const classes = useStyles({ open })

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
  const getEntryLabel = useCallback(
    (entry) => {
      if (!entry) {
        return ''
      }
      const rawTitle = (entry.title || '').trim()
      const rawArtist = (entry.artist || '').trim()
      const title = rawTitle || translate('notifications.missingTracksUnknownTitle')
      const unknownArtist = translate('notifications.missingTracksUnknownArtist').trim()
      const hasArtist =
        rawArtist && rawArtist.toLocaleLowerCase() !== unknownArtist.toLocaleLowerCase()
      return hasArtist ? `${title} — ${rawArtist}` : title
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
            <Typography className={classes.header} variant="subtitle2">
              {translate('notifications.missingTracksListTitle')}
            </Typography>
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
                  <ListItem key={`${entry.title || 'missing'}-${entry.artist || index}-${index}`} className={classes.listItem}>
                    <ListItemText primary={getEntryLabel(entry)} />
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
