import React, { useCallback, useEffect, useState } from 'react'
import {
  Badge,
  Card,
  CardContent,
  CircularProgress,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Popover,
  Tooltip,
  Typography,
  makeStyles,
} from '@material-ui/core'
import { MdOutlineNotifications } from 'react-icons/md'
import { useTranslate, useNotify } from 'react-admin'
import { useRefreshOnEvents } from '../common'
import { httpClient } from '../dataProvider'

const PAGE_SIZE = 100

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
    fontWeight: theme.typography.fontWeightBold,
    color: theme.palette.info.main,
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
  loadMoreItem: {
    display: 'flex',
    justifyContent: 'center',
  },
  notificationBadge: {
    '& .MuiBadge-badge': {
      minWidth: theme.spacing(2),
      height: theme.spacing(2),
      borderRadius: theme.spacing(1),
      fontSize: '0.65rem',
      padding: theme.spacing(0, 0.5),
      top: theme.spacing(0.5),
      right: theme.spacing(0.5),
    },
  },
}))

const MissingTracksPanel = () => {
  const translate = useTranslate()
  const notify = useNotify()
  const [anchorEl, setAnchorEl] = useState(null)
  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [totalCount, setTotalCount] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [nextOffset, setNextOffset] = useState(0)

  const open = Boolean(anchorEl)
  const classes = useStyles({ open })

  const fetchEntries = useCallback(
    (offset = 0, append = false) => {
      const setLoadingState = append ? setLoadingMore : setLoading
      setLoadingState(true)
      const params = new URLSearchParams({
        limit: PAGE_SIZE.toString(),
        offset: Math.max(offset, 0).toString(),
      })
      return httpClient(`/api/notifications/missing-tracks?${params.toString()}`)
        .then(({ json, headers }) => {
          const list = Array.isArray(json) ? json : []
          const totalHeader = headers && headers.get ? headers.get('X-Total-Count') : null
          const parsedTotal = totalHeader ? parseInt(totalHeader, 10) : NaN
          setEntries((prev) => {
            const nextEntries = append ? [...prev, ...list] : list
            const totalValue = Number.isNaN(parsedTotal) ? nextEntries.length : parsedTotal
            setTotalCount(totalValue)
            setNextOffset(nextEntries.length)
            setHasMore(nextEntries.length < totalValue && list.length > 0)
            return nextEntries
          })
        })
        .catch((error) => {
          notify('ra.notification.http_error', 'warning', {
            messageArgs: { error: error.message || 'Unknown error' },
          })
          if (!append) {
            setEntries([])
            setTotalCount(0)
            setNextOffset(0)
            setHasMore(false)
          }
        })
        .finally(() => setLoadingState(false))
    },
    [notify],
  )

  const refreshEntries = useCallback(() => fetchEntries(0, false), [fetchEntries])

  const handleOpen = useCallback(
    (event) => {
      setAnchorEl(event.currentTarget)
      fetchEntries(0, false)
    },
    [fetchEntries],
  )

  const handleClose = useCallback(() => {
    setAnchorEl(null)
  }, [])

  const handleLoadMore = useCallback(() => {
    fetchEntries(nextOffset, true)
  }, [fetchEntries, nextOffset])
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

  useEffect(() => {
    fetchEntries(0, false)
  }, [fetchEntries])

  useRefreshOnEvents({
    events: ['*'],
    onRefresh: refreshEntries,
  })

  return (
    <div>
      <Tooltip title={translate('notifications.missingTracks')}>
        <Badge
          badgeContent={totalCount}
          max={9999}
          color="secondary"
          invisible={totalCount === 0}
          className={classes.notificationBadge}
        >
          <IconButton
            className={classes.button}
            onClick={handleOpen}
            aria-label={translate('notifications.missingTracks')}
            aria-haspopup="true"
          >
            <MdOutlineNotifications size={20} />
          </IconButton>
        </Badge>
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
                {hasMore && (
                  <ListItem
                    button
                    onClick={handleLoadMore}
                    disabled={loadingMore}
                    className={classes.loadMoreItem}
                  >
                    {loadingMore ? (
                      <CircularProgress size={20} />
                    ) : (
                      <ListItemText
                        primary={translate('ra.action.load_more', { _: 'Load More…' })}
                        primaryTypographyProps={{ align: 'center' }}
                      />
                    )}
                  </ListItem>
                )}
              </List>
            )}
          </CardContent>
        </Card>
      </Popover>
    </div>
  )
}

export default MissingTracksPanel
