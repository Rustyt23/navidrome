import React, { useState, useCallback, memo, useEffect, useMemo, useRef } from 'react'
import { useDataProvider, useNotify, useRefresh } from 'react-admin'
import { useHistory } from 'react-router-dom'
import {
  Typography, List, ListItem, ListItemIcon, ListItemText,
  IconButton, makeStyles, Collapse, CircularProgress, Button,
} from '@material-ui/core'
import { BiCog } from 'react-icons/bi'
import { useDrop } from 'react-dnd'
import { RiFolder3Fill, RiPlayListFill } from 'react-icons/ri'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import ChevronRightIcon from '@material-ui/icons/ChevronRight'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import SubMenu from './SubMenu'
import { canChangeTracks } from '../common'
import { DraggableTypes } from '../consts'
import useDragAndDrop from '../common/useDragAndDrop'

const useStyles = makeStyles((theme) => ({
  listItem: {
    borderRadius: 6,
    marginRight: 4,
    paddingTop: 4,
    paddingBottom: 4,
    transition: 'all 0.2s ease-in-out',
    '&:hover': { backgroundColor: theme.palette.action.hover, transform: 'translateX(2px)' },
  },
  active: { backgroundColor: theme.palette.action.selected, fontWeight: theme.typography.fontWeightMedium },
  text: { transition: 'color 0.2s ease' },
  toggleButton: { padding: 4, marginRight: 4 },
  listItemIcon: { minWidth: 28 },
  spacer: { width: 24 },
  nested: { paddingLeft: theme.spacing(2) },
  depth: (props) => ({ paddingLeft: theme.spacing(2) + props.depth * theme.spacing(2) }),
  spinner: { marginLeft: 6 },
  showMoreButton: {
    textTransform: 'none',
    paddingLeft: 0,
    paddingRight: 0,
    justifyContent: 'flex-start',
    width: '100%',
    minWidth: 0,
  },
  showMoreSpinner: {
    marginLeft: 0,
  },
}))

// Sidebar lists render results in fixed batches so "Show more" can append in-place.
const MAX_SIDEBAR_ITEMS = 200

const parentKey = (id) => (id == null || id === '' ? '' : String(id))
const parentFilterValue = parentKey

const mergeUniqueItems = (prev = [], next = []) => {
  if (!prev.length) return [...next]

  const indexByKey = new Map()
  prev.forEach((item, index) => {
    indexByKey.set(`${item.type}:${item.id}`, index)
  })

  const merged = [...prev]
  next.forEach((item) => {
    const key = `${item.type}:${item.id}`
    if (indexByKey.has(key)) {
      const idx = indexByKey.get(key)
      merged[idx] = { ...merged[idx], ...item }
    } else {
      indexByKey.set(key, merged.length)
      merged.push(item)
    }
  })

  return merged
}

