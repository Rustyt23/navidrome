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
import { useSelector } from 'react-redux'
import { httpClient } from '../dataProvider'
import { useInterval } from '../common'
import { subscribeLibraryMutated } from '../utils/libraryMutationEvents'

const PAGE_SIZE = 100
const OPEN_POLL_INTERVAL = 20000
const CLOSED_POLL_INTERVAL = 60000
const REFRESH_RELEVANT_RESOURCES = [
  '*',
  'song',
  'playlist',
  'playlistTrack',
  'missing',
  'folder',
]

const buildEntryKey = (entry) => {
  if (!entry) {
    return 'missing-entry'
  }
  const title = (entry.title || '').toString().trim().toLowerCase()
  const artist = (entry.artist || '').toString().trim().toLowerCase()
  const source = `${title}:::${artist}`
  let hash = 0
  for (let i = 0; i < source.length; i += 1) {
    hash = (hash * 31 + source.charCodeAt(i)) | 0
  }
  return `missing-${hash.toString(16)}`
}

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
  const scanStatus = useSelector((state) => state.activity?.scanStatus || {})
  const refreshEvent = useSelector((state) => state.activity?.refresh)
  const streamReconnected = useSelector((state) => state.activity?.streamReconnected)

  const [anchorEl, setAnchorEl] = useState(null)
  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [totalCount, setTotalCount] = useState(0)
  const [hasMore, setHasMore] = useState(false)
  const [nextOffset, setNextOffset] = useState(0)

  const open = Boolean(anchorEl)
  const classes = useStyles({ open })
  const isMountedRef = useRef(false)
  const fetchIdRef = useRef(0)
  const lastRefreshHandledRef = useRef(0)
  const prevScanningRef = useRef(Boolean(scanStatus?.scanning))

  const fetchEntries = useCallback(
    (offset = 0, append = false) => {
      const setLoadingState = append ? setLoadingMore : setLoading
      setLoadingState(true)
      const requestId = ++fetchIdRef.current
      const params = new URLSearchParams({
        limit: PAGE_SIZE.toString(),
        offset: Math.max(offset, 0).toString(),
      })
      httpClient(`/api/notifications/missing-tracks?${params.toString()}`)
        .then(({ json, headers }) => {
          if (!isMountedRef.current || requestId !== fetchIdRef.current) {
            return
          }
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
          if (!isMountedRef.current || requestId !== fetchIdRef.current) {
            return
          }
          if (!append) {
            setEntries([])
            setTotalCount(0)
            setNextOffset(0)
            setHasMore(false)
          }
        })
        .finally(() => {
          if (!isMountedRef.current || requestId !== fetchIdRef.current) {
            return
          }
          setLoadingState(false)
        })
    },
    [notify],
  )

  const refetchEntries = useCallback(() => {
    fetchEntries(0, false)
  }, [fetchEntries])

  const handleOpen = useCallback((event) => {
    setAnchorEl(event.currentTarget)
  }, [])

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
    isMountedRef.current = true
    fetchEntries(0, false)
    return () => {
      isMountedRef.current = false
    }
  }, [fetchEntries])

  useEffect(() => {
    if (open) {
      refetchEntries()
    }
  }, [open, refetchEntries])

  useEffect(() => {
    if (streamReconnected) {
      refetchEntries()
    }
  }, [streamReconnected, refetchEntries])

  useEffect(() => {
    const unsubscribe = subscribeLibraryMutated(refetchEntries)
    return () => unsubscribe()
  }, [refetchEntries])

  useEffect(() => {
    const scanning = Boolean(scanStatus?.scanning)
    const wasScanning = Boolean(prevScanningRef.current)
    if (wasScanning && !scanning) {
      refetchEntries()
    }
    prevScanningRef.current = scanning
  }, [scanStatus, refetchEntries])

  useEffect(() => {
    const lastReceived = refreshEvent?.lastReceived
    const resources = refreshEvent?.resources
    if (!lastReceived || lastReceived <= lastRefreshHandledRef.current) {
      return
    }

    const shouldHandle =
      !resources ||
      resources['*'] === '*' ||
      REFRESH_RELEVANT_RESOURCES.some((resourceName) => resources?.[resourceName])

    if (shouldHandle) {
      lastRefreshHandledRef.current = lastReceived
      refetchEntries()
    }
  }, [refreshEvent, refetchEntries])

  useInterval(() => {
    if (open) {
      refetchEntries()
    }
  }, open ? OPEN_POLL_INTERVAL : null)

  useInterval(() => {
    if (!open) {
      refetchEntries()
    }
  }, open ? null : CLOSED_POLL_INTERVAL)

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
                {entries.map((entry) => (
                  <ListItem key={buildEntryKey(entry)} className={classes.listItem}>
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
