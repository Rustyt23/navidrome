import React, { useCallback, useState } from 'react'
import { useSelector } from 'react-redux'
import {
  Collapse,
  Divider,
  ListItemIcon,
  ListItemText,
  Typography,
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
import SubMenu from './SubMenu'
import { humanize, pluralize } from 'inflection'
import albumLists from '../album/albumLists'
import PlaylistsSubMenu from './PlaylistsSubMenu'
import DiscoverySubMenu from './DiscoverySubMenu'
import LibrarySelector from '../common/LibrarySelector'
import config from '../config'
import { useRetailPlayerDeviceStore } from '../retailPlayer/RetailPlayerDeviceStoreContext'
import useAssignRetailPlayerDeviceToFolder from '../retailPlayer/useAssignRetailPlayerDeviceToFolder'
import {
  useRetailPlayerDeviceDrag,
  useRetailPlayerFolderDrop,
} from '../retailPlayer/useRetailPlayerDnD'
import buildRetailPlayerDnDStyles from '../retailPlayer/retailPlayerDnDStyles'

const RetailPlayerMenuDeviceLink = ({
  node,
  paddingLeft,
  classes,
  open,
  dense,
}) => {
  const slug = node.slug || node.name || node.id
  const encodedSlug = encodeURIComponent(slug)
  const { dragRef, isDragging } = useRetailPlayerDeviceDrag({
    deviceId: node.id,
    deviceName: node.name,
    origin: 'sidebar-menu',
  })

  return (
    <div
      ref={dragRef}
      className={clsx(classes.dndWrapper, isDragging && classes.dragging)}
    >
      <MenuItemLink
        to={`/retailplayer/${encodedSlug}`}
        activeClassName={classes.active}
        primaryText={
          <Typography variant="body2" noWrap title={node.name}>
            {node.name}
          </Typography>
        }
        leftIcon={
          <SpeakerGroupIcon fontSize="small" className={classes.deviceIcon} />
        }
        sidebarIsOpen={open}
        dense={dense}
        exact
        className={classes.deviceItem}
        style={{ paddingLeft }}
      />
    </div>
  )
}

const RetailPlayerMenuFolderItem = ({
  node,
  depth,
  paddingLeft,
  childPadding,
  dense,
  classes,
  isOpen,
  onToggle,
  renderChildren,
  onDeviceDrop,
}) => {
  const { dropRef, isOver, canDrop } = useRetailPlayerFolderDrop({
    folderId: node.id,
    onDrop: (deviceId, folderId) => onDeviceDrop(deviceId, folderId),
  })

  return (
    <>
      <MenuItem
        ref={dropRef}
        dense={dense}
        className={clsx(
          classes.folderItem,
          classes.dropTarget,
          canDrop && classes.dropTargetCanDrop,
          canDrop && isOver && classes.dropTargetActive,
        )}
        style={{ paddingLeft }}
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
        <ListItemText
          primary={
            <Typography variant="body1" noWrap title={node.name}>
              {node.name}
            </Typography>
          }
        />
      </MenuItem>
      <Collapse in={isOpen} timeout="auto" unmountOnExit>
        <div
          className={classes.folderChildren}
          style={{ paddingLeft: childPadding }}
        >
          {renderChildren(node.children || [], depth + 1)}
        </div>
      </Collapse>
    </>
  )
}

const useStyles = makeStyles((theme) => {
  const dndStyles = buildRetailPlayerDnDStyles(theme)
  return {
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
    paddingTop: theme.spacing(0),
    paddingBottom: theme.spacing(0),
    minHeight: 0,
    fontSize: theme.typography.pxToRem(14),
    '& .MuiListItemIcon-root': {
      minWidth: theme.spacing(4),
    },

  '& .MuiListItemIcon-root svg': {
    fontSize: theme.typography.pxToRem(16), 
   },

    '& .MuiTypography-body1': {
      lineHeight: 1.2,
      fontSize: theme.typography.pxToRem(14),
      color: theme.palette.common.white,
    },
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
    dropTarget: dndStyles.dropTarget,
    dropTargetCanDrop: dndStyles.dropTargetCanDrop,
    dropTargetActive: dndStyles.dropTargetActive,
    dragging: dndStyles.dragItem,
    dndWrapper: {
      width: '100%',
      borderRadius: theme.shape.borderRadius,
    },
  }
})

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
    menuRetailPlayer: false,
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
  } = useRetailPlayerDeviceStore()

  const [openFolders, setOpenFolders] = useState({})
  const assignDeviceToFolder = useAssignRetailPlayerDeviceToFolder()

  const handleDeviceDrop = useCallback(
    async (deviceId, folderId) => {
      if (!deviceId || !folderId) {
        return
      }
      try {
        const changed = await assignDeviceToFolder(deviceId, folderId)
        if (changed) {
          setOpenFolders((prev) => ({ ...prev, [folderId]: true }))
        }
      } catch (err) {
        console.error('Failed to assign retail player device to folder', err)
      }
    },
    [assignDeviceToFolder],
  )

  const toggleFolder = useCallback((folderId) => {
    setOpenFolders((prev) => ({
      ...prev,
      [folderId]: !prev[folderId],
    }))
  }, [])

  const goToRetailPlayerSettings = useCallback(() => {
    history.push('/retail-player/devices')
  }, [history])

  const renderRetailPlayerNodes = useCallback(
    (nodes, depth = 0) =>
      nodes.map((node) => {
        if (node.type === 'folder') {
          const isOpen = openFolders[node.id] ?? false
          const padding = theme.spacing(4 + depth * 2)
          const childPadding = theme.spacing(2)
          return (
            <RetailPlayerMenuFolderItem
              key={`retailfolder-${node.id}`}
              node={node}
              depth={depth}
              paddingLeft={padding}
              childPadding={childPadding}
              dense={dense}
              classes={classes}
              isOpen={isOpen}
              onToggle={toggleFolder}
              renderChildren={renderRetailPlayerNodes}
              onDeviceDrop={handleDeviceDrop}
            />
          )
        }
        const padding = theme.spacing(4 + depth * 2)
        return (
          <RetailPlayerMenuDeviceLink
            key={`retailplayer-${node.apiId || node.id}`}
            node={node}
            paddingLeft={padding}
            classes={classes}
            open={open}
            dense={dense}
          />
        )
      }),
    [
      classes,
      dense,
      handleDeviceDrop,
      openFolders,
      open,
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
