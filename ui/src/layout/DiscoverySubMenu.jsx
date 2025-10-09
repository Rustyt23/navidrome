import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useDataProvider, useNotify } from 'react-admin'
import { useHistory, useLocation } from 'react-router-dom'
import {
  Collapse,
  CircularProgress,
  IconButton,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Typography,
  makeStyles,
} from '@material-ui/core'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import ChevronRightIcon from '@material-ui/icons/ChevronRight'
import ExploreIcon from '@material-ui/icons/Explore'
import FolderIcon from '@material-ui/icons/Folder'
import RefreshIcon from '@material-ui/icons/Refresh'
import SubMenu from './SubMenu'
import config from '../config'
import { useTheme } from '@material-ui/core/styles'
import { REST_URL } from '../consts'
import httpClient from '../dataProvider/httpClient'
import { baseUrl } from '../utils/urls'

const useStyles = makeStyles((theme) => ({
  listItem: {
    borderRadius: 6,
    marginRight: 4,
    paddingTop: 1,
    paddingBottom: 1,
    transition: 'all 0.2s ease-in-out',
    '&:hover': { backgroundColor: theme.palette.action.hover, transform: 'translateX(2px)' },
  },
  nested: { paddingLeft: theme.spacing(2) },
  listItemIcon: { minWidth: 28 },
  spinner: { margin: theme.spacing(1, 0, 1, 1) },
  active: { fontWeight: theme.typography.fontWeightMedium },
  spacer: { width: 24, flexShrink: 0 },
  toggleButton: { padding: 4, marginRight: 4 },
}))

const normalisePathSegments = (path) =>
  (path || '')
    .replace(/\\/g, '/')
    .split('/')
    .filter(Boolean)

const buildTree = (discoveries) => {
  if (!discoveries?.length) {
    return new Map([['', { folders: [], discoveries: [] }]])
  }

  const segmentsList = discoveries.map((disc) => normalisePathSegments(disc.path))
  let commonPrefix = segmentsList[0]
  for (let i = 1; i < segmentsList.length; i += 1) {
    const parts = segmentsList[i]
    const len = Math.min(commonPrefix.length, parts.length)
    const prefix = []
    for (let j = 0; j < len; j += 1) {
      if (commonPrefix[j] !== parts[j]) {
        break
      }
      prefix.push(commonPrefix[j])
    }
    commonPrefix = prefix
    if (commonPrefix.length === 0) {
      break
    }
  }

  const children = new Map()
  const ensureParent = (id) => {
    if (!children.has(id)) {
      children.set(id, { folders: [], discoveries: [] })
    }
    return children.get(id)
  }
  ensureParent('')

  const folderCache = new Map()

  const sortByName = (a, b) =>
    (a.name || '').localeCompare(b.name || '', undefined, { sensitivity: 'base', numeric: true })

  discoveries.forEach((disc) => {
    const parts = normalisePathSegments(disc.path)
    const relSegments = parts.slice(commonPrefix.length)
    const effectiveSegments = relSegments.length ? relSegments : [disc.name || disc.id]

    let parentId = ''
    effectiveSegments.forEach((segment, idx) => {
      const isLeaf = idx === effectiveSegments.length - 1
      if (isLeaf) {
        const parentChildren = ensureParent(parentId)
        parentChildren.discoveries.push({
          ...disc,
          type: 'discovery',
          parent_id: parentId,
        })
        parentChildren.discoveries.sort(sortByName)
      } else {
        const folderId = parentId ? `${parentId}/${segment}` : segment
        if (!folderCache.has(folderId)) {
          const folder = { id: folderId, name: segment, parent_id: parentId }
          folderCache.set(folderId, folder)
          const parentChildren = ensureParent(parentId)
          parentChildren.folders.push(folder)
          parentChildren.folders.sort(sortByName)
        }
        parentId = folderId
        ensureParent(parentId)
      }
    })
  })

  return children
}

