import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import {
  Typography,
  Button,
  ButtonBase,
  CircularProgress,
  IconButton,
  TextField,
} from '@material-ui/core'
import { Title } from 'react-admin'
import ChevronRightIcon from '@material-ui/icons/ChevronRight'
import { useHistory } from 'react-router-dom'
import RefreshIcon from '@material-ui/icons/Refresh'
import { getDevices } from '../services/retailPlayerService'

const useStyles = makeStyles((theme) => ({
  root: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(5),
    padding: theme.spacing(5),
    maxWidth: 1200,
    width: '100%',
    margin: '0 auto',
    [theme.breakpoints.down('md')]: {
      padding: theme.spacing(4),
      gap: theme.spacing(4),
    },
    [theme.breakpoints.down('sm')]: {
      padding: theme.spacing(2.5),
      gap: theme.spacing(3),
    },
  },
  heading: {
    fontWeight: theme.typography.fontWeightBold,
    fontSize: theme.typography.pxToRem(48),
    [theme.breakpoints.down('md')]: {
      fontSize: theme.typography.pxToRem(36),
    },
    [theme.breakpoints.down('sm')]: {
      fontSize: theme.typography.pxToRem(26),
    },
  },
  searchRow: {
    display: 'flex',
    justifyContent: 'flex-start',
    alignItems: 'center',
    gap: theme.spacing(2),
  },
  searchField: {
    maxWidth: 360,
  },
  refreshButton: {
    padding: theme.spacing(1),
  },
  lastUpdated: {
    color: theme.palette.text.secondary,
    fontSize: theme.typography.pxToRem(13),
  },
  table: {
    borderRadius: theme.shape.borderRadius,
    border: `1px solid ${theme.palette.divider}`,
    overflow: 'hidden',
    backgroundColor: theme.palette.background.paper,
  },
  headerRow: {
    display: 'grid',
    gridTemplateColumns: '160px 1.5fr 1fr 1fr 1fr',
    padding: `${theme.spacing(2)}px ${theme.spacing(3)}px`,
    backgroundColor: theme.palette.action.hover,
    gap: theme.spacing(2),
    [theme.breakpoints.down('sm')]: {
      gridTemplateColumns: '120px 1.2fr 1fr',
      gridTemplateAreas: "'actions name name' 'channel channelList org'",
      rowGap: theme.spacing(1),
    },
  },
  headerCell: {
    fontSize: theme.typography.pxToRem(14),
    fontWeight: theme.typography.fontWeightMedium,
    textTransform: 'uppercase',
    letterSpacing: 1,
    color: theme.palette.text.secondary,
    [theme.breakpoints.down('sm')]: {
      '&[data-area="actions"]': {
        gridArea: 'actions',
      },
      '&[data-area="name"]': {
        gridArea: 'name',
      },
      '&[data-area="channel"]': {
        gridArea: 'channel',
      },
      '&[data-area="channelList"]': {
        gridArea: 'channelList',
      },
      '&[data-area="organization"]': {
        gridArea: 'org',
      },
    },
  },
  buttonBase: {
    display: 'block',
    width: '100%',
    justifyContent: 'flex-start',
    textAlign: 'left',
    borderBottom: `1px solid ${theme.palette.divider}`,
    '&:last-child': {
      borderBottom: 'none',
    },
    '&:hover $rowButton, &:focus-visible $rowButton': {
      backgroundColor: theme.palette.action.hover,
    },
  },
  rowButton: {
    display: 'grid',
    gridTemplateColumns: '160px 1.5fr 1fr 1fr 1fr',
    padding: `${theme.spacing(2)}px ${theme.spacing(3)}px`,
    textAlign: 'left',
    gap: theme.spacing(2),
    width: '100%',
    alignItems: 'center',
    transition: theme.transitions.create(['background-color'], {
      duration: theme.transitions.duration.shortest,
    }),
    [theme.breakpoints.down('sm')]: {
      gridTemplateColumns: '120px 1.2fr 1fr',
      gridTemplateAreas: "'actions name name' 'channel channelList org'",
      rowGap: theme.spacing(1.5),
    },
  },
  cell: {
    fontSize: theme.typography.pxToRem(16),
    color: theme.palette.text.primary,
    [theme.breakpoints.down('sm')]: {
      '&[data-area="actions"]': {
        gridArea: 'actions',
      },
      '&[data-area="name"]': {
        gridArea: 'name',
      },
      '&[data-area="channel"]': {
        gridArea: 'channel',
      },
      '&[data-area="channelList"]': {
        gridArea: 'channelList',
      },
      '&[data-area="organization"]': {
        gridArea: 'org',
      },
    },
  },
  actionCell: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
    color:
      (theme.palette.primary && theme.palette.primary.main) ||
      theme.palette.text.primary,
    fontWeight: theme.typography.fontWeightMedium,
  },
  chevron: {
    fontSize: theme.typography.pxToRem(20),
  },
  noResults: {
    padding: `${theme.spacing(3)}px ${theme.spacing(3)}px ${theme.spacing(4)}px`,
    color: theme.palette.text.secondary,
    fontStyle: 'italic',
  },
  statusRow: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
    padding: `${theme.spacing(2)}px ${theme.spacing(3)}px`,
    color: theme.palette.text.secondary,
  },
  statusActions: {
    display: 'flex',
    gap: theme.spacing(1),
  },
}))

