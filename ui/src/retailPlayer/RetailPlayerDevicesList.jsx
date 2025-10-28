import React, { useMemo, useState } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import { Typography, ButtonBase, TextField } from '@material-ui/core'
import { Title, useTranslate } from 'react-admin'
import ChevronRightIcon from '@material-ui/icons/ChevronRight'
import { useHistory } from 'react-router-dom'
import useRetailPlayerDevices from './useRetailPlayerDevices'

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
  },
  searchField: {
    maxWidth: 360,
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
}))

const RetailPlayerDevicesList = () => {
  const classes = useStyles()
  const history = useHistory()
  const translate = useTranslate()
  const [searchTerm, setSearchTerm] = useState('')
  const {
    devices,
    error: devicesError,
    isLoading: devicesLoading,
  } = useRetailPlayerDevices()

  const handleNavigate = (deviceId) => {
    history.push(`/retailplayer/${deviceId}`)
  }

  const filteredDevices = useMemo(() => {
    const normalizedTerm = searchTerm.trim().toLowerCase()
    if (!normalizedTerm) {
      return devices
    }

    return devices.filter((device) =>
      device.name.toLowerCase().includes(normalizedTerm),
    )
  }, [devices, searchTerm])

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
        {devicesLoading ? (
          <div className={classes.noResults}>
            {translate('menu.retailPlayer.loading', { _: 'Loading devices…' })}
          </div>
        ) : devicesError ? (
          <div className={classes.noResults}>
            {translate('menu.retailPlayer.error', { _: 'Unable to load devices' })}
          </div>
        ) : filteredDevices.length > 0 ? (
          filteredDevices.map((device) => (
            <ButtonBase
              key={device.id}
              className={classes.buttonBase}
              onClick={() => handleNavigate(device.id)}
              focusRipple
              aria-label={`Open ${device.name}`}
            >
              <span className={classes.rowButton}>
                <span className={`${classes.cell} ${classes.actionCell}`} data-area="actions">
                  View
                  <ChevronRightIcon className={classes.chevron} aria-hidden="true" />
                </span>
                <span className={classes.cell} data-area="name">
                  {device.name}
                </span>
                <span className={classes.cell} data-area="channel">
                  {device.channel}
                </span>
                <span className={classes.cell} data-area="channelList">
                  {device.channelList}
                </span>
                <span className={classes.cell} data-area="organization">
                  {device.organization}
                </span>
              </span>
            </ButtonBase>
          ))
        ) : (
          <div className={classes.noResults}>No devices match this search.</div>
        )}
      </div>
    </div>
  )
}

export default RetailPlayerDevicesList