const DiscoverySubMenu = ({ state, setState, sidebarIsOpen, dense }) => {
  const classes = useStyles()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const history = useHistory()
  const location = useLocation()
  const theme = useTheme()

  const [discoveries, setDiscoveries] = useState([])
  const [loading, setLoading] = useState(false)
  const [openMap, setOpenMap] = useState({})

  const fetchDiscoveryList = useCallback(async () => {
    const res = await dataProvider.getList('discovery', {
      pagination: { page: 1, perPage: config.maxSidebarPlaylists },
      sort: { field: 'name', order: 'ASC' },
      filter: {},
    })
    return res?.data || []
  }, [dataProvider])

  useEffect(() => {
    let cancelled = false

    const load = async () => {
      setLoading(true)
      try {
        const data = await fetchDiscoveryList()
        if (!cancelled) {
          setDiscoveries(data)
        }
      } catch {
        if (!cancelled) {
          notify('ra.page.error', 'warning')
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    load()
    return () => {
      cancelled = true
    }
  }, [fetchDiscoveryList, notify])

  const navigateToDiscovery = useCallback(() => {
    if (typeof window !== 'undefined' && window.location) {
      const target = baseUrl('#/discovery')
      window.location.assign(target)
    } else {
      history.push('/discovery')
    }
  }, [history])

  const handleRefresh = useCallback(async () => {
    setLoading(true)
    try {
      await httpClient(`${REST_URL}/discovery/sync`, { method: 'POST' })
      const data = await fetchDiscoveryList()
      setDiscoveries(data)
      notify('resources.discovery.notifications.synced', 'info', {
        _: 'Discovery folders scanned.',
      })
    } catch (error) {
      notify('ra.page.error', 'warning')
    } finally {
      setLoading(false)
      navigateToDiscovery()
    }
  }, [fetchDiscoveryList, navigateToDiscovery, notify])

  const childrenMap = useMemo(() => buildTree(discoveries), [discoveries])

  const handleToggle = useCallback(
    (menu) => {
      setState((prev) => ({ ...prev, [menu]: !prev[menu] }))
    },
    [setState]
  )

  const rootChildren = childrenMap.get('') || { folders: [], discoveries: [] }

  const isActive = useCallback((id) => location.pathname.includes(`/discovery/${id}`), [location.pathname])

  const renderDiscovery = useCallback(
    (disc, depth) => (
      <ListItem
        button
        key={disc.id}
        onClick={() => history.push(`/discovery/${disc.id}/show`)}
        className={classes.listItem}
        style={{ paddingLeft: theme.spacing(3) + depth * theme.spacing(2) }}
      >
        <span className={classes.spacer} />
        <ListItemIcon className={classes.listItemIcon}>
          <ExploreIcon fontSize="small" />
        </ListItemIcon>
        <ListItemText
          primary={
            <Typography
              variant="body2"
              noWrap
              className={isActive(disc.id) ? classes.active : undefined}
            >
              {disc.name}
            </Typography>
          }
        />
      </ListItem>
    ),
    [classes, history, isActive, theme]
  )

  const renderFolder = useCallback(
    (folder, depth = 0) => {
      const open = !!openMap[folder.id]
      const childGroup = childrenMap.get(folder.id) || { folders: [], discoveries: [] }

      const toggle = (event) => {
        event.stopPropagation()
        setOpenMap((prev) => ({ ...prev, [folder.id]: !open }))
      }

      return (
        <React.Fragment key={folder.id}>
          <ListItem
            button
            onClick={toggle}
            className={classes.listItem}
            style={{ paddingLeft: theme.spacing(3) + depth * theme.spacing(2) }}
          >
            <IconButton size="small" onClick={toggle} className={classes.toggleButton}>
              {open ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
            </IconButton>
            <ListItemIcon className={classes.listItemIcon}>
              <FolderIcon fontSize="small" />
            </ListItemIcon>
            <ListItemText
              primary={
                <Typography variant="body2" noWrap>
                  {folder.name}
                </Typography>
              }
            />
          </ListItem>
          <Collapse in={open} timeout="auto" unmountOnExit>
            <List disablePadding className={classes.nested}>
              {childGroup.folders.map((child) => renderFolder(child, depth + 1))}
              {childGroup.discoveries.map((disc) => renderDiscovery(disc, depth + 1))}
            </List>
          </Collapse>
        </React.Fragment>
      )
    },
    [childrenMap, classes, openMap, renderDiscovery, theme]
  )

  return (
    <SubMenu
      handleToggle={() => handleToggle('menuDiscovery')}
      isOpen={state.menuDiscovery}
      sidebarIsOpen={sidebarIsOpen}
      name="menu.discovery"
      icon={<ExploreIcon />}
      dense={dense}
      actionIcon={<RefreshIcon fontSize="small" />}
      onAction={handleRefresh}
    >
      <List disablePadding>
        {loading ? (
          <ListItem>
            <CircularProgress size={16} className={classes.spinner} />
          </ListItem>
        ) : (
          <>
            {rootChildren.folders.map((folder) => renderFolder(folder, 0))}
            {rootChildren.discoveries.map((disc) => renderDiscovery(disc, 0))}
          </>
        )}
      </List>
    </SubMenu>
  )
}

export default DiscoverySubMenu
