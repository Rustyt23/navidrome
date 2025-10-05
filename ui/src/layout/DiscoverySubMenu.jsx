import React, { useCallback, useEffect, useState } from 'react'
import {
  ListItem,
  ListItemIcon,
  ListItemText,
  CircularProgress,
  makeStyles,
} from '@material-ui/core'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import RefreshIcon from '@material-ui/icons/Refresh'
import { FaCompass } from 'react-icons/fa'
import { useHistory } from 'react-router-dom'
import { useNotify, useTranslate } from 'react-admin'
import SubMenu from './SubMenu'
import httpClient from '../dataProvider/httpClient'
import { REST_URL } from '../consts'

const useStyles = makeStyles((theme) => ({
  item: {
    borderRadius: 6,
    marginRight: 4,
    paddingTop: 1,
    paddingBottom: 1,
    '&:hover': { backgroundColor: theme.palette.action.hover },
  },
  icon: { minWidth: 28 },
  placeholder: { padding: theme.spacing(1, 2) },
  spinner: { marginLeft: theme.spacing(1) },
}))

const DiscoverySubMenu = ({ state, setState, sidebarIsOpen, dense }) => {
  const classes = useStyles()
  const notify = useNotify()
  const translate = useTranslate()
  const history = useHistory()
  const [items, setItems] = useState([])
  const [loading, setLoading] = useState(false)
  const fetchDiscovery = useCallback(
    async (refresh = false) => {
      setLoading(true)
      try {
        const suffix = refresh ? '?refresh=true' : ''
        const res = await httpClient(`${REST_URL}/discovery${suffix}`)
        const list = Array.isArray(res?.json) ? res.json : []
        setItems(list)
      } catch (error) {
        notify('ra.page.error', 'warning')
      } finally {
        setLoading(false)
      }
    },
    [notify]
  )

  useEffect(() => {
    fetchDiscovery(false)
  }, [fetchDiscovery])

  const handleToggle = () => {
    setState((prev) => ({ ...prev, menuDiscovery: !prev.menuDiscovery }))
  }

  const handleNavigate = (item) => {
    history.push(`/discovery/${item.id}`)
  }

  const handleRefresh = () => {
    if (!loading) {
      history.push('/discovery')
      fetchDiscovery(true)
    }
  }

  return (
    <SubMenu
      handleToggle={handleToggle}
      isOpen={state.menuDiscovery}
      sidebarIsOpen={sidebarIsOpen}
      name="menu.discovery"
      icon={<FaCompass />}
      dense={dense}
      onAction={handleRefresh}
      actionIcon={loading ? <CircularProgress size={16} className={classes.spinner} /> : <RefreshIcon fontSize="small" />}
    >
      {items.length === 0 && !loading ? (
        <ListItem disabled className={classes.placeholder}>
          <ListItemText primary={translate('menu.discovery_empty', { _: translate('ra.message.no_results') })} />
        </ListItem>
      ) : null}
      {items.map((item) => (
        <ListItem
          button
          dense={dense}
          key={item.id}
          onClick={() => handleNavigate(item)}
          className={classes.item}
        >
          <ListItemIcon className={classes.icon}>
            <QueueMusicIcon fontSize="small" />
          </ListItemIcon>
          <ListItemText primary={item.name} />
        </ListItem>
      ))}
    </SubMenu>
  )
}

export default DiscoverySubMenu
