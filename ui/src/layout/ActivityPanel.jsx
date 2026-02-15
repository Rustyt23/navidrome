import React, { useState, useEffect, useCallback } from 'react'
import { useSelector } from 'react-redux'
import { useNotify, useTranslate } from 'react-admin'
import {
  Popover,
  CircularProgress,
  IconButton,
  makeStyles,
  Tooltip,
  Card,
  CardContent,
  CardActions,
  Divider,
  Box,
  Typography,
} from '@material-ui/core'
import { FiActivity } from 'react-icons/fi'
import { BiError, BiLink, BiDownload } from 'react-icons/bi'
import { VscSync } from 'react-icons/vsc'
import { GiMagnifyingGlass } from 'react-icons/gi'
import subsonic from '../subsonic'
import { httpClient } from '../dataProvider'
import { useInitialScanStatus } from './useInitialScanStatus'
import { useInterval } from '../common'
import { useScanElapsedTime } from './useScanElapsedTime'
import { formatDuration, formatShortDuration } from '../utils'
import config from '../config'

const useStyles = makeStyles((theme) => ({
  wrapper: {
    position: 'relative',
    color: (props) => (props.up ? null : 'orange'),
  },
  progress: {
    color: theme.palette.primary.light,
    position: 'absolute',
    top: 10,
    left: 10,
    zIndex: 1,
  },
  button: {
    color: 'inherit',
    zIndex: 2,
  },
  counterStatus: {
    minWidth: '20em',
  },
  error: {
    color: theme.palette.error.main,
  },
  card: {
    maxWidth: 'none',
  },
  cardContent: {
    padding: theme.spacing(2, 3),
  },
  metadataGrid: {
    display: 'grid',
    gap: theme.spacing(1),
    gridTemplateColumns: 'repeat(3, minmax(11em, 1fr))',
    marginTop: theme.spacing(2),
  },
  metadataCard: {
    border: `1px solid ${theme.palette.divider}`,
    borderRadius: theme.shape.borderRadius,
    padding: theme.spacing(1.2),
  },
  metadataRow: {
    display: 'flex',
    justifyContent: 'space-between',
    fontSize: '0.8rem',
    marginTop: theme.spacing(0.5),
  },
}))

const getUptime = (serverStart) =>
  formatDuration((Date.now() - serverStart.startTime) / 1000)

const Uptime = () => {
  const serverStart = useSelector((state) => state.activity.serverStart)
  const [uptime, setUptime] = useState(getUptime(serverStart))
  useInterval(() => {
    setUptime(getUptime(serverStart))
  }, 1000)
  return <span>{uptime}</span>
}

const emptyProgress = { missing: 0, fetching: 0, fetched: 0, updated: 0, left: 0 }
const emptyMetadataStatus = {
  running: false,
  album: emptyProgress,
  year: emptyProgress,
  genre: emptyProgress,
}

