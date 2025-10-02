import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useHistory, useLocation } from 'react-router-dom'
import {
  CircularProgress,
  Collapse,
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
import FolderIcon from '@material-ui/icons/Folder'
import { BiCog } from 'react-icons/bi'
import { RiFolder3Fill } from 'react-icons/ri'
import { useNotify, useTranslate } from 'react-admin'
import SubMenu from './SubMenu'
import httpClient from '../dataProvider/httpClient'
import { addDiscoveryChangedListener } from '../discovery_folder/events'

const useStyles = makeStyles((theme) => ({
  listItem: {
    borderRadius: 6,
    marginRight: 4,
    paddingTop: 1,
    paddingBottom: 1,
    transition: 'all 0.2s ease-in-out',
    '&:hover': { backgroundColor: theme.palette.action.hover, transform: 'translateX(2px)' },
  },
  active: {
    backgroundColor: theme.palette.action.selected,
    fontWeight: theme.typography.fontWeightMedium,
  },
  text: { transition: 'color 0.2s ease' },
  toggleButton: { padding: 4, marginRight: 4 },
  listItemIcon: { minWidth: 28 },
  spacer: { width: 24 },
  nested: { paddingLeft: theme.spacing(2) },
  depth: (props) => ({ paddingLeft: theme.spacing(2) + props.depth * theme.spacing(2) }),
  spinner: { marginLeft: 6 },
}))

const fetchDiscoveryFolders = async (path) => {
  const query = path ? `?path=${encodeURIComponent(path)}` : ''
  const { json } = await httpClient(`/api/discoveryfs/list${query}`)
  const folders = Array.isArray(json?.folders) ? json.folders : []
  return folders.map((item) => ({
    name: item.name,
    path: item.path,
  }))
}

const DiscoveryFolderRow = ({
  node,
  depth,
  openMap,
  setOpenMap,
  onFolderSelected,
  activePath,
  version,
}) => {
  const classes = useStyles({ depth })
  const notify = useNotify()
  const [loading, setLoading] = useState(false)
  const [children, setChildren] = useState([])

  const isOpen = !!openMap[node.path]

  const loadChildren = useCallback(async () => {
    if (!isOpen) return
    setLoading(true)
    try {
      const folders = await fetchDiscoveryFolders(node.path)
      setChildren(folders)
    } catch (error) {
      notify('ra.page.error', 'warning')
    } finally {
      setLoading(false)
    }
  }, [isOpen, node.path, notify])

  useEffect(() => { loadChildren() }, [loadChildren, version])

  const toggle = (event) => {
    event.stopPropagation()
    setOpenMap((state) => ({ ...state, [node.path]: !isOpen }))
  }

  const handleClick = () => {
    onFolderSelected(node.path)
  }

  return (
    <>
      <ListItem
        button
        onClick={handleClick}
        className={`${classes.listItem} ${classes.depth} ${activePath === node.path ? classes.active : ''}`}
      >
        <IconButton size="small" className={classes.toggleButton} onClick={toggle}>
          {isOpen ? <ExpandMoreIcon fontSize="small" /> : <ChevronRightIcon fontSize="small" />}
        </IconButton>
        <ListItemIcon className={classes.listItemIcon}>
          <RiFolder3Fill />
        </ListItemIcon>
        <ListItemText
          primary={
            <Typography variant="body2" noWrap className={classes.text}>
              {node.name}
            </Typography>
          }
        />
        {loading && <CircularProgress size={14} className={classes.spinner} />}
      </ListItem>
      <Collapse in={isOpen} timeout="auto" unmountOnExit>
        <List disablePadding className={classes.nested}>
          {children.map((child) => (
            <DiscoveryFolderRow
              key={child.path}
              node={child}
              depth={depth + 1}
              openMap={openMap}
              setOpenMap={setOpenMap}
              onFolderSelected={onFolderSelected}
              activePath={activePath}
              version={version}
            />
          ))}
        </List>
      </Collapse>
    </>
  )
}

const DiscoverySubMenu = ({ state, setState, sidebarIsOpen, dense }) => {
  const classes = useStyles({ depth: 0 })
  const history = useHistory()
  const location = useLocation()
  const notify = useNotify()
  const translate = useTranslate()
  const [rootFolders, setRootFolders] = useState([])
  const [loadingRoot, setLoadingRoot] = useState(false)
  const [openMap, setOpenMap] = useState({})
  const [version, setVersion] = useState(0)

  const activePath = useMemo(() => {
    const params = new URLSearchParams(location.search)
    const raw = params.get('path') || ''
    return raw.replace(/^\/+/, '')
  }, [location.search])

  const ensureRoot = useCallback(async () => {
    setLoadingRoot(true)
    try {
      const folders = await fetchDiscoveryFolders('')
      setRootFolders(folders)
    } catch (error) {
      notify('ra.page.error', 'warning')
    } finally {
      setLoadingRoot(false)
    }
  }, [notify])

  useEffect(() => {
    ensureRoot()
  }, [ensureRoot])

  useEffect(() => {
    return addDiscoveryChangedListener(() => {
      ensureRoot()
      setVersion((v) => v + 1)
    })
  }, [ensureRoot])

  useEffect(() => {
    const segments = activePath ? activePath.split('/') : []
    if (!segments.length) return
    setOpenMap((prev) => {
      const next = { ...prev }
      let cumulative = ''
      segments.forEach((segment) => {
        cumulative = cumulative ? `${cumulative}/${segment}` : segment
        next[cumulative] = true
      })
      return next
    })
  }, [activePath])

  const handleToggle = useCallback(
    (menu) => setState((prev) => ({ ...prev, [menu]: !prev[menu] })),
    [setState],
  )

  const goToFolder = useCallback(
    (path) => {
      const query = path ? `?path=${encodeURIComponent(path)}` : ''
      history.push(`/discovery${query}`)
    },
    [history],
  )

  return (
    <SubMenu
      handleToggle={() => handleToggle('menuDiscovery')}
      isOpen={state.menuDiscovery}
      sidebarIsOpen={sidebarIsOpen}
      name={'menu.discovery'}
      icon={<FolderIcon />}
      dense={dense}
      actionIcon={<BiCog />}
      onAction={() => goToFolder('')}
    >
      <List disablePadding>
        {loadingRoot ? (
          <ListItem>
            <CircularProgress size={16} className={classes.spinner} />
          </ListItem>
        ) : rootFolders.length === 0 ? (
          <ListItem>
            <ListItemText
              primary={
                <Typography variant="body2" color="textSecondary">
                  {translate('resources.discovery.empty', { _: 'This folder is empty.' })}
                </Typography>
              }
            />
          </ListItem>
        ) : (
          rootFolders.map((folder) => (
            <DiscoveryFolderRow
              key={folder.path}
              node={folder}
              depth={0}
              openMap={openMap}
              setOpenMap={setOpenMap}
              onFolderSelected={goToFolder}
              activePath={activePath}
              version={version}
            />
          ))
        )}
      </List>
    </SubMenu>
  )
}

export default DiscoverySubMenu
