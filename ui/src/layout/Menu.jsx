import React, { useCallback, useMemo, useState } from 'react'
import { useSelector } from 'react-redux'
import {
  Collapse,
  Divider,
  ListItemIcon,
  ListItemText,
  makeStyles,
  useTheme,
} from '@material-ui/core'
import clsx from 'clsx'
import { useTranslate, MenuItemLink, getResources } from 'react-admin'
import ViewListIcon from '@material-ui/icons/ViewList'
import AlbumIcon from '@material-ui/icons/Album'
import MenuItem from '@material-ui/core/MenuItem'
import SpeakerGroupIcon from '@material-ui/icons/SpeakerGroup'
import ChevronRightIcon from '@material-ui/icons/ChevronRight'
import ExpandMoreIcon from '@material-ui/icons/ExpandMore'
import FolderIcon from '@material-ui/icons/Folder'
import { useHistory } from 'react-router-dom'
import { fade } from '@material-ui/core/styles/colorManipulator'
import { BiCog } from 'react-icons/bi'
import SubMenu from './SubMenu'
import { humanize, pluralize } from 'inflection'
import albumLists from '../album/albumLists'
import PlaylistsSubMenu from './PlaylistsSubMenu'
import DiscoverySubMenu from './DiscoverySubMenu'
import LibrarySelector from '../common/LibrarySelector'
import config from '../config'
import { useRetailPlayerDeviceStore } from '../retailPlayer/RetailPlayerDeviceStoreContext'

const useStyles = makeStyles((theme) => ({
  root: {
    marginTop: theme.spacing(1),
    marginBottom: theme.spacing(1),
    transition: theme.transitions.create('width', {
      easing: theme.transitions.easing.sharp,
      duration: theme.transitions.duration.leavingScreen,
    }),
    paddingBottom: (props) => (props.addPadding ? '80px' : '20px'),
  },
  open: {
    width: 240,
  },
  closed: {
    width: 55,
  },
  active: {
    color: theme.palette.text.primary,
    fontWeight: 'bold',
  },
  retailPlayerSubItem: {
    paddingTop: theme.spacing(0.5),
    paddingBottom: theme.spacing(0.5),
    fontSize: theme.typography.pxToRem(13),
    '& .RaMenuItemLink-primaryText': {
      fontSize: theme.typography.pxToRem(13),
    },
  },
  folderItem: {
    paddingTop: theme.spacing(0.5),
    paddingBottom: theme.spacing(0.5),
    fontSize: theme.typography.pxToRem(14),
    '& .MuiListItemIcon-root': {
      minWidth: theme.spacing(4),
    },
    '& .MuiListItemIcon-root svg': {
      fontSize: theme.typography.pxToRem(16),
    },
    '& .MuiTypography-body1': {
      fontSize: theme.typography.pxToRem(14),
      color: theme.palette.common.white,
    },
  },
  folderDropTarget: {
    borderRadius: theme.shape.borderRadius / 2,
    transition: theme.transitions.create(['background-color', 'box-shadow'], {
      duration: theme.transitions.duration.shorter,
    }),
  },
  folderDropTargetActive: {
    backgroundColor: fade(theme.palette.primary.main, 0.18),
    boxShadow: `inset 0 0 0 2px ${fade(theme.palette.primary.main, 0.35)}`,
  },
  folderChildren: {
    '& > *': {
      width: '100%',
    },
  },
  deviceItem: {
    paddingTop: theme.spacing(0.5),
    paddingBottom: theme.spacing(0.5),
    fontSize: theme.typography.pxToRem(12),
    '& .RaMenuItemLink-icon': {
      minWidth: theme.spacing(4),
      color: theme.palette.common.white,
    },
    '& .RaMenuItemLink-primaryText': {
      color: theme.palette.primary.main,
    },
  },
  deviceIcon: {
    color: theme.palette.common.white,
    fontSize: theme.typography.pxToRem(16),
  },
}))

const translatedResourceName = (resource, translate) =>
  translate(`resources.${resource.name}.name`, {
    smart_count: 2,
    _:
      resource.options && resource.options.label
        ? translate(resource.options.label, {
            smart_count: 2,
            _: resource.options.label,
          })
        : humanize(pluralize(resource.name)),
  })