const useChildrenStore = () => {
  const [store, setStore] = useState({})
  const inFlightRef = useRef(new Map())

  const setItems = useCallback((parentId, items, { append = false, total, batchSize } = {}) => {
    const key = parentKey(parentId)
    const enriched = (items || []).map((it) => ({
      ...it,
      parent_id:
        it.parent_id ?? it.parentId ?? it.folder_id ?? it.folderId ?? key,
    }))
    setStore((s) => {
      const prev = s[key] || { items: [], total: undefined, hasMore: false }
      const merged = append ? mergeUniqueItems(prev.items, enriched) : enriched
      const pageSize = batchSize || 0
      let totalCount = prev.total

      if (typeof total === 'number') {
        totalCount = total
      } else if (!pageSize || (items || []).length < pageSize) {
        totalCount = append ? merged.length : (items || []).length
      }

      const hasMore =
        typeof totalCount === 'number'
          ? merged.length < totalCount
          : !!pageSize && (items || []).length === pageSize
      return {
        ...s,
        [key]: {
          items: merged,
          dirty: false,
          cached: true,
          total: totalCount,
          hasMore,
        },
      }
    })
  }, [])

  const markDirty = useCallback((parentIds) => {
    setStore((s) => {
      const next = { ...s }
      parentIds.forEach((pid) => {
        const key = parentKey(pid)
        const prev =
          next[key] || { items: [], cached: false, total: undefined, hasMore: false }
        next[key] = { ...prev, dirty: true, cached: !!prev.cached }
      })
      return next
    })
  }, [])

  const get = useCallback(
    (pid) =>
      store[parentKey(pid)] || {
        items: [],
        dirty: true,
        cached: false,
        total: undefined,
        hasMore: false,
      },
    [store]
  )

  const ensure = useCallback(
    async (parentId, fetcher, { append = false, force = false, batchSize } = {}) => {
      const key = parentKey(parentId)
      const entry = store[key]
      if (!force && entry && entry.cached && !entry.dirty) return entry.items

      const inFlight = inFlightRef.current.get(key)
      if (inFlight) return inFlight

      const p = (async () => {
        const result = await fetcher()
        const payload = Array.isArray(result)
          ? { items: result, total: undefined }
          : result || { items: [], total: undefined }
        setItems(parentId, payload.items || [], {
          append,
          total: payload.total,
          batchSize,
        })
        inFlightRef.current.delete(key)
        return payload.items
      })().catch((e) => {
        inFlightRef.current.delete(key)
        throw e
      })
      inFlightRef.current.set(key, p)
      return p
    },
    [setItems, store]
  )

  const moveItem = useCallback((item, fromPid, toPid) => {
    setStore((s) => {
      const next = { ...s }
      const fromKey = parentKey(fromPid)
      const toKey = parentKey(toPid)

      if (next[fromKey]?.items) {
        next[fromKey] = {
          ...next[fromKey],
          items: next[fromKey].items.filter((i) => !(i.id === item.id && i.type === item.type)),
        }
      }
      if (next[toKey]?.items) {
        const withoutDup = next[toKey].items.filter((i) => !(i.id === item.id && i.type === item.type))
        next[toKey] = {
          ...next[toKey],
          items: [...withoutDup, { ...item, parent_id: toPid }],
        }
      }
      return next
    })
  }, [])

  return { get, setItems, markDirty, moveItem, ensure }
}

const PlaylistMenuItemLink = memo(({ pls, depth = 0 }) => {
  const classes = useStyles({ depth })
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const history = useHistory()

  const parentIdForDnD = pls.parent_id ?? ''

  const { dragDropRef, isDragging } = useDragAndDrop(
    DraggableTypes.PLAYLIST,
    { id: pls.id, type: 'playlist', parentId: parentIdForDnD },
    canChangeTracks(pls) ? DraggableTypes.ALL : [],
    (item) =>
      dataProvider
        .addToPlaylist(pls.id, item)
        .then((res) => notify('message.songsAddedToPlaylist', 'info', { smart_count: res?.data?.added }))
        .catch(() => notify('ra.page.error', 'warning'))
  )

  return (
    <ListItem
      button
      onClick={() => history.push(`/playlist/${pls.id}/show`)}
      className={`${classes.listItem} ${classes.depth}`}
      ref={dragDropRef}
      style={{ opacity: isDragging ? 0.5 : 1 }}
    >
      <span className={classes.spacer} />
      <ListItemIcon className={classes.listItemIcon}><RiPlayListFill /></ListItemIcon>
      <ListItemText primary={<Typography variant="body2" noWrap className={classes.text}>{pls.name}</Typography>} />
    </ListItem>
  )
})
PlaylistMenuItemLink.displayName = 'PlaylistMenuItemLink'

