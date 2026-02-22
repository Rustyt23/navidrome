import React, { useCallback, useEffect, useState } from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  List,
  SearchInput,
  TextField,
  TopToolbar,
  useNotify,
  usePermissions,
  useTranslate,
} from 'react-admin'
import { Box, Button, Card, CardContent, Grid, Typography } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import { BiDownload } from 'react-icons/bi'
import { DurationField } from '../common'
import { httpClient } from '../dataProvider'

const useStyles = makeStyles({
  mbidText: {
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
  actionsContainer: {
    width: '100%',
    marginTop: '0.5rem',
  },
  actionsHeader: {
    display: 'flex',
    flexWrap: 'wrap',
    alignItems: 'center',
    gap: '0.75rem',
    marginBottom: '0.75rem',
  },
  progressCard: {
    height: '100%',
  },
  progressCardContent: {
    display: 'flex',
    flexDirection: 'column',
    gap: '0.25rem',
  },
  progressRow: {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
  },
})

const CovertartFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const emptyProgress = { missing: 0, fetching: 0, fetched: 0, updated: 0, left: 0 }

const MetadataProgressCard = ({ title, progress, translate, classes }) => (
  <Card className={classes.progressCard}>
    <CardContent className={classes.progressCardContent}>
      <Typography variant="subtitle2">{title}</Typography>
      <Box className={classes.progressRow} mt={1}>
        <Typography variant="body2">{translate('activity.musicbrainz.missing')}</Typography>
        <Typography variant="body2">{progress.missing || 0}</Typography>
      </Box>
      <Box className={classes.progressRow}>
        <Typography variant="body2">{translate('activity.musicbrainz.fetching')}</Typography>
        <Typography variant="body2">{progress.fetching || 0}</Typography>
      </Box>
      <Box className={classes.progressRow}>
        <Typography variant="body2">{translate('activity.musicbrainz.fetched')}</Typography>
        <Typography variant="body2">{progress.fetched || 0}</Typography>
      </Box>
      <Box className={classes.progressRow}>
        <Typography variant="body2">{translate('activity.musicbrainz.updated')}</Typography>
        <Typography variant="body2">{progress.updated || 0}</Typography>
      </Box>
      <Box className={classes.progressRow}>
        <Typography variant="body2">{translate('activity.musicbrainz.left')}</Typography>
        <Typography variant="body2">{progress.left || 0}</Typography>
      </Box>
    </CardContent>
  </Card>
)

const CovertartListActions = () => {
  const classes = useStyles()
  const translate = useTranslate()
  const notify = useNotify()
  const { permissions } = usePermissions()
  const isAdmin = permissions === 'admin'
  const [status, setStatus] = useState({
    running: false,
    album: emptyProgress,
    year: emptyProgress,
    genre: emptyProgress,
    recordingMbid: emptyProgress,
    releaseMbid: emptyProgress,
    coverArt: emptyProgress,
  })

  const loadStatus = useCallback(() => {
    if (!isAdmin) {
      return
    }
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
  }, [isAdmin])

  useEffect(() => {
    loadStatus()
  }, [loadStatus])

  useEffect(() => {
    if (!status.running) {
      return undefined
    }
    const timer = setInterval(() => loadStatus(), 2000)
    return () => clearInterval(timer)
  }, [status.running, loadStatus])

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
    <TopToolbar>
      {isAdmin && (
        <Box className={classes.actionsContainer}>
          <Box className={classes.actionsHeader}>
            <Button
              color="primary"
              variant="contained"
              startIcon={<BiDownload />}
              onClick={startFetch}
              disabled={status.running}
              data-testid="covertart-metadata-fetch-btn"
            >
              {translate('activity.musicbrainz.fetch')}
            </Button>
            <Typography variant="subtitle1">
              {translate('activity.musicbrainz.title')}
            </Typography>
          </Box>
          <Grid container spacing={2}>
            <Grid item xs={12} sm={6} md={4} lg={2}>
              <MetadataProgressCard
                title={translate('activity.musicbrainz.album')}
                progress={status.album || emptyProgress}
                translate={translate}
                classes={classes}
              />
            </Grid>
            <Grid item xs={12} sm={6} md={4} lg={2}>
              <MetadataProgressCard
                title={translate('activity.musicbrainz.year')}
                progress={status.year || emptyProgress}
                translate={translate}
                classes={classes}
              />
            </Grid>
            <Grid item xs={12} sm={6} md={4} lg={2}>
              <MetadataProgressCard
                title={translate('activity.musicbrainz.genre')}
                progress={status.genre || emptyProgress}
                translate={translate}
                classes={classes}
              />
            </Grid>
            <Grid item xs={12} sm={6} md={4} lg={2}>
              <MetadataProgressCard
                title="Recording MBID"
                progress={status.recordingMbid || emptyProgress}
                translate={translate}
                classes={classes}
              />
            </Grid>
            <Grid item xs={12} sm={6} md={4} lg={2}>
              <MetadataProgressCard
                title="Release MBID"
                progress={status.releaseMbid || emptyProgress}
                translate={translate}
                classes={classes}
              />
            </Grid>
            <Grid item xs={12} sm={6} md={4} lg={2}>
              <MetadataProgressCard
                title="Cover Art"
                progress={status.coverArt || emptyProgress}
                translate={translate}
                classes={classes}
              />
            </Grid>
          </Grid>
        </Box>
      )}
    </TopToolbar>
  )
}

const CovertartList = (props) => {
  const classes = useStyles()

  return (
    <List
      {...props}
      sort={{ field: 'title', order: 'ASC' }}
      filters={<CovertartFilter />}
      exporter={false}
      bulkActionButtons={false}
      perPage={50}
      actions={<CovertartListActions />}
    >
      <Datagrid rowClick={false}>
        <FunctionField
          label="Cover Art"
          sortable={false}
          render={(record) => {
            const coverSrc = record?.mbzReleaseId
              ? `/api/cover/${record.mbzReleaseId}`
              : '/default-cover.png'

            return (
              <img
                src={coverSrc}
                alt={record.title || 'cover art'}
                width="50"
                height="50"
                loading="lazy"
              />
            )
          }}
        />
        <TextField source="title" />
        <TextField source="artist" label="Artist" />
        <TextField source="album" label="Album" />
        <TextField source="year" label="Release Year" />
        <TextField source="genre" label="Genre" />
        <FunctionField
          label="Recording MBID"
          sortable={false}
          render={(record) => (
            <span className={classes.mbidText}>{record?.mbzRecordingID || ''}</span>
          )}
        />
        <FunctionField
          label="Release MBID"
          sortable={false}
          render={(record) => (
            <span className={classes.mbidText}>{record?.mbzReleaseId || ''}</span>
          )}
        />
        <DurationField source="duration" />
      </Datagrid>
    </List>
  )
}

export default CovertartList
