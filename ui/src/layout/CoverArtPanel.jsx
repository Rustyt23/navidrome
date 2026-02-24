import React, { useCallback, useEffect, useState } from 'react'
import { useNotify, useTranslate } from 'react-admin'
import {
  Popover,
  Button,
  makeStyles,
  Card,
  CardContent,
  Box,
  Typography,
  Grid,
  IconButton,
  Tooltip,
} from '@material-ui/core'
import ImageOutlinedIcon from '@material-ui/icons/ImageOutlined'
import { BiDownload } from 'react-icons/bi'
import { MdSave } from 'react-icons/md'
import { httpClient } from '../dataProvider'

const emptyProgress = {
  existing: 0,
  missing: 0,
  fetching: 0,
  fetched: 0,
  updated: 0,
  left: 0,
  couldntFetch: 0,
}

const useStyles = makeStyles((theme) => ({
  iconButton: {
    color: 'inherit',
    padding: theme.spacing(1),
  },
  card: {
    minWidth: 680,
    maxWidth: 920,
    padding: theme.spacing(1),
  },
  title: {
    fontWeight: 600,
    marginBottom: theme.spacing(1.5),
  },
  actionBar: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: theme.spacing(2),
    gap: theme.spacing(2),
  },
  actions: {
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  progressCard: {
    height: '100%',
  },
  row: {
    display: 'flex',
    justifyContent: 'space-between',
    marginTop: theme.spacing(0.5),
  },
}))

const ProgressCard = ({ title, progress, translate, classes }) => (
  <Card variant="outlined" className={classes.progressCard}>
    <CardContent>
      <Typography variant="subtitle2">{title}</Typography>
      <Box className={classes.row}>
        <span>{translate('activity.musicbrainz.existing')}</span>
        <span>{progress.existing || 0}</span>
      </Box>
      <Box className={classes.row}>
        <span>{translate('activity.musicbrainz.missing')}</span>
        <span>{progress.missing || 0}</span>
      </Box>
      <Box className={classes.row}>
        <span>{translate('activity.musicbrainz.fetching')}</span>
        <span>{progress.fetching || 0}</span>
      </Box>
      <Box className={classes.row}>
        <span>{translate('activity.musicbrainz.fetched')}</span>
        <span>{progress.fetched || 0}</span>
      </Box>
      <Box className={classes.row}>
        <span>{translate('activity.musicbrainz.updated')}</span>
        <span>{progress.updated || 0}</span>
      </Box>
      <Box className={classes.row}>
        <span>{translate('activity.musicbrainz.toBeFetch')}</span>
        <span>{progress.left || 0}</span>
      </Box>
      <Box className={classes.row}>
        <span>{translate('activity.musicbrainz.couldntFetch')}</span>
        <span>{progress.couldntFetch || 0}</span>
      </Box>
    </CardContent>
  </Card>
)

const CoverArtPanel = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const [anchorEl, setAnchorEl] = useState(null)
  const [status, setStatus] = useState({
    running: false,
    album: emptyProgress,
    year: emptyProgress,
    genre: emptyProgress,
    recordingMbid: emptyProgress,
    releaseMbid: emptyProgress,
    coverArt: emptyProgress,
  })

  const open = Boolean(anchorEl)

  const loadStatus = useCallback(() => {
    httpClient('/api/metadata/musicbrainz/status')
      .then(({ json }) => {
        setStatus(
          json || {
            running: false,
            album: emptyProgress,
            year: emptyProgress,
            genre: emptyProgress,
            recordingMbid: emptyProgress,
            releaseMbid: emptyProgress,
            coverArt: emptyProgress,
          },
        )
      })
      .catch(() => {})
  }, [])

  useEffect(() => {
    if (!open) {
      return undefined
    }
    loadStatus()
    const interval = setInterval(
      () => {
        loadStatus()
      },
      status.running ? 2000 : 5000,
    )
    return () => clearInterval(interval)
  }, [open, status.running, loadStatus])

  const startFetch = () => {
    httpClient('/api/metadata/musicbrainz/fetch', { method: 'POST' })
      .then(({ status: code }) => {
        if (code === 202) {
          notify('activity.musicbrainz.started', 'info')
        } else {
          notify('activity.musicbrainz.alreadyRunning', 'warning')
        }
        loadStatus()
      })
      .catch(() => notify('activity.musicbrainz.failed', 'warning'))
  }

  const saveMetadata = () => {
    httpClient('/api/metadata/musicbrainz/save', { method: 'POST' })
      .then(({ status: code }) => {
        if (code === 200) {
          notify('activity.musicbrainz.saved', 'info')
        } else if (code === 207) {
          notify('activity.musicbrainz.savePartial', 'warning')
        } else {
          notify('activity.musicbrainz.saveFailed', 'warning')
        }
      })
      .catch(() => notify('activity.musicbrainz.saveFailed', 'warning'))
  }

  return (
    <>
      <Tooltip title="Coverart">
        <IconButton
          className={classes.iconButton}
          onClick={(event) => setAnchorEl(event.currentTarget)}
          data-testid="coverart-panel-btn"
        >
          <ImageOutlinedIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Popover
        id="panel-coverart"
        anchorEl={anchorEl}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        open={open}
        onClose={() => setAnchorEl(null)}
      >
        <Card className={classes.card}>
          <CardContent>
            <Box className={classes.actionBar}>
              <Typography variant="h6" className={classes.title}>
                {translate('activity.musicbrainz.title')}
              </Typography>
              <Box className={classes.actions}>
                <Button
                  color="primary"
                  variant="contained"
                  startIcon={<MdSave />}
                  onClick={saveMetadata}
                  disabled={status.running}
                  data-testid="coverart-metadata-save-btn"
                >
                  {translate('activity.musicbrainz.save')}
                </Button>
                <Button
                  color="primary"
                  variant="contained"
                  startIcon={<BiDownload />}
                  onClick={startFetch}
                  disabled={status.running}
                  data-testid="coverart-metadata-fetch-btn"
                >
                  {translate('activity.musicbrainz.fetch')}
                </Button>
              </Box>
            </Box>
            <Grid container spacing={2}>
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title="Cover Art"
                  progress={status.coverArt || emptyProgress}
                  translate={translate}
                  classes={classes}
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title={translate('activity.musicbrainz.album')}
                  progress={status.album || emptyProgress}
                  translate={translate}
                  classes={classes}
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title={translate('activity.musicbrainz.year')}
                  progress={status.year || emptyProgress}
                  translate={translate}
                  classes={classes}
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title={translate('activity.musicbrainz.genre')}
                  progress={status.genre || emptyProgress}
                  translate={translate}
                  classes={classes}
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title="Recording MBID"
                  progress={status.recordingMbid || emptyProgress}
                  translate={translate}
                  classes={classes}
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title="Release MBID"
                  progress={status.releaseMbid || emptyProgress}
                  translate={translate}
                  classes={classes}
                />
              </Grid>
            </Grid>
          </CardContent>
        </Card>
      </Popover>
    </>
  )
}

export default CoverArtPanel