const FolderRow = memo(function FolderRow({
  node,
  depth,
  open,
  openMap = {},
  setOpenMap,
  childrenStore,
  onAnyMove,
}) {
  const classes = useStyles({ depth })
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const history = useHistory()
  const refresh = useRefresh()
  const [loading, setLoading] = useState(false)
  const [loadingMore, setLoadingMore] = useState(false)

  const { get, markDirty, moveItem, ensure } = childrenStore
  const { items, dirty, cached, hasMore, total } = get(node.id)

  const fetchChildrenOnce = useCallback(async () => {
    if (!open) return
    if (!dirty && cached) return
    setLoading(true)
    try {
      await ensure(
        node.id,
        async () => {
          const res = await dataProvider.getList('folder', {
            pagination: { page: 1, perPage: MAX_SIDEBAR_ITEMS },
            sort: { field: 'name', order: 'ASC' },
            filter: { parent_id: parentFilterValue(node.id) },
          })
          return { items: res?.data || [], total: res?.total }
        },
        { batchSize: MAX_SIDEBAR_ITEMS }
      )
    } catch {
      notify('ra.page.error', 'warning')
    } finally {
      setLoading(false)
    }
  }, [open, dirty, cached, ensure, node.id, dataProvider, notify])

  useEffect(() => { fetchChildrenOnce() }, [fetchChildrenOnce])

  const toggle = (e) => {
    e.stopPropagation()
    setOpenMap((s) => ({ ...s, [node.id]: !open }))
  }

  const parentIdForDnD = node.parent_id ?? ''

  const { dragDropRef, isDragging } = useDragAndDrop(
    DraggableTypes.FOLDER,
    { id: node.id, type: 'folder', parentId: parentIdForDnD },
    [DraggableTypes.FOLDER, DraggableTypes.PLAYLIST],
    async (item) => {
      try {
        if (item.id === node.id) return

        const sourceParentId = item.parentId ?? ''

        if (item.type === 'playlist') {
          await dataProvider.setPlaylistFolder({
            playlistId: item.id,
            targetFolderId: node.id,
            sourceParentId,
          })
        } else if (item.type === 'folder') {
          await dataProvider.moveFolder({
            folderId: item.id,
            targetParentId: node.id,
            sourceParentId,
          })
        }

        moveItem(
          { id: item.id, type: item.type, name: item.name ?? '', parent_id: node.id },
          sourceParentId,
          node.id
        )
        markDirty([node.id])

        refresh()
        onAnyMove?.()
        await fetchChildrenOnce()
      } catch {
        notify('ra.page.error', 'warning')
      }
    }
  )

  const childrenTyped = useMemo(() => ({
    folders: (items || []).filter((i) => i.type === 'folder'),
    playlists: (items || []).filter((i) => i.type === 'playlist'),
  }), [items])

  const remainingCount = useMemo(() => {
    if (typeof total !== 'number') return undefined
    const diff = total - (items?.length || 0)
    return diff > 0 ? diff : 0
  }, [total, items])

  const loadMoreChildren = useCallback(async () => {
    if (!hasMore || loadingMore) return
    setLoadingMore(true)
    try {
      // Keep the already loaded chunk and request the next page based on the batch size.
      const currentCount = items?.length || 0
      const nextPage = Math.ceil(currentCount / MAX_SIDEBAR_ITEMS) + 1
      await ensure(
        node.id,
        async () => {
          const res = await dataProvider.getList('folder', {
            pagination: { page: nextPage, perPage: MAX_SIDEBAR_ITEMS },
            sort: { field: 'name', order: 'ASC' },
            filter: { parent_id: parentFilterValue(node.id) },
          })
          return { items: res?.data || [], total: res?.total }
        },
        { append: true, force: true, batchSize: MAX_SIDEBAR_ITEMS }
      )
    } catch {
      notify('ra.page.error', 'warning')
    } finally {
      setLoadingMore(false)
    }
  }, [hasMore, loadingMore, items, ensure, node.id, dataProvider, notify])

  return (
    <>
      <ListItem
        button
        onClick={() => history.push(`/folder/${node.id}/show`)}
        className={`${classes.listItem} ${classes.depth}`}
        ref={dragDropRef}
        style={{ opacity: isDragging ? 0.5 : 1 }}
      >
        <IconButton size="small" className={classes.toggleButton} onClick={toggle}>
          {open ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
        </IconButton>
        <ListItemIcon className={classes.listItemIcon}><RiFolder3Fill /></ListItemIcon>
        <ListItemText primary={<Typography variant="body2" noWrap className={classes.text}>{node.name}</Typography>} />
        {loading && <CircularProgress size={14} className={classes.spinner} />}
      </ListItem>

      <Collapse in={open} timeout="auto" unmountOnExit>
        <List disablePadding className={classes.nested}>
          {childrenTyped.folders.map((f) => (
            <FolderRow
              key={f.id}
              node={f}
              depth={depth + 1}
              open={!!openMap[f.id]}
              openMap={openMap}
              setOpenMap={setOpenMap}
              childrenStore={childrenStore}
              onAnyMove={onAnyMove}
            />
          ))}
          {childrenTyped.playlists.map((p) => (
            <PlaylistMenuItemLink key={p.id} pls={p} depth={depth + 1} />
          ))}
          {hasMore && (
            <ListItem className={`${classes.listItem} ${classes.depth}`}>
              <span className={classes.spacer} />
              <Button
                size="small"
                color="primary"
                onClick={loadMoreChildren}
                disabled={loadingMore}
                className={classes.showMoreButton}
                startIcon={
                  loadingMore ? (
                    <CircularProgress size={14} color="inherit" className={classes.showMoreSpinner} />
                  ) : null
                }
              >
                {`Show more${
                  typeof remainingCount === 'number' && remainingCount > 0
                    ? ` (${remainingCount})`
                    : ''
                }`}
              </Button>
            </ListItem>
          )}
        </List>
      </Collapse>
    </>
  )
}, (prev, next) => {
  return (
    prev.node.id === next.node.id &&
    prev.open === next.open &&
    prev.depth === next.depth &&
    prev.childrenStore === next.childrenStore
  )
})

const PlaylistsSubMenu = ({ state, setState, sidebarIsOpen, dense }) => {
  const history = useHistory()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const refresh = useRefresh()
  const classes = useStyles()
  const [openMap, setOpenMap] = useState({})
  const childrenStore = useChildrenStore()
  const [loadingMoreRoot, setLoadingMoreRoot] = useState(false)

  const handleToggle = (menu) => setState((s) => ({ ...s, [menu]: !s[menu] }))

  const onPlaylistConfig = useCallback(() => {
    history.push({ pathname: '/folder', state: { parentId: null } })
  }, [history])

  const { get, markDirty, ensure, moveItem } = childrenStore
  const {
    items: rootItems,
    dirty: rootDirty,
    cached: rootCached,
    hasMore: rootHasMore,
    total: rootTotal,
  } = get('')

  const fetchRootOnce = useCallback(async () => {
    if (!rootDirty && rootCached) return
    try {
      await ensure(
        '',
        async () => {
          const res = await dataProvider.getList('folder', {
            pagination: { page: 1, perPage: MAX_SIDEBAR_ITEMS },
            sort: { field: 'name', order: 'ASC' },
            filter: { parent_id: parentFilterValue('') },
          })
          return { items: res?.data || [], total: res?.total }
        },
        { batchSize: MAX_SIDEBAR_ITEMS }
      )
    } catch {
      notify('ra.page.error', 'warning')
    }
  }, [rootDirty, rootCached, ensure, dataProvider, notify])

  useEffect(() => { fetchRootOnce() }, [fetchRootOnce])

  useEffect(() => {
    const onChanged = (e) => {
      const { sourceParentId, targetParentId } = e.detail || {}
      const targets = []
      if (sourceParentId !== undefined) targets.push(parentKey(sourceParentId))
      if (targetParentId !== undefined) targets.push(parentKey(targetParentId))
      if (!targets.length) targets.push('')
      markDirty(targets)

      refresh()
    }
    window.addEventListener('folder:changed', onChanged)
    return () => window.removeEventListener('folder:changed', onChanged)
  }, [markDirty, refresh])

  const [, dropRef] = useDrop(() => ({
    accept: [DraggableTypes.PLAYLIST, DraggableTypes.FOLDER],
    drop: async (item) => {
      try {
        const sourceParentId = item.parentId ?? ''

        if (item.type === 'playlist') {
          await dataProvider.setPlaylistFolder({
            playlistId: item.id,
            targetFolderId: null,
            sourceParentId,
          })
        } else if (item.type === 'folder') {
          await dataProvider.moveFolder({
            folderId: item.id,
            targetParentId: null,
            sourceParentId,
          })
        }
        moveItem(
          { id: item.id, type: item.type, name: item.name ?? '', parent_id: '' },
          sourceParentId,
          ''
        )
        markDirty([''])

        refresh()
      } catch {
        notify('ra.page.error', 'warning')
      }
    },
  }), [dataProvider, notify, moveItem, markDirty, refresh])

  const folders = useMemo(() => (rootItems || []).filter((i) => i.type === 'folder'), [rootItems])
  const playlists = useMemo(() => (rootItems || []).filter((i) => i.type === 'playlist'), [rootItems])

  const remainingRoot = useMemo(() => {
    if (typeof rootTotal !== 'number') return undefined
    const diff = rootTotal - (rootItems?.length || 0)
    return diff > 0 ? diff : 0
  }, [rootTotal, rootItems])

  const loadMoreRoot = useCallback(async () => {
    if (!rootHasMore || loadingMoreRoot) return
    setLoadingMoreRoot(true)
    try {
      // Sidebar root uses the same batching strategy as nested folders.
      const currentCount = rootItems?.length || 0
      const nextPage = Math.ceil(currentCount / MAX_SIDEBAR_ITEMS) + 1
      await ensure(
        '',
        async () => {
          const res = await dataProvider.getList('folder', {
            pagination: { page: nextPage, perPage: MAX_SIDEBAR_ITEMS },
            sort: { field: 'name', order: 'ASC' },
            filter: { parent_id: parentFilterValue('') },
          })
          return { items: res?.data || [], total: res?.total }
        },
        { append: true, force: true, batchSize: MAX_SIDEBAR_ITEMS }
      )
    } catch {
      notify('ra.page.error', 'warning')
    } finally {
      setLoadingMoreRoot(false)
    }
  }, [rootHasMore, loadingMoreRoot, rootItems, ensure, dataProvider, notify])

  return (
    <SubMenu
      handleToggle={() => handleToggle('menuPlaylists')}
      isOpen={state.menuPlaylists}
      sidebarIsOpen={sidebarIsOpen}
      name={'menu.playlists'}
      icon={<QueueMusicIcon />}
      dense={dense}
      actionIcon={<BiCog />}
      onAction={onPlaylistConfig}
      ref={dropRef}
    >
      <List disablePadding>
        {!rootCached && rootDirty ? (
          <ListItem><CircularProgress size={16} className={classes.spinner} /></ListItem>
        ) : (
          <>
            {folders.map((f) => (
              <FolderRow
                key={f.id}
                node={f}
                depth={0}
                open={!!openMap[f.id]}
                openMap={openMap}
                setOpenMap={setOpenMap}
                childrenStore={childrenStore}
                onAnyMove={refresh}
              />
            ))}
            {playlists.map((p) => (
              <PlaylistMenuItemLink key={p.id} pls={p} depth={0} />
            ))}
            {rootHasMore && (
              <ListItem className={classes.listItem}>
                <span className={classes.spacer} />
                <Button
                  size="small"
                  color="primary"
                  onClick={loadMoreRoot}
                  disabled={loadingMoreRoot}
                  className={classes.showMoreButton}
                  startIcon={
                    loadingMoreRoot ? (
                      <CircularProgress size={14} color="inherit" className={classes.showMoreSpinner} />
                    ) : null
                  }
                >
                  {`Show more${
                    typeof remainingRoot === 'number' && remainingRoot > 0
                      ? ` (${remainingRoot})`
                      : ''
                  }`}
                </Button>
              </ListItem>
            )}
          </>
        )}
      </List>
    </SubMenu>
  )
}

export default PlaylistsSubMenu
