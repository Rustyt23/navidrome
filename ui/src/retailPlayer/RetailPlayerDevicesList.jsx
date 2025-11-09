import React, { useMemo, useState } from 'react'
import { makeStyles } from '@material-ui/core/styles'
import { Typography, ButtonBase, TextField } from '@material-ui/core'
import { Title, useTranslate } from 'react-admin'
import ChevronRightIcon from '@material-ui/icons/ChevronRight'
import ArrowDownwardIcon from '@material-ui/icons/ArrowDownward'
import ArrowUpwardIcon from '@material-ui/icons/ArrowUpward'
import UnfoldMoreIcon from '@material-ui/icons/UnfoldMore'
import { useHistory } from 'react-router-dom'
import { useSmartSort, SortDirection, SortType } from '../utils'
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
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
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
  headerButton: {
    background: 'none',
    border: 'none',
    padding: 0,
    margin: 0,
    font: 'inherit',
    color: 'inherit',
    display: 'inline-flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
    cursor: 'pointer',
    textTransform: 'inherit',
  },
  headerButtonInactive: {
    opacity: 0.7,
  },
  sortIcon: {
    fontSize: theme.typography.pxToRem(14),
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

  const handleNavigate = (device) => {
    if (!device) {
      return
    }
    const slug = device.slug || device.name || device.id
    const encodedSlug = encodeURIComponent(slug)
    history.push(`/retailplayer/${encodedSlug}`)
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

  const columnConfig = useMemo(
    () => ({
      name: { type: SortType.STRING },
      channel: { type: SortType.STRING },
      channelList: { type: SortType.STRING },
      organization: { type: SortType.STRING },
    }),
    [],
  )

  const { sortedData: sortedDevices, requestSort, getSortDirection } = useSmartSort(
    filteredDevices,
    {
      initialKey: 'name',
      initialDirection: SortDirection.ASC,
      columns: columnConfig,
    },
  )

  const getAriaSort = (key) => {
    const direction = getSortDirection(key)
    if (!direction) {
      return 'none'
    }
    return direction === SortDirection.ASC ? 'ascending' : 'descending'
  }

  const getSortLabel = (key, label) => {
    const direction = getSortDirection(key)
    if (!direction) {
      return `Sort by ${label}`
    }
    const directionLabel =
      direction === SortDirection.ASC ? 'ascending' : 'descending'
    return `Sort by ${label}, currently ${directionLabel}`
  }

  const headerButtonClass = (key) => {
    const direction = getSortDirection(key)
    return direction
      ? classes.headerButton
      : `${classes.headerButton} ${classes.headerButtonInactive}`
  }

  const renderSortIcon = (key) => {
    const direction = getSortDirection(key)
    if (!direction) {
      return <UnfoldMoreIcon className={classes.sortIcon} aria-hidden="true" />
    }
    return direction === SortDirection.ASC ? (
      <ArrowUpwardIcon className={classes.sortIcon} aria-hidden="true" />
    ) : (
      <ArrowDownwardIcon className={classes.sortIcon} aria-hidden="true" />
    )
  }

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
          <span
            className={classes.headerCell}
            data-area="name"
            role="columnheader"
            aria-sort={getAriaSort('name')}
          >
            <button
              type="button"
              className={headerButtonClass('name')}
              onClick={() => requestSort('name')}
              aria-label={getSortLabel('name', 'Name')}
            >
              <span>Name</span>
              {renderSortIcon('name')}
            </button>
          </span>
          <span
            className={classes.headerCell}
            data-area="channel"
            role="columnheader"
            aria-sort={getAriaSort('channel')}
          >
            <button
              type="button"
              className={headerButtonClass('channel')}
              onClick={() => requestSort('channel')}
              aria-label={getSortLabel('channel', 'Channel')}
            >
              <span>Channel</span>
              {renderSortIcon('channel')}
            </button>
          </span>
          <span
            className={classes.headerCell}
            data-area="channelList"
            role="columnheader"
            aria-sort={getAriaSort('channelList')}
          >
            <button
              type="button"
              className={headerButtonClass('channelList')}
              onClick={() => requestSort('channelList')}
              aria-label={getSortLabel('channelList', 'Channel List')}
            >
              <span>Channel List</span>
              {renderSortIcon('channelList')}
            </button>
          </span>
          <span
            className={classes.headerCell}
            data-area="organization"
            role="columnheader"
            aria-sort={getAriaSort('organization')}
          >
            <button
              type="button"
              className={headerButtonClass('organization')}
              onClick={() => requestSort('organization')}
              aria-label={getSortLabel('organization', 'Organization')}
            >
              <span>Organization</span>
              {renderSortIcon('organization')}
            </button>
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
        ) : sortedDevices.length > 0 ? (
          sortedDevices.map((device) => (
          <ButtonBase
            key={device.apiId || device.id}
              className={classes.buttonBase}
              onClick={() => handleNavigate(device)}
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
