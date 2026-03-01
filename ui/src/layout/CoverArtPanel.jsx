import React, { useCallback, useEffect, useMemo, useState } from 'react'
import { useNotify, useTranslate, useUnselectAll } from 'react-admin'
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
import { useSelector } from 'react-redux'
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

const emptySaveSummary = {
  saved: 0,
  remaining: 0,
  saving: 0,
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
  spotifyButton: {
    backgroundColor: '#1DB954',
    color: theme.palette.common.white,
    '&:hover': {
      backgroundColor: '#1aa34a',
    },
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
    newMbGenre: emptyProgress,
    recordingMbid: emptyProgress,
    releaseMbid: emptyProgress,
    coverArt: emptyProgress,
  })
  const [spotifyStatus, setSpotifyStatus] = useState({
    running: false,
    album: emptyProgress,
    coverArt: emptyProgress,
  })
  const [saveSummary, setSaveSummary] = useState(emptySaveSummary)

  const unselectAll = useUnselectAll()
  const selectedCoverArtIds = useSelector(
    (state) => state?.admin?.resources?.covertart?.list?.selectedIds || [],
  )
  const selectedSongIDs = useMemo(
    () => selectedCoverArtIds.map((id) => String(id)),
    [selectedCoverArtIds],
  )

  const open = Boolean(anchorEl)

  const loadStatus = useCallback(() => {
    httpClient('/api/metadata/musicbrainz/status')
      .then(({ json }) => {
        setStatus(
          json || {
            running: false,
            album: emptyProgress,
            year: emptyProgress,
            newMbGenre: emptyProgress,
            recordingMbid: emptyProgress,
            releaseMbid: emptyProgress,
            coverArt: emptyProgress,
          },
        )
      })
      .catch(() => {})

    httpClient('/api/metadata/musicbrainz/spotify/status')
      .then(({ json }) => {
        setSpotifyStatus(
          json || {
            running: false,
            album: emptyProgress,
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
      status.running || spotifyStatus.running ? 2000 : 5000,
    )
    return () => clearInterval(interval)
  }, [open, status.running, spotifyStatus.running, loadStatus])

  const postFetch = (url, startedMessage) => {
    const options = { method: 'POST' }

    if (selectedSongIDs.length > 0) {
      options.body = JSON.stringify({ songIds: selectedSongIDs })
      options.headers = new Headers({ 'Content-Type': 'application/json' })
    }

    httpClient(url, options)
      .then(({ status: code }) => {
        if (code === 202) {
          notify(startedMessage, 'info')
          if (selectedSongIDs.length > 0) {
            unselectAll('covertart')
          }
        } else {
          notify('activity.musicbrainz.alreadyRunning', 'warning')
        }
        loadStatus()
      })
      .catch(() => notify('activity.musicbrainz.failed', 'warning'))
  }

  const startFetch = () => {
    postFetch('/api/metadata/musicbrainz/fetch', 'activity.musicbrainz.started')
  }

  const saveMetadata = () => {
    setSaveSummary((current) => ({ ...current, saving: 1 }))

    httpClient('/api/metadata/musicbrainz/save', { method: 'POST' })
      .then(({ status: code, json }) => {
        if (code === 200) {
          notify('activity.musicbrainz.saved', 'info')
          setSaveSummary({
            saved: json?.saved || 0,
            remaining: json?.remaining || 0,
            saving: json?.saving || 0,
          })
        } else {
          notify('activity.musicbrainz.saveFailed', 'warning')
          setSaveSummary((current) => ({ ...current, saving: 0 }))
        }
      })
      .catch(() => {
        notify('activity.musicbrainz.saveFailed', 'warning')
        setSaveSummary((current) => ({ ...current, saving: 0 }))
      })
  }

  const startSpotifyFetch = () => {
    postFetch(
      '/api/metadata/musicbrainz/spotify/fetch',
      'activity.musicbrainz.spotifyStarted',
    )
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
                  variant="contained"
                  className={classes.spotifyButton}
                  startIcon={<BiDownload />}
                  onClick={startSpotifyFetch}
                  disabled={status.running || spotifyStatus.running}
                  data-testid="coverart-metadata-fetch-spotify-btn"
                >
                  {translate('activity.musicbrainz.fetchSpotify')}
                </Button>
                <Button
                  color="primary"
                  variant="contained"
                  startIcon={<MdSave />}
                  onClick={saveMetadata}
                  disabled={status.running || spotifyStatus.running}
                  data-testid="coverart-metadata-save-btn"
                >
                  {translate('activity.musicbrainz.save')}
                </Button>
                <Button
                  color="primary"
                  variant="contained"
                  startIcon={<BiDownload />}
                  onClick={startFetch}
                  disabled={status.running || spotifyStatus.running}
                  data-testid="coverart-metadata-fetch-btn"
                >
                  {translate('activity.musicbrainz.fetch')}
                </Button>
              </Box>
            </Box>
            <Grid container spacing={2}>
              <Grid item xs={12} md={4}>
                <Card variant="outlined" className={classes.progressCard}>
                  <CardContent>
                    <Typography variant="subtitle2">
                      {translate('activity.musicbrainz.saveProgressTitle')}
                    </Typography>
                    <Box className={classes.row}>
                      <span>{translate('activity.musicbrainz.savedCount')}</span>
                      <span>{saveSummary.saved || 0}</span>
                    </Box>
                    <Box className={classes.row}>
                      <span>{translate('activity.musicbrainz.remainingCount')}</span>
                      <span>{saveSummary.remaining || 0}</span>
                    </Box>
                    <Box className={classes.row}>
                      <span>{translate('activity.musicbrainz.savingCount')}</span>
                      <span>{saveSummary.saving || 0}</span>
                    </Box>
                  </CardContent>
                </Card>
              </Grid>
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title="Cover Art (MusicBrainz)"
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
                  title="New MB-Genre"
                  progress={status.newMbGenre || emptyProgress}
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
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title="Cover Art (Spotify)"
                  progress={spotifyStatus.coverArt || emptyProgress}
                  translate={translate}
                  classes={classes}
                />
              </Grid>
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title="Album (Spotify)"
                  progress={spotifyStatus.album || emptyProgress}
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