const RetailPlayerDevicesList = () => {
  const classes = useStyles()
  const history = useHistory()
  const [searchTerm, setSearchTerm] = useState('')
  const [devices, setDevices] = useState([])
  const [isLoading, setIsLoading] = useState(true)
  const [hasError, setHasError] = useState(false)
  const [lastUpdated, setLastUpdated] = useState(null)

  const normalizeDevices = useCallback((response) => {
    if (!response) {
      return []
    }

    if (Array.isArray(response)) {
      return response
    }

    if (Array.isArray(response.devices)) {
      return response.devices
    }

    if (Array.isArray(response.items)) {
      return response.items
    }

    if (Array.isArray(response.data)) {
      return response.data
    }

    return []
  }, [])

  const loadDevices = useCallback(async () => {
    setIsLoading(true)
    setHasError(false)

    try {
      const response = await getDevices()
      const fetchedDevices = normalizeDevices(response)
      setDevices(fetchedDevices)
      setLastUpdated(new Date())
    } catch (error) {
      setHasError(true)
      setDevices([])
    } finally {
      setIsLoading(false)
    }
  }, [normalizeDevices])

  useEffect(() => {
    loadDevices()
  }, [loadDevices])

  const handleNavigate = (deviceId) => {
    history.push(`/retailplayer/${deviceId}`)
  }

  const getDeviceDisplayName = useCallback((device) => {
    return (
      (device?.name && device.name.trim()) ||
      (device?.displayName && device.displayName.trim()) ||
      (device?.deviceName && device.deviceName.trim()) ||
      (device?.id && String(device.id)) ||
      '—'
    )
  }, [])

  const getDeviceId = (device) => {
    return (
      device?.id ||
      device?.deviceId ||
      device?.device_id ||
      device?.uuid ||
      null
    )
  }

  const getDeviceChannel = (device) => {
    return (
      device?.channel?.name ||
      device?.currentChannel?.name ||
      device?.assignedChannel?.name ||
      device?.channelName ||
      device?.currentChannelName ||
      '—'
    )
  }

  const getDeviceOrganization = (device) => {
    const organizationName =
      device?.organization?.name ||
      device?.orgName ||
      device?.organizationName

    if (organizationName) {
      return organizationName
    }

    const organizationId =
      device?.organization?.id ||
      device?.orgId ||
      device?.org_id ||
      device?.organizationId

    if (organizationId) {
      return String(organizationId).slice(0, 8)
    }

    return '—'
  }

  const formattedLastUpdated = useMemo(() => {
    if (!lastUpdated) {
      return null
    }

    return lastUpdated.toLocaleTimeString([], {
      hour: '2-digit',
      minute: '2-digit',
    })
  }, [lastUpdated])

  const filteredDevices = useMemo(() => {
    const normalizedTerm = searchTerm.trim().toLowerCase()
    if (!normalizedTerm) {
      return devices
    }

    return devices.filter((device) =>
      getDeviceDisplayName(device).toLowerCase().includes(normalizedTerm),
    )
  }, [devices, getDeviceDisplayName, searchTerm])

  return (
    <div className={classes.root}>
      <Title title="Retail Player Devices" />
      <Typography component="h1" className={classes.heading}>
        Retail Player Devices
      </Typography>
      <div className={classes.searchRow}>
        <TextField
          className={classes.searchField}
          variant="outlined"
          size="small"
          placeholder="Search devices…"
          value={searchTerm}
          onChange={(event) => setSearchTerm(event.target.value)}
          inputProps={{ 'aria-label': 'Search devices' }}
        />
        <IconButton
          className={classes.refreshButton}
          aria-label="Refresh devices"
          onClick={loadDevices}
          disabled={isLoading}
        >
          <RefreshIcon fontSize="small" />
        </IconButton>
        {formattedLastUpdated && (
          <Typography variant="caption" className={classes.lastUpdated}>
            Last updated {formattedLastUpdated}
          </Typography>
        )}
      </div>
      <div className={classes.table}>
        <div className={classes.headerRow}>
          <span className={classes.headerCell} data-area="actions">
            Actions
          </span>
          <span className={classes.headerCell} data-area="name">
            Name
          </span>
          <span className={classes.headerCell} data-area="channel">
            Channel
          </span>
          <span className={classes.headerCell} data-area="channelList">
            Channel List
          </span>
          <span className={classes.headerCell} data-area="organization">
            Organization
          </span>
        </div>
        {isLoading ? (
          <div className={classes.statusRow}>
            <CircularProgress size={18} thickness={4} />
            <span>Loading devices…</span>
          </div>
        ) : hasError ? (
          <div className={classes.statusRow}>
            <span>Couldn't load devices.</span>
            <span className={classes.statusActions}>
              <Button
                variant="outlined"
                size="small"
                onClick={loadDevices}
              >
                Retry
              </Button>
            </span>
          </div>
        ) : filteredDevices.length > 0 ? (
          filteredDevices.map((device) => {
            const deviceId = getDeviceId(device)
            return (
              <ButtonBase
                key={deviceId || getDeviceDisplayName(device)}
                className={classes.buttonBase}
                onClick={() => deviceId && handleNavigate(deviceId)}
                focusRipple
                aria-label={`Open ${getDeviceDisplayName(device)}`}
              >
                <span className={classes.rowButton}>
                  <span className={`${classes.cell} ${classes.actionCell}`} data-area="actions">
                    View
                    <ChevronRightIcon className={classes.chevron} aria-hidden="true" />
                  </span>
                  <span className={classes.cell} data-area="name">
                    {getDeviceDisplayName(device)}
                  </span>
                  <span className={classes.cell} data-area="channel">
                    {getDeviceChannel(device)}
                  </span>
                  <span className={classes.cell} data-area="channelList">
                    {'—'}
                  </span>
                  <span className={classes.cell} data-area="organization">
                    {getDeviceOrganization(device)}
                  </span>
                </span>
              </ButtonBase>
            )
          })
        ) : (
          <div className={classes.noResults}>No devices match this search.</div>
        )}
      </div>
    </div>
  )
}

export default RetailPlayerDevicesList