const Menu = ({ dense = false }) => {
  const open = useSelector((state) => state.admin.ui.sidebarOpen)
  const translate = useTranslate()
  const queue = useSelector((state) => state.player?.queue)
  const classes = useStyles({ addPadding: queue.length > 0 })
  const theme = useTheme()
  const resources = useSelector(getResources).filter(
    (r) => r.name !== 'radio' && r.name !== 'share',
  )
  const history = useHistory()

  // TODO State is not persisted in mobile when you close the sidebar menu. Move to redux?
  const [state, setState] = useState({
    menuAlbumList: false,
    menuPlaylists: true,
    menuDiscovery: true,
    menuSharedPlaylists: true,
    menuRetailPlayer: true,
  })

  const handleToggle = (menu) => {
    setState((state) => ({ ...state, [menu]: !state[menu] }))
  }

  const renderResourceMenuItemLink = (resource) => (
    <MenuItemLink
      key={resource.name}
      to={`/${resource.name}`}
      activeClassName={classes.active}
      primaryText={translatedResourceName(resource, translate)}
      leftIcon={resource.icon || <ViewListIcon />}
      sidebarIsOpen={open}
      dense={dense}
    />
  )

  const renderAlbumMenuItemLink = (type, al) => {
    const resource = resources.find((r) => r.name === 'album')
    if (!resource) {
      return null
    }

    const albumListAddress = `/album/${type}`

    const name = translate(`resources.album.lists.${type || 'default'}`, {
      _: translatedResourceName(resource, translate),
    })

    return (
      <MenuItemLink
        key={albumListAddress}
        to={albumListAddress}
        activeClassName={classes.active}
        primaryText={name}
        leftIcon={al.icon || <ViewListIcon />}
        sidebarIsOpen={open}
        dense={dense}
        exact
      />
    )
  }

  const subItems = (subMenu) => (resource) =>
    resource.hasList && resource.options && resource.options.subMenu === subMenu

  const {
    state: {
      tree: retailTree,
      loading: retailDevicesLoading,
      error: retailDevicesError,
      folders: retailFolders,
      devices: retailDevices,
    },
    actions: { updateDevice },
    dragState: {
      draggedDevice,
      dropTargetFolderId,
      setDropTargetFolderId,
      setLastDeviceDrop,
    },
  } = useRetailPlayerDeviceStore()

  const folderMap = useMemo(() => {
    const map = new Map()
    if (Array.isArray(retailFolders)) {
      retailFolders.forEach((folder) => {
        if (folder?.id) {
          map.set(folder.id, folder)
        }
      })
    }
    return map
  }, [retailFolders])

  const deviceMap = useMemo(() => {
    const map = new Map()
    if (Array.isArray(retailDevices)) {
      retailDevices.forEach((device) => {
        if (device?.id) {
          map.set(device.id, device)
        }
      })
    }
    return map
  }, [retailDevices])

  const draggedDeviceId = draggedDevice?.id || null

  const canDropDeviceOnFolder = useCallback(
    (deviceId, folderId) => {
      if (!deviceId || !folderId) {
        return false
      }
      if (!folderMap.has(folderId)) {
        return false
      }
      const device = deviceMap.get(deviceId)
      if (!device) {
        return false
      }
      if (draggedDevice?.sourceFolderId === folderId) {
        return false
      }
      const currentFolders = Array.isArray(device.folderIds)
        ? device.folderIds.filter(Boolean)
        : device.folderId
        ? [device.folderId].filter(Boolean)
        : []
      if (currentFolders.length === 1 && currentFolders[0] === folderId) {
        return false
      }
      return true
    },
    [deviceMap, folderMap, draggedDevice],
  )

  const extractDeviceIdFromEvent = useCallback(
    (event) => {
      if (draggedDeviceId) {
        return draggedDeviceId
      }
      const transfer = event?.dataTransfer
      if (!transfer) {
        return null
      }
      try {
        const raw = transfer.getData('application/json')
        if (raw) {
          const parsed = JSON.parse(raw)
          if (parsed?.deviceId) {
            return parsed.deviceId
          }
        }
      } catch (error) {
        // Ignore malformed data
      }
      try {
        const fallback = transfer.getData('text/plain')
        if (fallback) {
          return fallback
        }
      } catch (error) {
        // Ignore malformed data
      }
      return null
    },
    [draggedDeviceId],
  )

  const handleFolderDragOver = useCallback(
    (event, folderId) => {
      if (!draggedDeviceId) {
        return
      }
      if (!canDropDeviceOnFolder(draggedDeviceId, folderId)) {
        return
      }
      event.preventDefault()
      if (event.dataTransfer) {
        event.dataTransfer.dropEffect = 'move'
      }
    },
    [draggedDeviceId, canDropDeviceOnFolder],
  )

  const handleFolderDragEnter = useCallback(
    (event, folderId) => {
      if (!draggedDeviceId) {
        return
      }
      if (!canDropDeviceOnFolder(draggedDeviceId, folderId)) {
        return
      }
      event.preventDefault()
      setDropTargetFolderId(folderId)
    },
    [draggedDeviceId, canDropDeviceOnFolder, setDropTargetFolderId],
  )

  const handleFolderDragLeave = useCallback(
    (event, folderId) => {
      if (!draggedDeviceId) {
        return
      }
      const nextTarget = event.relatedTarget
      if (nextTarget && event.currentTarget.contains(nextTarget)) {
        return
      }
      setDropTargetFolderId((previous) => (previous === folderId ? null : previous))
    },
    [draggedDeviceId, setDropTargetFolderId],
  )

  const handleDeviceDropOnFolder = useCallback(
    (deviceId, folderId) => {
      if (!deviceId || !folderId) {
        return
      }
      const device = deviceMap.get(deviceId)
      if (!device) {
        return
      }
      const existingFolderIds = Array.isArray(device.folderIds)
        ? device.folderIds.filter(Boolean)
        : device.folderId
        ? [device.folderId].filter(Boolean)
        : []
      if (existingFolderIds.length === 1 && existingFolderIds[0] === folderId) {
        return
      }
      updateDevice({ id: deviceId, folderIds: [folderId] })
      setLastDeviceDrop({ deviceId, folderId, timestamp: Date.now() })
    },
    [deviceMap, updateDevice, setLastDeviceDrop],
  )

  const handleFolderDrop = useCallback(
    (event, folderId) => {
      const deviceId = extractDeviceIdFromEvent(event)
      if (!deviceId) {
        return
      }
      if (!canDropDeviceOnFolder(deviceId, folderId)) {
        return
      }
      event.preventDefault()
      event.stopPropagation()
      handleDeviceDropOnFolder(deviceId, folderId)
      setDropTargetFolderId(null)
    },
    [
      extractDeviceIdFromEvent,
      canDropDeviceOnFolder,
      handleDeviceDropOnFolder,
      setDropTargetFolderId,
    ],
  )

  const [openFolders, setOpenFolders] = useState({})

  const toggleFolder = useCallback((folderId) => {
    setOpenFolders((prev) => ({
      ...prev,
      [folderId]: !prev[folderId],
    }))
  }, [])

  const goToRetailPlayerSettings = useCallback(() => {
    history.push('/retail-player/devices')
  }, [history])

  const renderDeviceLink = useCallback(
    (node, depth) => {
      const slug = node.slug || node.name || node.id
      const encodedSlug = encodeURIComponent(slug)
      const padding = theme.spacing(4 + depth * 2)
      return (
        <MenuItemLink
          key={`retailplayer-${node.apiId || node.id}`}
          to={`/retailplayer/${encodedSlug}`}
          activeClassName={classes.active}
          primaryText={node.name}
          leftIcon={
            <SpeakerGroupIcon fontSize="small" className={classes.deviceIcon} />
          }
          sidebarIsOpen={open}
          dense={dense}
          exact
          className={classes.deviceItem}
          style={{ paddingLeft: padding }}
        />
      )
    },
    [classes.active, classes.deviceIcon, classes.deviceItem, dense, open, theme],
  )

  const renderRetailPlayerNodes = useCallback(
    (nodes, depth = 0) =>
      nodes.map((node) => {
        if (node.type === 'folder') {
          const isOpen = openFolders[node.id] ?? false
          const padding = theme.spacing(4 + depth * 2)
          const childPadding = theme.spacing(2)
          const isActiveDropTarget =
            dropTargetFolderId === node.id &&
            !!draggedDeviceId &&
            canDropDeviceOnFolder(draggedDeviceId, node.id)
          return (
            <React.Fragment key={`retailfolder-${node.id}`}>
              <MenuItem
                dense={dense}
                className={clsx(classes.folderItem, classes.folderDropTarget, {
                  [classes.folderDropTargetActive]: isActiveDropTarget,
                })}
                style={{ paddingLeft: padding }}
                onClick={() => toggleFolder(node.id)}
                onDragOver={(event) => handleFolderDragOver(event, node.id)}
                onDragEnter={(event) => handleFolderDragEnter(event, node.id)}
                onDragLeave={(event) => handleFolderDragLeave(event, node.id)}
                onDrop={(event) => handleFolderDrop(event, node.id)}
              >
                <ListItemIcon>
                  {isOpen ? (
                    <ExpandMoreIcon fontSize="small" />
                  ) : (
                    <ChevronRightIcon fontSize="small" />
                  )}
                </ListItemIcon>
                <ListItemIcon>
                  <FolderIcon fontSize="small" />
                </ListItemIcon>
                <ListItemText primary={node.name} />
              </MenuItem>
              <Collapse in={isOpen} timeout="auto" unmountOnExit>
                <div
                  className={classes.folderChildren}
                  style={{ paddingLeft: childPadding }}
                >
                  {renderRetailPlayerNodes(node.children || [], depth + 1)}
                </div>
              </Collapse>
            </React.Fragment>
          )
        }
        return renderDeviceLink(node, depth)
      }),
    [
      classes.folderChildren,
      classes.folderItem,
      classes.folderDropTarget,
      classes.folderDropTargetActive,
      dense,
      dropTargetFolderId,
      draggedDeviceId,
      canDropDeviceOnFolder,
      handleFolderDragEnter,
      handleFolderDragLeave,
      handleFolderDragOver,
      handleFolderDrop,
      openFolders,
      renderDeviceLink,
      theme,
      toggleFolder,
    ],
  )

  const renderRetailPlayerDevices = () => {
    if (retailDevicesLoading) {
      return (
        <MenuItem dense={dense} disabled className={classes.retailPlayerSubItem}>
          {translate('menu.retailPlayer.loading', { _: 'Loading devices…' })}
        </MenuItem>
      )
    }

    if (retailDevicesError) {
      return (
        <MenuItem dense={dense} disabled className={classes.retailPlayerSubItem}>
          {translate('menu.retailPlayer.error', {
            _: 'Unable to load devices',
          })}
        </MenuItem>
      )
    }

    if (!retailTree.length) {
      return (
        <MenuItem dense={dense} disabled className={classes.retailPlayerSubItem}>
          {translate('menu.retailPlayer.empty', { _: 'No devices available' })}
        </MenuItem>
      )
    }

    return renderRetailPlayerNodes(retailTree)
  }

  const renderRetailPlayerMenu = () => (
    <SubMenu
      handleToggle={() => handleToggle('menuRetailPlayer')}
      isOpen={state.menuRetailPlayer}
      sidebarIsOpen={open}
      name="menu.retailPlayer.name"
      icon={<SpeakerGroupIcon />}
      dense={dense}
      actionIcon={<BiCog />}
      onAction={goToRetailPlayerSettings}
    >
      {renderRetailPlayerDevices()}
    </SubMenu>
  )

  return (
    <div
      className={clsx(classes.root, {
        [classes.open]: open,
        [classes.closed]: !open,
      })}
    >
      {open && <LibrarySelector />}
      <SubMenu
        handleToggle={() => handleToggle('menuAlbumList')}
        isOpen={state.menuAlbumList}
        sidebarIsOpen={open}
        name="menu.albumList"
        icon={<AlbumIcon />}
        dense={dense}
      >
        {Object.keys(albumLists).map((type) =>
          renderAlbumMenuItemLink(type, albumLists[type]),
        )}
      </SubMenu>
      {resources.filter(subItems(undefined)).map(renderResourceMenuItemLink)}
      {config.devSidebarPlaylists && open ? (
        <>
          {renderRetailPlayerMenu()}
          <Divider />
          <DiscoverySubMenu
            state={state}
            setState={setState}
            sidebarIsOpen={open}
            dense={dense}
          />
          <Divider />
          <PlaylistsSubMenu
            state={state}
            setState={setState}
            sidebarIsOpen={open}
            dense={dense}
          />
        </>
      ) : (
        <>
          {renderRetailPlayerMenu()}
          {resources.filter(subItems('discovery')).map(renderResourceMenuItemLink)}
          {resources.filter(subItems('playlist')).map(renderResourceMenuItemLink)}
        </>
      )}
    </div>
  )
}

export default Menu
