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

  const mergeEntries = useCallback((base, incoming) => {
    if (!incoming) {
      return base
    }
    const merged = { ...base }
    Object.keys(incoming).forEach((key) => {
      const value = incoming[key]
      if (value === undefined || value === null) {
        return
      }
      if (typeof value === 'string' && value.trim() === '') {
        return
      }
      const existing = merged[key]
      if (existing === undefined || existing === null || (typeof existing === 'string' && existing.trim() === '')) {
        merged[key] = value
      }
    })
    return merged
  }, [])

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

      const missingParams = new URLSearchParams({
        _sort: 'updated_at',
        _order: 'DESC',
        _start: start.toString(),
        _end: end.toString(),
      })
      missingParams.set('_', Date.now().toString())

      const notificationsParams = new URLSearchParams({
        limit: PAGE_SIZE.toString(),
        offset: start.toString(),
      })
      notificationsParams.set('_', Date.now().toString())

      const requestOptions = {
        cache: 'no-store',
        signal: controller.signal,
        headers: new Headers({
          Accept: 'application/json',
          'Cache-Control': 'no-cache',
          Pragma: 'no-cache',
        }),
      }

      const requests = [
        httpClient(`/api/notifications/missing-tracks?${notificationsParams.toString()}`, requestOptions),
        httpClient(`${REST_URL}/missing?${missingParams.toString()}`, requestOptions),
      ]

      return Promise.allSettled(requests)
        .then((results) => {
          if (activeRequestRef.current.id !== nextRequestId) {
            return
          }

          const [notificationsResult, missingResult] = results
          const notificationsSuccess =
            notificationsResult.status === 'fulfilled' ? notificationsResult.value : null
          const missingSuccess = missingResult.status === 'fulfilled' ? missingResult.value : null

          const isNotificationsError =
            notificationsResult.status === 'rejected' && notificationsResult.reason?.name !== 'AbortError'
          const isMissingError =
            missingResult.status === 'rejected' && missingResult.reason?.name !== 'AbortError'

          if (!notificationsSuccess && !missingSuccess) {
            if (isNotificationsError || isMissingError) {
              const error = isMissingError ? missingResult.reason : notificationsResult.reason
              notify('ra.notification.http_error', 'warning', {
                messageArgs: { error: (error && error.message) || 'Unknown error' },
              })
            }
            if (!append) {
              setEntries([])
              setTotalCount(0)
              setNextOffset(0)
              setHasMore(false)
            }
            return
          }

          if (isNotificationsError || isMissingError) {
            const error = isMissingError ? missingResult.reason : notificationsResult.reason
            if (error?.name !== 'AbortError') {
              notify('ra.notification.http_error', 'warning', {
                messageArgs: { error: (error && error.message) || 'Unknown error' },
              })
            }
          }

          const notificationList = Array.isArray(notificationsSuccess?.json)
            ? notificationsSuccess.json
            : []
          const missingList = Array.isArray(missingSuccess?.json) ? missingSuccess.json : []

          const notificationsHeader = notificationsSuccess?.headers?.get
            ? notificationsSuccess.headers.get('X-Total-Count')
            : null
          const parsedNotificationsTotal = notificationsHeader
            ? parseInt(notificationsHeader, 10)
            : NaN

          const missingHeader = missingSuccess?.headers?.get
            ? missingSuccess.headers.get('X-Total-Count')
            : null
          const parsedMissingTotal = (() => {
            if (!missingHeader) {
              return missingList.length + (append ? start : 0)
            }
            const segments = missingHeader.split('/')
            const parsed = parseInt(segments[segments.length - 1], 10)
            if (Number.isNaN(parsed)) {
              return missingList.length + (append ? start : 0)
            }
            return parsed
          })()

          setEntries((prev) => {
            const previous = append ? prev : []
            const nextEntriesMap = new Map()
            const getKey = (entry, fallbackIndex) => {
              if (!entry) {
                return `unknown-${fallbackIndex}`
              }
              return (
                entry.id ||
                entry.path ||
                entry.trackPath ||
                `${entry.title || ''}-${entry.artist || ''}` ||
                `unknown-${fallbackIndex}`
              )
            }

            const addEntry = (entry, indexOffset = 0) => {
              if (!entry) {
                return
              }
              const key = getKey(entry, indexOffset)
              const existing = nextEntriesMap.get(key)
              if (existing) {
                nextEntriesMap.set(key, mergeEntries(existing, entry))
              } else {
                nextEntriesMap.set(key, { ...entry })
              }
            }

            previous.forEach((entry, index) => addEntry(entry, index))
            missingList.forEach((entry, index) => addEntry(entry, start + index))
            notificationList.forEach((entry, index) => addEntry(entry, start + index + missingList.length))

            const nextEntries = Array.from(nextEntriesMap.values())

            const hasMoreMissing =
              Number.isFinite(parsedMissingTotal) && start + missingList.length < parsedMissingTotal
            const hasMoreNotifications =
              Number.isFinite(parsedNotificationsTotal) &&
              start + notificationList.length < parsedNotificationsTotal

            const computedTotal = Math.max(
              nextEntries.length,
              Number.isFinite(parsedMissingTotal) ? parsedMissingTotal : 0,
              Number.isFinite(parsedNotificationsTotal) ? parsedNotificationsTotal : 0,
            )

            setTotalCount(computedTotal)
            setNextOffset(start + Math.max(missingList.length, notificationList.length))
            setHasMore(
              (hasMoreMissing || hasMoreNotifications) &&
              (missingList.length > 0 || notificationList.length > 0),
            )

            return nextEntries
          })
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
    [mergeEntries, notify],
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
