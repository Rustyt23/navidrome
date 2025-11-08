import React, { useCallback, useState } from 'react'
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
import { BiCog } from 'react-icons/bi'
import { useDrag, useDrop } from 'react-dnd'
import { fade } from '@material-ui/core/styles/colorManipulator'
import SubMenu from './SubMenu'
import { humanize, pluralize } from 'inflection'
import albumLists from '../album/albumLists'
import PlaylistsSubMenu from './PlaylistsSubMenu'
import DiscoverySubMenu from './DiscoverySubMenu'
import LibrarySelector from '../common/LibrarySelector'
import config from '../config'
import { useRetailPlayerDeviceStore } from '../retailPlayer/RetailPlayerDeviceStoreContext'
import { RetailPlayerDndItemTypes } from '../retailPlayer/dndTypes'

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
  folderChildren: {
    '& > *': {
      width: '100%',
    },
  },
  dropTarget: {
    transition: 'background-color 120ms ease, box-shadow 120ms ease',
  },
  dropAllowed: {
    backgroundColor: fade(theme.palette.primary.main, 0.08),
  },
  dropActive: {
    backgroundColor: fade(theme.palette.primary.main, 0.16),
    boxShadow: `inset 0 0 0 2px ${fade(theme.palette.primary.main, 0.32)}`,
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
  deviceDragWrapper: {
    width: '100%',
  },
  deviceIcon: {
    color: theme.palette.common.white,
    fontSize: theme.typography.pxToRem(16),
  },
  dragging: {
    opacity: 0.55,
  },
}))

const RetailPlayerDeviceMenuItem = ({
  node,
  depth,
  classes,
  dense,
  sidebarIsOpen,
  activeClassName,
  theme,
}) => {
  const slug = node.slug || node.name || node.id
  const encodedSlug = encodeURIComponent(slug)
  const padding = theme.spacing(4 + depth * 2)

  const [{ isDragging }, dragRef] = useDrag(
    () => ({
      type: RetailPlayerDndItemTypes.DEVICE,
      canDrag: () => Boolean(node?.id),
      item: { deviceId: node?.id },
      collect: (monitor) => ({
        isDragging: monitor.isDragging(),
      }),
    }),
    [node?.id],
  )

  return (
    <div
      ref={dragRef}
      className={clsx(
        classes.deviceDragWrapper,
        isDragging && classes.dragging,
      )}
    >
      <MenuItemLink
        to={`/retailplayer/${encodedSlug}`}
        activeClassName={activeClassName}
        primaryText={node.name}
        leftIcon={<SpeakerGroupIcon fontSize="small" className={classes.deviceIcon} />}
        sidebarIsOpen={sidebarIsOpen}
        dense={dense}
        exact
        className={classes.deviceItem}
        style={{ paddingLeft: padding }}
      />
    </div>
  )
}

const RetailPlayerFolderMenuItem = ({
  node,
  depth,
  classes,
  dense,
  isOpen,
  onToggle,
  renderChildren,
  onDropDevice,
  theme,
}) => {
  const padding = theme.spacing(4 + depth * 2)
  const childPadding = theme.spacing(2)

  const [{ isOver, canDrop }, dropRef] = useDrop(
    () => ({
      accept: RetailPlayerDndItemTypes.DEVICE,
      canDrop: (item) => Boolean(item?.deviceId) && Boolean(node?.id),
      drop: (item, monitor) => {
        if (!monitor.didDrop() && item?.deviceId && node?.id) {
          onDropDevice(item.deviceId, node.id)
        }
      },
      collect: (monitor) => ({
        isOver: monitor.isOver({ shallow: true }),
        canDrop: monitor.canDrop(),
      }),
    }),
    [node?.id, onDropDevice],
  )

  return (
    <React.Fragment>
      <MenuItem
        ref={dropRef}
        dense={dense}
        className={clsx(
          classes.folderItem,
          classes.dropTarget,
          canDrop && classes.dropAllowed,
          isOver && classes.dropActive,
        )}
        style={{ paddingLeft: padding }}
        onClick={() => onToggle(node.id)}
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
          {renderChildren(node.children || [], depth + 1)}
        </div>
      </Collapse>
    </React.Fragment>
  )
}

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
    },
    actions: { assignDeviceToFolder },
  } = useRetailPlayerDeviceStore()

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

  const handleDeviceDropOnFolder = useCallback(
    (deviceId, folderId) => {
      if (!deviceId || !folderId) {
        return
      }
      assignDeviceToFolder({ id: deviceId, folderIds: [folderId] })
    },
    [assignDeviceToFolder],
  )

  const renderRetailPlayerNodes = useCallback(
    (nodes, depth = 0) =>
      nodes.map((node) => {
        if (node.type === 'folder') {
          const isOpen = openFolders[node.id] ?? false
          return (
            <RetailPlayerFolderMenuItem
              key={`retailfolder-${node.id}`}
              node={node}
              depth={depth}
              classes={classes}
              dense={dense}
              isOpen={isOpen}
              onToggle={toggleFolder}
              renderChildren={renderRetailPlayerNodes}
              onDropDevice={handleDeviceDropOnFolder}
              theme={theme}
            />
          )
        }
        return (
          <RetailPlayerDeviceMenuItem
            key={`retailplayer-${node.apiId || node.id}`}
            node={node}
            depth={depth}
            classes={classes}
            dense={dense}
            sidebarIsOpen={open}
            activeClassName={classes.active}
            theme={theme}
          />
        )
      }),
    [
      classes,
      dense,
      handleDeviceDropOnFolder,
      open,
      openFolders,
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
