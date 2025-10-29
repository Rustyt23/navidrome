import React, { useCallback, useEffect, useRef, useState } from 'react'
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
import { REST_URL } from '../consts'

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
  const activeRequestRef = useRef({ id: 0, controller: null })

  const open = Boolean(anchorEl)
  const classes = useStyles({ open })

  const fetchEntries = useCallback(
    (offset = 0, append = false) => {
      const controller = new AbortController()
      const setLoadingState = append ? setLoadingMore : setLoading
      const nextRequestId = activeRequestRef.current.id + 1
      const start = Math.max(offset, 0)
      const end = start + PAGE_SIZE

      if (activeRequestRef.current.controller) {
        activeRequestRef.current.controller.abort()
      }

      activeRequestRef.current = {
        id: nextRequestId,
        controller,
      }

      setLoadingState(true)

      const params = new URLSearchParams({
        _sort: 'updated_at',
        _order: 'DESC',
        _start: start.toString(),
        _end: end.toString(),
      })
      params.set('_', Date.now().toString())

      return httpClient(`${REST_URL}/missing?${params.toString()}`, {
        cache: 'no-store',
        signal: controller.signal,
        headers: new Headers({
          Accept: 'application/json',
          'Cache-Control': 'no-cache',
          Pragma: 'no-cache',
        }),
      })
        .then(({ json, headers }) => {
          if (activeRequestRef.current.id !== nextRequestId) {
            return
          }

          const list = Array.isArray(json) ? json : []
          const rawTotal = headers && headers.get ? headers.get('X-Total-Count') : null
          const totalValue = (() => {
            if (!rawTotal) {
              return list.length + (append ? start : 0)
            }
            const segments = rawTotal.split('/')
            const parsed = parseInt(segments[segments.length - 1], 10)
            if (Number.isNaN(parsed)) {
              return list.length + (append ? start : 0)
            }
            return parsed
          })()

          setEntries((prev) => {
            const previous = append ? prev : []
            const nextEntriesMap = new Map()
            previous.forEach((entry) => {
              const key = entry?.id || entry?.path || `${entry?.title || ''}-${entry?.artist || ''}`
              nextEntriesMap.set(key, entry)
            })
            list.forEach((entry) => {
              const key = entry?.id || entry?.path || `${entry?.title || ''}-${entry?.artist || ''}`
              nextEntriesMap.set(key, entry)
            })
            const nextEntries = Array.from(nextEntriesMap.values())
            setTotalCount(totalValue)
            setNextOffset(start + list.length)
            setHasMore(start + list.length < totalValue && list.length > 0)
            return nextEntries
          })
        })
        .catch((error) => {
          if (error?.name === 'AbortError') {
            return
          }
          if (activeRequestRef.current.id !== nextRequestId) {
            return
          }
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
        .finally(() => {
          if (activeRequestRef.current.id !== nextRequestId) {
            return
          }
          setLoadingState(false)
          if (activeRequestRef.current.controller === controller) {
            activeRequestRef.current.controller = null
          }
        })
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
      const rawTitle = (entry.title || entry.name || '').trim()
      const rawArtist = (entry.artist || entry.artistName || '').trim()
      const fallbackTitle = (entry.path || '').split(/[/\\]/).pop() || ''
      const title =
        rawTitle ||
        fallbackTitle ||
        translate('notifications.missingTracksUnknownTitle')
      const unknownArtist = translate('notifications.missingTracksUnknownArtist').trim()
      const hasArtist =
        rawArtist && rawArtist.toLocaleLowerCase() !== unknownArtist.toLocaleLowerCase()
      return hasArtist ? `${title} — ${rawArtist}` : title
    },
    [translate],
  )

  const getEntrySecondaryLabel = useCallback((entry) => {
    if (!entry) {
      return ''
    }
    const details = [entry.libraryName, entry.album || entry.albumName]
      .map((value) => (typeof value === 'string' ? value.trim() : ''))
      .filter(Boolean)
    return details.join(' • ')
  }, [])

  useEffect(() => {
    fetchEntries(0, false)
  }, [fetchEntries])

  useEffect(
    () => () => {
      if (activeRequestRef.current.controller) {
        activeRequestRef.current.controller.abort()
        activeRequestRef.current.controller = null
      }
    },
    [],
  )

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
                {entries.map((entry, index) => {
                  const key = entry?.id || entry?.path || `${index}-${entry?.title || 'missing'}`
                  return (
                    <ListItem key={key} className={classes.listItem}>
                      <ListItemText
                        primary={getEntryLabel(entry)}
                        secondary={getEntrySecondaryLabel(entry)}
                        secondaryTypographyProps={{ variant: 'body2', color: 'textSecondary' }}
                      />
                    </ListItem>
                  )
                })}
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
