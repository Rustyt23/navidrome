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
})

const CovertartFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const emptyProgress = { missing: 0, fetching: 0, fetched: 0, updated: 0, left: 0 }

const MetadataProgressCard = ({ title, progress, translate }) => (
  <Card>
    <CardContent>
      <Typography variant="subtitle2">{title}</Typography>
      <Box display="flex" justifyContent="space-between" mt={1}>
        <span>{translate('activity.musicbrainz.missing')}</span>
        <span>{progress.missing || 0}</span>
      </Box>
      <Box display="flex" justifyContent="space-between">
        <span>{translate('activity.musicbrainz.fetching')}</span>
        <span>{progress.fetching || 0}</span>
      </Box>
      <Box display="flex" justifyContent="space-between">
        <span>{translate('activity.musicbrainz.fetched')}</span>
        <span>{progress.fetched || 0}</span>
      </Box>
      <Box display="flex" justifyContent="space-between">
        <span>{translate('activity.musicbrainz.updated')}</span>
        <span>{progress.updated || 0}</span>
      </Box>
      <Box display="flex" justifyContent="space-between">
        <span>{translate('activity.musicbrainz.left')}</span>
        <span>{progress.left || 0}</span>
      </Box>
    </CardContent>
  </Card>
)

const CovertartListActions = () => {
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
      )}
      {isAdmin && (
        <Box width="100%" mt={2}>
          <Typography variant="subtitle1">
            {translate('activity.musicbrainz.title')}
          </Typography>
          <Grid container spacing={2}>
            <Grid item xs={12} md={3}>
              <MetadataProgressCard
                title={translate('activity.musicbrainz.album')}
                progress={status.album || emptyProgress}
                translate={translate}
              />
            </Grid>
            <Grid item xs={12} md={3}>
              <MetadataProgressCard
                title={translate('activity.musicbrainz.year')}
                progress={status.year || emptyProgress}
                translate={translate}
              />
            </Grid>
            <Grid item xs={12} md={2}>
              <MetadataProgressCard
                title={translate('activity.musicbrainz.genre')}
                progress={status.genre || emptyProgress}
                translate={translate}
              />
            </Grid>
            <Grid item xs={12} md={2}>
              <MetadataProgressCard
                title="Recording MBID"
                progress={status.recordingMbid || emptyProgress}
                translate={translate}
              />
            </Grid>
            <Grid item xs={12} md={2}>
              <MetadataProgressCard
                title="Release MBID"
                progress={status.releaseMbid || emptyProgress}
                translate={translate}
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
          render={(record) => (
            <img
              src={record?.cover_art_url || '/default-cover.png'}
              alt={record.title || 'cover art'}
              width="50"
              height="50"
              onError={(e) => {
                e.currentTarget.src = '/default-cover.png'
              }}
            />
          )}
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
