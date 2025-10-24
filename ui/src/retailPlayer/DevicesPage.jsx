import React, { useMemo } from 'react'
import clsx from 'clsx'
import {
  Box,
  Divider,
  IconButton,
  InputAdornment,
  Paper,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TableSortLabel,
  TextField,
  Tooltip,
  Typography,
  makeStyles,
} from '@material-ui/core'
import SearchIcon from '@material-ui/icons/Search'
import RefreshIcon from '@material-ui/icons/Refresh'
import AddIcon from '@material-ui/icons/Add'
import LinkIcon from '@material-ui/icons/Link'
import VolumeUpIcon from '@material-ui/icons/VolumeUp'
import VolumeOffIcon from '@material-ui/icons/VolumeOff'
import WifiTetheringIcon from '@material-ui/icons/WifiTethering'
import EmojiEventsIcon from '@material-ui/icons/EmojiEvents'
import ChevronLeftIcon from '@material-ui/icons/ChevronLeft'
import ChevronRightIcon from '@material-ui/icons/ChevronRight'

const useStyles = makeStyles((theme) => ({
  root: {
    display: 'flex',
    flexDirection: 'column',
    gap: theme.spacing(3),
  },
  surface: {
    backgroundColor: theme.palette.background.paper,
    borderRadius: theme.shape.borderRadius * 1.5,
    border: `1px solid ${theme.palette.divider}`,
    boxShadow: theme.shadows[1],
    overflow: 'hidden',
  },
  toolbar: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: theme.spacing(2),
    flexWrap: 'wrap',
    padding: theme.spacing(2.5, 3),
  },
  toolbarTitle: {
    flex: '1 1 auto',
    minWidth: 200,
  },
  toolbarSubtitle: {
    marginTop: theme.spacing(0.5),
    color: theme.palette.text.secondary,
  },
  toolbarActions: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1.5),
  },
  searchField: {
    minWidth: 220,
    '& .MuiOutlinedInput-root': {
      borderRadius: theme.shape.borderRadius,
    },
  },
  tableContainer: {
    maxWidth: '100%',
  },
  table: {
    minWidth: 960,
  },
  headCell: {
    color: theme.palette.text.secondary,
    fontSize: 12,
    letterSpacing: 0.5,
    textTransform: 'uppercase',
    borderBottom: `1px solid ${theme.palette.divider}`,
  },
  bodyCell: {
    borderBottom: `1px solid ${theme.palette.divider}`,
    color: theme.palette.text.primary,
    fontSize: 14,
    maxWidth: 220,
  },
  actionsCell: {
    width: 140,
    maxWidth: 140,
    whiteSpace: 'nowrap',
  },
  actionsGroup: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(0.5),
  },
  actionButton: {
    padding: theme.spacing(0.5),
  },
  nameCell: {
    fontWeight: 600,
  },
  ellipsis: {
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  },
  heroIcon: {
    color: theme.palette.primary.light,
  },
  footer: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    flexWrap: 'wrap',
    gap: theme.spacing(2),
    padding: theme.spacing(2, 3),
  },
  footerControls: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  footerButton: {
    padding: theme.spacing(0.5),
  },
}))

const mockRows = [
  {
    id: 'device-1',
    name: 'Lobby Speaker - North Entrance',
    uptime: '12d 4h',
    channel: 'Main Floor Mix',
    channelList: 'Weekday Rotation',
    organization: 'Acme Retail Group',
    retailHero: true,
    status: 'online',
  },
  {
    id: 'device-2',
    name: 'Cafe Bar Display',
    uptime: '3d 9h',
    channel: 'Acoustic Evenings',
    channelList: 'Weekend Chill',
    organization: 'Acme Retail Group',
    retailHero: false,
    status: 'online',
  },
  {
    id: 'device-3',
    name: 'Warehouse Zone C',
    uptime: '8h 22m',
    channel: 'Logistics Updates',
    channelList: 'Operations Messaging',
    organization: 'Acme Logistics',
    retailHero: true,
    status: 'muted',
  },
]

