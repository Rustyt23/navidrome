import React, { useCallback, useEffect, useState } from 'react'
import {
  ListItem,
  ListItemIcon,
  ListItemText,
  IconButton,
  CircularProgress,
  makeStyles,
} from '@material-ui/core'
import ListItemSecondaryAction from '@material-ui/core/ListItemSecondaryAction'
import QueueMusicIcon from '@material-ui/icons/QueueMusic'
import RefreshIcon from '@material-ui/icons/Refresh'
import CloudDownloadIcon from '@material-ui/icons/CloudDownload'
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

  const handleExport = (item) => {
    window.open(`${REST_URL}/discovery/${item.id}/export`, '_blank')
  }

  const songCountLabel = (item) => {
    if (typeof item.songCount !== 'number') return ''
    return translate('resources.playlist.fields.songCount', {
      smart_count: item.songCount,
      _: translate('ra.page.items', { smart_count: item.songCount }),
    })
  }

  const handleRefresh = () => {
    if (!loading) {
      fetchDiscovery(true)
    }
  }

  return (
    <SubMenu
      handleToggle={handleToggle}
      isOpen={state.menuDiscovery}
      sidebarIsOpen={sidebarIsOpen}
      name="menu.discovery"
      icon={<QueueMusicIcon />}
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
          <ListItemText primary={item.name} secondary={songCountLabel(item)} />
          <ListItemSecondaryAction>
            <IconButton edge="end" size="small" onClick={() => handleExport(item)} aria-label={translate('action.export')}>
              <CloudDownloadIcon fontSize="small" />
            </IconButton>
          </ListItemSecondaryAction>
        </ListItem>
      ))}
    </SubMenu>
  )
}

export default DiscoverySubMenu
