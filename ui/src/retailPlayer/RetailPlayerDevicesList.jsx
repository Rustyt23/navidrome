import React, { useMemo, useState } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import {
  Card,
  CardContent,
  CircularProgress,
  List,
  ListItem,
  ListItemText,
  TextField,
  Typography,
} from '@material-ui/core'
import { Title, useTranslate } from 'react-admin'
import { useHistory } from 'react-router-dom'
import useRetailPlayerDevices from './useRetailPlayerDevices'

const useStyles = makeStyles((theme) => ({
  root: {
    maxWidth: 960,
    margin: '0 auto',
    padding: theme.spacing(4),
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
  },
  searchRow: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: theme.spacing(2),
    alignItems: 'center',
  },
  searchField: {
    maxWidth: 360,
  },
  listCard: {
    width: '100%',
  },
  loading: {
    display: 'flex',
    justifyContent: 'center',
    padding: theme.spacing(2),
  },
  error: {
    color: theme.palette.error.main,
  },
  empty: {
    color: theme.palette.text.secondary,
  },
}))

const RetailPlayerDevicesList = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const history = useHistory()
  const [searchTerm, setSearchTerm] = useState('')
  const { devices, error, isLoading } = useRetailPlayerDevices()

  const filteredDevices = useMemo(() => {
    const term = searchTerm.trim().toLowerCase()
    if (!term) {
      return devices
    }

    return devices.filter((device) =>
      (device.name || '').toLowerCase().includes(term),
    )
  }, [devices, searchTerm])

  const handleNavigate = (device) => {
    if (!device) {
      return
    }
    const slug = device.slug || device.name || device.id
    history.push(`/retailplayer/${encodeURIComponent(slug)}`)
  }

  return (
    <div className={classes.root}>
      <Title title="Retail Player Devices" />
      <Typography component="h1" variant="h4">
        {translate('menu.retailPlayer.devices', {
          _: 'Retail Player Devices',
        })}
      </Typography>

      <div className={classes.searchRow}>
        <TextField
          className={classes.searchField}
          variant="outlined"
          size="small"
          label={translate('menu.retailPlayer.search', {
            _: 'Search devices',
          })}
          placeholder={translate('menu.retailPlayer.searchPlaceholder', {
            _: 'Type to filter devices…',
          })}
          value={searchTerm}
          onChange={(event) => setSearchTerm(event.target.value)}
        />
      </div>

      <Card className={classes.listCard}>
        <CardContent>
          {isLoading ? (
            <div className={classes.loading}>
              <CircularProgress size={32} />
            </div>
          ) : error ? (
            <Typography className={classes.error}>
              {error.message || 'Unable to load retail player devices.'}
            </Typography>
          ) : filteredDevices.length ? (
            <List>
              {filteredDevices.map((device) => {
                const secondaryText = [
                  device.channel,
                  device.organization,
                ]
                  .map((value) => (value ? value : null))
                  .filter(Boolean)
                  .join(' • ')

                return (
                  <ListItem
                    key={device.apiId || device.id}
                    button
                    onClick={() => handleNavigate(device)}
                  >
                    <ListItemText
                      primary={device.name}
                      secondary={secondaryText || undefined}
                    />
                  </ListItem>
                )
              })}
            </List>
          ) : (
            <Typography className={classes.empty}>
              {translate('menu.retailPlayer.empty', {
                _: 'No devices available.',
              })}
            </Typography>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

export default RetailPlayerDevicesList
