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
import { MdDriveFileRenameOutline, MdInfoOutline } from 'react-icons/md'
import { PlayArrow } from '@material-ui/icons'
import { useTranslate, useNotify } from 'react-admin'
import { useDispatch } from 'react-redux'
import { httpClient } from '../dataProvider'
import { setNamingEnforcement } from '../actions'

const PAGE_SIZE = 100
const LIST_URL = '/api/notifications/naming-issues'
const RENAME_URL = '/api/song/naming/rename'
const SETTINGS_URL = '/api/song/naming/settings'

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
    width: '38em',
    maxHeight: '28em',
    overflowY: 'auto',
    padding: 0,
  },
  listItem: {
    paddingLeft: theme.spacing(0.5),
    paddingRight: theme.spacing(0.5),
  },
  header: {
    padding: theme.spacing(0, 1.5, 1),
    fontWeight: theme.typography.fontWeightBold,
    color: theme.palette.info.main,
  },
  subHeader: {
    padding: theme.spacing(0, 1.5, 1),
    color: theme.palette.text.secondary,
  },
  empty: { padding: theme.spacing(1, 2) },
  progressWrapper: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    padding: theme.spacing(2),
  },
  loadMoreItem: { display: 'flex', justifyContent: 'center' },
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
  // The rename control sits on the left of each row, the way a play button
  // does: one click, one track, the obvious action for that line.
  renameButton: {
    marginRight: theme.spacing(0.5),
    color: theme.palette.primary.main,
  },
  infoButton: { color: theme.palette.text.secondary },
  flow: {
    padding: theme.spacing(1.5),
    maxWidth: '32em',
  },
  flowName: {
    fontFamily: 'monospace',
    wordBreak: 'break-all',
  },
  flowArrow: {
    padding: theme.spacing(0.5, 0),
    color: theme.palette.text.secondary,
  },
  flowTo: {
    fontFamily: 'monospace',
    wordBreak: 'break-all',
    color: theme.palette.success.main,
  },
}))

