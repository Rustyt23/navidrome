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
} from '@material-ui/core'
import { BiDownload } from 'react-icons/bi'
import { httpClient } from '../dataProvider'

const emptyProgress = {
  missing: 0,
  fetching: 0,
  fetched: 0,
  updated: 0,
  left: 0,
}

const useStyles = makeStyles((theme) => ({
  button: {
    marginLeft: theme.spacing(1),
    textTransform: 'none',
    fontWeight: 600,
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
        <span>{translate('activity.musicbrainz.left')}</span>
        <span>{progress.left || 0}</span>
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

  return (
    <>
      <Button
        className={classes.button}
        color="inherit"
        variant="outlined"
        onClick={(event) => setAnchorEl(event.currentTarget)}
        data-testid="coverart-panel-btn"
      >
        Coverart
      </Button>
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
            <Grid container spacing={2}>
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
              <Grid item xs={12} md={4}>
                <ProgressCard
                  title="Cover Art"
                  progress={status.coverArt || emptyProgress}
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