const DevicesPage = () => {
  const classes = useStyles()
  const devices = useMemo(() => mockRows, [])

  return (
    <div className={classes.root}>
      <Paper className={classes.surface} elevation={0}>
        <div className={classes.toolbar}>
          <div className={classes.toolbarTitle}>
            <Typography variant="h5" component="h1" noWrap>
              Acme Retail Group’s Devices
            </Typography>
            <Typography variant="body2" className={classes.toolbarSubtitle} noWrap>
              Monitor connectivity, playback, and assignments at a glance.
            </Typography>
          </div>
          <div className={classes.toolbarActions}>
            <TextField
              variant="outlined"
              size="small"
              placeholder="Search devices"
              aria-label="Search devices"
              className={classes.searchField}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" />
                  </InputAdornment>
                ),
              }}
            />
            <Tooltip title="Refresh">
              <IconButton aria-label="Refresh" size="small">
                <RefreshIcon />
              </IconButton>
            </Tooltip>
            <Tooltip title="Add device">
              <IconButton aria-label="Add device" size="small" color="primary">
                <AddIcon />
              </IconButton>
            </Tooltip>
          </div>
        </div>
        <Divider />
        <TableContainer className={classes.tableContainer}>
          <Table className={classes.table} size="medium" aria-label="Devices table">
            <TableHead>
              <TableRow>
                <TableCell className={clsx(classes.headCell, classes.actionsCell)} align="left">
                  Actions
                </TableCell>
                <TableCell className={classes.headCell}>
                  <TableSortLabel active direction="asc" hideSortIcon={false}>
                    Name
                  </TableSortLabel>
                </TableCell>
                <TableCell className={classes.headCell}>Uptime</TableCell>
                <TableCell className={classes.headCell}>RetailHero</TableCell>
                <TableCell className={classes.headCell}>Channel</TableCell>
                <TableCell className={classes.headCell}>Channel List</TableCell>
                <TableCell className={classes.headCell}>Organization</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {devices.map((device) => (
                <TableRow hover key={device.id}>
                  <TableCell className={clsx(classes.bodyCell, classes.actionsCell)}>
                    <div className={classes.actionsGroup}>
                      <Tooltip title="Connection status">
                        <IconButton
                          aria-label={`Connection status for ${device.name}`}
                          size="small"
                          className={classes.actionButton}
                        >
                          <WifiTetheringIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title="Link">
                        <IconButton
                          aria-label={`Link ${device.name}`}
                          size="small"
                          className={classes.actionButton}
                        >
                          <LinkIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                      <Tooltip title={device.status === 'muted' ? 'Unmute' : 'Mute'}>
                        <IconButton
                          aria-label={`${device.status === 'muted' ? 'Unmute' : 'Mute'} ${device.name}`}
                          size="small"
                          className={classes.actionButton}
                        >
                          {device.status === 'muted' ? (
                            <VolumeOffIcon fontSize="small" />
                          ) : (
                            <VolumeUpIcon fontSize="small" />
                          )}
                        </IconButton>
                      </Tooltip>
                    </div>
                  </TableCell>
                  <TableCell className={clsx(classes.bodyCell, classes.nameCell)}>
                    <Typography variant="body1" className={classes.ellipsis} noWrap>
                      {device.name}
                    </Typography>
                  </TableCell>
                  <TableCell className={classes.bodyCell}>
                    <Typography variant="body2" className={classes.ellipsis} noWrap>
                      {device.uptime}
                    </Typography>
                  </TableCell>
                  <TableCell className={classes.bodyCell}>
                    {device.retailHero ? (
                      <Tooltip title="RetailHero enabled">
                        <EmojiEventsIcon className={classes.heroIcon} fontSize="small" />
                      </Tooltip>
                    ) : (
                      <Typography variant="body2" color="textSecondary">
                        —
                      </Typography>
                    )}
                  </TableCell>
                  <TableCell className={classes.bodyCell}>
                    <Typography variant="body2" className={classes.ellipsis} noWrap>
                      {device.channel}
                    </Typography>
                  </TableCell>
                  <TableCell className={classes.bodyCell}>
                    <Typography variant="body2" className={classes.ellipsis} noWrap>
                      {device.channelList}
                    </Typography>
                  </TableCell>
                  <TableCell className={classes.bodyCell}>
                    <Typography variant="body2" className={classes.ellipsis} noWrap>
                      {device.organization}
                    </Typography>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableContainer>
        <Divider />
        <Box className={classes.footer} role="navigation" aria-label="Device pagination">
          <Typography variant="body2" color="textSecondary" noWrap>
            Showing {devices.length} of {devices.length} devices
          </Typography>
          <div className={classes.footerControls}>
            <Typography variant="body2" color="textSecondary">
              Page 1 of 1
            </Typography>
            <IconButton
              aria-label="Previous page"
              size="small"
              className={classes.footerButton}
              disabled
            >
              <ChevronLeftIcon />
            </IconButton>
            <IconButton
              aria-label="Next page"
              size="small"
              className={classes.footerButton}
              disabled
            >
              <ChevronRightIcon />
            </IconButton>
          </div>
        </Box>
      </Paper>
    </div>
  )
}

export default DevicesPage
