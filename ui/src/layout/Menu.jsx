import React, { useState } from 'react'
import { useSelector } from 'react-redux'
import { Divider, makeStyles } from '@material-ui/core'
import clsx from 'clsx'
import { useTranslate, MenuItemLink, getResources } from 'react-admin'
import ViewListIcon from '@material-ui/icons/ViewList'
import AlbumIcon from '@material-ui/icons/Album'
import MenuItem from '@material-ui/core/MenuItem'
import SpeakerGroupIcon from '@material-ui/icons/SpeakerGroup'
import SubMenu from './SubMenu'
import { humanize, pluralize } from 'inflection'
import albumLists from '../album/albumLists'
import PlaylistsSubMenu from './PlaylistsSubMenu'
import DiscoverySubMenu from './DiscoverySubMenu'
import LibrarySelector from '../common/LibrarySelector'
import config from '../config'
import useRetailPlayerDevices from '../retailPlayer/useRetailPlayerDevices'

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
  const resources = useSelector(getResources).filter(
    (r) => r.name !== 'radio' && r.name !== 'share',
  )

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
    devices: retailDevices,
    error: retailDevicesError,
    isLoading: retailDevicesLoading,
  } = useRetailPlayerDevices()

  const renderRetailPlayerDevices = () => {
    if (retailDevicesLoading) {
      return (
        <MenuItem dense={dense} disabled>
          {translate('menu.retailPlayer.loading', { _: 'Loading devices…' })}
        </MenuItem>
      )
    }

    if (retailDevicesError) {
      return (
        <MenuItem dense={dense} disabled>
          {translate('menu.retailPlayer.error', {
            _: 'Unable to load devices',
          })}
        </MenuItem>
      )
    }

    if (!retailDevices.length) {
      return (
        <MenuItem dense={dense} disabled>
          {translate('menu.retailPlayer.empty', { _: 'No devices available' })}
        </MenuItem>
      )
    }

    return retailDevices.map((device) => {
      const slug = device.slug || device.name || device.id
      const encodedSlug = encodeURIComponent(slug)
      return (
        <MenuItemLink
          key={`retailplayer-${device.apiId || device.id}`}
          to={`/retailplayer/${encodedSlug}`}
          activeClassName={classes.active}
          primaryText={device.name}
          sidebarIsOpen={open}
          dense={dense}
          exact
        />
      )
    })
  }

  const renderRetailPlayerMenu = () => (
    <SubMenu
      handleToggle={() => handleToggle('menuRetailPlayer')}
      isOpen={state.menuRetailPlayer}
      sidebarIsOpen={open}
      name="menu.retailPlayer.name"
      icon={<SpeakerGroupIcon />}
      dense={dense}
    >
      <MenuItemLink
        key="retailplayer"
        to="/retailplayer/devices"
        activeClassName={classes.active}
        primaryText={translate('menu.retailPlayer.allDevices', {
          _: 'All Devices',
        })}
        sidebarIsOpen={open}
        dense={dense}
        exact
      />
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