// NamingIssuesPanel lists tracks whose filename does not match
// "Artist - Title.mp3".
//
// Those tracks are held out of the storage bucket by the sync service: the
// bucket is flat and playlists reference tracks by bare filename, so an
// oddly-named object is one no playlist entry can resolve. This panel is where
// they surface and where one click fixes them.
const NamingIssuesPanel = () => {
  const translate = useTranslate()
  const notify = useNotify()
  const dispatch = useDispatch()
  const [anchorEl, setAnchorEl] = useState(null)
  const [infoAnchorEl, setInfoAnchorEl] = useState(null)
  const [infoEntry, setInfoEntry] = useState(null)
  const [entries, setEntries] = useState([])
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)
  const [renamingId, setRenamingId] = useState(null)
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
      const requestId = activeRequestRef.current.id + 1
      const start = Math.max(offset, 0)

      if (activeRequestRef.current.controller) {
        activeRequestRef.current.controller.abort()
      }
      activeRequestRef.current = { id: requestId, controller }
      setLoadingState(true)

      const params = new URLSearchParams({
        limit: PAGE_SIZE.toString(),
        offset: start.toString(),
        _: Date.now().toString(),
      })

      return httpClient(`${LIST_URL}?${params.toString()}`, {
        cache: 'no-store',
        signal: controller.signal,
        headers: new Headers({
          Accept: 'application/json',
          'Cache-Control': 'no-cache',
        }),
      })
        .then(({ json, headers }) => {
          if (activeRequestRef.current.id !== requestId) {
            return
          }
          const list = Array.isArray(json) ? json : []
          const header = headers?.get ? headers.get('X-Total-Count') : null
          const total = header ? parseInt(header, 10) : NaN
          const resolvedTotal = Number.isFinite(total) ? total : list.length

          setEntries((prev) => (append ? [...prev, ...list] : list))
          setTotalCount(resolvedTotal)
          setNextOffset(start + list.length)
          setHasMore(start + list.length < resolvedTotal && list.length > 0)
        })
        .catch((error) => {
          if (error?.name === 'AbortError') {
            return
          }
          notify('ra.notification.http_error', 'warning', {
            messageArgs: { error: error?.message || 'Unknown error' },
          })
          if (!append) {
            setEntries([])
            setTotalCount(0)
            setHasMore(false)
          }
        })
        .finally(() => {
          if (activeRequestRef.current.id !== requestId) {
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

  const handleOpen = useCallback(
    (event) => {
      setAnchorEl(event.currentTarget)
      fetchEntries(0, false)
    },
    [fetchEntries],
  )

  const handleClose = useCallback(() => setAnchorEl(null), [])

  const handleLoadMore = useCallback(
    () => fetchEntries(nextOffset, true),
    [fetchEntries, nextOffset],
  )

  const handleShowInfo = useCallback((event, entry) => {
    event.stopPropagation()
    setInfoEntry(entry)
    setInfoAnchorEl(event.currentTarget)
  }, [])

  const handleCloseInfo = useCallback(() => {
    setInfoAnchorEl(null)
    setInfoEntry(null)
  }, [])

  const handleRename = useCallback(
    (entry) => {
      if (!entry?.id) {
        return
      }
      setRenamingId(entry.id)
      httpClient(RENAME_URL, {
        method: 'POST',
        body: JSON.stringify({ ids: [entry.id] }),
      })
        .then(({ json }) => {
          const result = json?.results?.[0]
          if (result?.error) {
            notify(result.error, 'warning')
            return
          }
          notify('notifications.namingIssuesRenamed', 'info', {
            from: result?.from || entry.currentName,
            to: result?.to || entry.expectedName,
          })
          // Drop the row rather than refetching: the server caches the scan
          // briefly, so an immediate refetch would show the track again.
          setEntries((prev) => prev.filter((e) => e.id !== entry.id))
          setTotalCount((prev) => Math.max(prev - 1, 0))
        })
        .catch((error) => {
          notify(error?.message || 'ra.notification.http_error', 'warning')
        })
        .finally(() => setRenamingId(null))
    },
    [notify],
  )

  useEffect(() => {
    fetchEntries(0, false)
  }, [fetchEntries])

  // Keep the store in step so the Personal settings toggle and this icon agree.
  useEffect(() => {
    httpClient(SETTINGS_URL)
      .then(({ json }) => {
        if (typeof json?.enforced === 'boolean') {
          dispatch(setNamingEnforcement(json.enforced))
        }
      })
      .catch(() => {})
  }, [dispatch])

  useEffect(
    () => () => {
      if (activeRequestRef.current.controller) {
        activeRequestRef.current.controller.abort()
        activeRequestRef.current.controller = null
      }
    },
    [],
  )

  return (
    <div>
      <Tooltip title={translate('notifications.namingIssues')}>
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
            aria-label={translate('notifications.namingIssues')}
            aria-haspopup="true"
          >
            <MdDriveFileRenameOutline size={20} />
          </IconButton>
        </Badge>
      </Tooltip>

      <Popover
        id="panel-naming-issues"
        anchorEl={anchorEl}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        open={open}
        onClose={handleClose}
      >
        <Card className={classes.card}>
          <CardContent className={classes.cardContent}>
            <Typography className={classes.header} variant="subtitle2">
              {translate('notifications.namingIssuesListTitle')}
            </Typography>
            <Typography className={classes.subHeader} variant="caption">
              {translate('notifications.namingIssuesHint')}
            </Typography>
            {loading ? (
              <div className={classes.progressWrapper}>
                <CircularProgress size={24} />
              </div>
            ) : entries.length === 0 ? (
              <Typography className={classes.empty}>
                {translate('notifications.namingIssuesEmpty')}
              </Typography>
            ) : (
              <List className={classes.list} dense>
                {entries.map((entry, index) => (
                  <ListItem
                    key={entry.id || `${index}-${entry.currentName}`}
                    className={classes.listItem}
                  >
                    <Tooltip
                      title={translate(
                        'notifications.namingIssuesRenameAction',
                      )}
                    >
                      <span>
                        <IconButton
                          size="small"
                          className={classes.renameButton}
                          disabled={renamingId === entry.id}
                          onClick={() => handleRename(entry)}
                          aria-label={translate(
                            'notifications.namingIssuesRenameAction',
                          )}
                        >
                          {renamingId === entry.id ? (
                            <CircularProgress size={16} />
                          ) : (
                            <PlayArrow fontSize="small" />
                          )}
                        </IconButton>
                      </span>
                    </Tooltip>
                    <ListItemText
                      primary={entry.currentName}
                      secondary={
                        entry.artist
                          ? `${entry.artist} — ${entry.title}`
                          : entry.title
                      }
                      primaryTypographyProps={{ noWrap: true }}
                      secondaryTypographyProps={{ noWrap: true }}
                    />
                    <Tooltip
                      title={translate('notifications.namingIssuesInfoAction')}
                    >
                      <IconButton
                        size="small"
                        className={classes.infoButton}
                        onClick={(event) => handleShowInfo(event, entry)}
                        aria-label={translate(
                          'notifications.namingIssuesInfoAction',
                        )}
                      >
                        <MdInfoOutline size={18} />
                      </IconButton>
                    </Tooltip>
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
                        primary={translate('ra.action.load_more', {
                          _: 'Load More…',
                        })}
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

      <Popover
        id="panel-naming-issue-info"
        anchorEl={infoAnchorEl}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'center' }}
        transformOrigin={{ vertical: 'top', horizontal: 'center' }}
        open={Boolean(infoAnchorEl)}
        onClose={handleCloseInfo}
      >
        <div className={classes.flow}>
          <Typography variant="caption" color="textSecondary">
            {translate('notifications.namingIssuesCurrentName')}
          </Typography>
          <Typography variant="body2" className={classes.flowName}>
            {infoEntry?.currentName}
          </Typography>
          <Typography variant="body1" className={classes.flowArrow}>
            ↓
          </Typography>
          <Typography variant="caption" color="textSecondary">
            {translate('notifications.namingIssuesNewName')}
          </Typography>
          <Typography variant="body2" className={classes.flowTo}>
            {infoEntry?.expectedName}
          </Typography>
        </div>
      </Popover>
    </div>
  )
}

export default NamingIssuesPanel