const ActivityPanel = () => {
  const serverStart = useSelector((state) => state.activity.serverStart)
  const up = serverStart.startTime
  const scanStatus = useSelector((state) => state.activity.scanStatus)
  const elapsed = useScanElapsedTime(
    scanStatus.scanning,
    scanStatus.elapsedTime,
  )
  const [acknowledgedError, setAcknowledgedError] = useState(null)
  const isErrorVisible =
    scanStatus.error && scanStatus.error !== acknowledgedError
  const classes = useStyles({
    up: up && (!scanStatus.error || !isErrorVisible),
  })
  const translate = useTranslate()
  const notify = useNotify()
  const [anchorEl, setAnchorEl] = useState(null)
  const [metadataStatus, setMetadataStatus] = useState(emptyMetadataStatus)
  const open = Boolean(anchorEl)
  useInitialScanStatus()

  const fetchMetadataStatus = useCallback(() => {
    httpClient('/api/metadata/musicbrainz/status')
      .then(({ json }) => setMetadataStatus(json || emptyMetadataStatus))
      .catch(() => {})
  }, [])

  const handleMenuOpen = (event) => {
    if (scanStatus.error) {
      setAcknowledgedError(scanStatus.error)
    }
    setAnchorEl(event.currentTarget)
  }

  const handleMenuClose = () => setAnchorEl(null)
  const triggerScan = (full) => () => subsonic.startScan({ fullScan: full })
  const triggerSync = () =>
    httpClient('/api/sync')
      .then(({ json }) => {
        if (json?.message) {
          notify(json.message, 'info')
        }
      })
      .catch(() => notify('Sync failed', 'warning'))

  const triggerMetadataFetch = () =>
    httpClient('/api/metadata/musicbrainz/fetch', { method: 'POST' })
      .then(({ status }) => {
        if (status === 202) {
          notify('activity.musicbrainz.started', 'info')
        } else {
          notify('activity.musicbrainz.alreadyRunning', 'warning')
        }
        fetchMetadataStatus()
      })
      .catch(() => notify('activity.musicbrainz.failed', 'warning'))

  useEffect(() => {
    if (serverStart.version && serverStart.version !== config.version) {
      notify('ra.notification.new_version', 'info', {}, false, 604800000 * 50)
    }
  }, [serverStart, notify])

  useEffect(() => {
    if (open) {
      fetchMetadataStatus()
    }
  }, [open, fetchMetadataStatus])

  useInterval(() => {
    if (open && metadataStatus.running) {
      fetchMetadataStatus()
    }
  }, open && metadataStatus.running ? 2000 : null)

  const tooltipTitle = scanStatus.error
    ? `${translate('activity.status')}: ${scanStatus.error}`
    : translate('activity.title')

  const lastScanType = (() => {
    switch (scanStatus.scanType) {
      case 'full':
        return translate('activity.fullScan')
      case 'quick':
        return translate('activity.quickScan')
      default:
        return ''
    }
  })()

  const renderMetadataCard = (title, key) => {
    const progress = metadataStatus?.[key] || emptyProgress
    return (
      <Box className={classes.metadataCard}>
        <Typography variant="subtitle2">{title}</Typography>
        <Box className={classes.metadataRow}>
          <span>{translate('activity.musicbrainz.missing')}</span>
          <span>{progress.missing || 0}</span>
        </Box>
        <Box className={classes.metadataRow}>
          <span>{translate('activity.musicbrainz.fetching')}</span>
          <span>{progress.fetching || 0}</span>
        </Box>
        <Box className={classes.metadataRow}>
          <span>{translate('activity.musicbrainz.fetched')}</span>
          <span>{progress.fetched || 0}</span>
        </Box>
        <Box className={classes.metadataRow}>
          <span>{translate('activity.musicbrainz.updated')}</span>
          <span>{progress.updated || 0}</span>
        </Box>
        <Box className={classes.metadataRow}>
          <span>{translate('activity.musicbrainz.left')}</span>
          <span>{progress.left || 0}</span>
        </Box>
      </Box>
    )
  }

  return (
    <div className={classes.wrapper}>
      <Tooltip title={tooltipTitle}>
        <IconButton className={classes.button} onClick={handleMenuOpen}>
          {!up || isErrorVisible ? (
            <BiError data-testid="activity-error-icon" size={'20'} />
          ) : (
            <FiActivity data-testid="activity-ok-icon" size={'20'} />
          )}
        </IconButton>
      </Tooltip>
      {scanStatus.scanning && (
        <CircularProgress size={24} className={classes.progress} />
      )}
      <Popover
        id="panel-activity"
        anchorEl={anchorEl}
        anchorOrigin={{
          vertical: 'bottom',
          horizontal: 'right',
        }}
        transformOrigin={{
          vertical: 'top',
          horizontal: 'right',
        }}
        open={open}
        onClose={handleMenuClose}
      >
        <Card className={classes.card}>
          <CardContent className={classes.cardContent}>
            <Box display="flex" className={classes.counterStatus}>
              <Box component="span" flex={2}>
                {translate('activity.serverUptime')}:
              </Box>
              <Box component="span" flex={1}>
                {up ? <Uptime /> : translate('activity.serverDown')}
              </Box>
            </Box>
          </CardContent>
          <Divider />
          <CardContent className={classes.cardContent}>
            <Box display="flex" className={classes.counterStatus}>
              <Box component="span" flex={2}>
                {translate('activity.totalScanned')}:
              </Box>
              <Box component="span" flex={1}>
                {scanStatus.folderCount || '-'}
              </Box>
            </Box>

            <Box display="flex" className={classes.counterStatus} mt={2}>
              <Box component="span" flex={2}>
                {translate('activity.scanType')}:
              </Box>
              <Box component="span" flex={1}>
                {lastScanType}
              </Box>
            </Box>

            <Box display="flex" className={classes.counterStatus} mt={2}>
              <Box component="span" flex={2}>
                {translate('activity.elapsedTime')}:
              </Box>
              <Box component="span" flex={1}>
                {formatShortDuration(elapsed)}
              </Box>
            </Box>

            <Box mt={2}>
              <Typography variant="subtitle2">
                {translate('activity.musicbrainz.title')}
              </Typography>
              <Box className={classes.metadataGrid}>
                {renderMetadataCard(
                  translate('activity.musicbrainz.album'),
                  'album',
                )}
                {renderMetadataCard(translate('activity.musicbrainz.year'), 'year')}
                {renderMetadataCard(
                  translate('activity.musicbrainz.genre'),
                  'genre',
                )}
              </Box>
            </Box>

            {scanStatus.error && (
              <Box
                display="flex"
                flexDirection="column"
                mt={2}
                className={classes.error}
              >
                <Typography variant="subtitle2">
                  {translate('activity.status')}:
                </Typography>
                <Typography variant="body2">{scanStatus.error}</Typography>
              </Box>
            )}
          </CardContent>
          <Divider />
          <CardActions>
            <Tooltip title={translate('activity.quickScan')}>
              <IconButton
                onClick={triggerScan(false)}
                disabled={scanStatus.scanning}
              >
                <VscSync />
              </IconButton>
            </Tooltip>
            <Tooltip title={translate('activity.sync')}>
              <IconButton onClick={triggerSync} disabled={scanStatus.scanning}>
                <BiLink />
              </IconButton>
            </Tooltip>
            <Tooltip title={translate('activity.fullScan')}>
              <IconButton
                onClick={triggerScan(true)}
                disabled={scanStatus.scanning}
              >
                <GiMagnifyingGlass />
              </IconButton>
            </Tooltip>
            <Tooltip title={translate('activity.musicbrainz.fetch')}>
              <IconButton
                onClick={triggerMetadataFetch}
                disabled={metadataStatus.running}
              >
                <BiDownload />
              </IconButton>
            </Tooltip>
          </CardActions>
        </Card>
      </Popover>
    </div>
  )
}

export default ActivityPanel
