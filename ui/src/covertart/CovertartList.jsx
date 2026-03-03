import React, { useCallback, useEffect, useMemo, useState } from 'react'
import IconButton from '@material-ui/core/IconButton'
import Tooltip from '@material-ui/core/Tooltip'
import ImageOutlinedIcon from '@material-ui/icons/ImageOutlined'
import {
  Datagrid,
  DateField,
  Filter,
  FunctionField,
  List,
  SearchInput,
  TextField,
  TopToolbar,
  useListContext,
  useTranslate,
} from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import { DurationField, Pagination, ToggleFieldsMenu, useSelectedFields } from '../common'
import { httpClient } from '../dataProvider'
import subsonic from '../subsonic'
import CovertartSongBulkActions from './CovertartSongBulkActions'

const useStyles = makeStyles({
  mbidText: {
    fontFamily: 'monospace',
    fontSize: '0.75rem',
  },
})

const FetchedToggleButton = () => {
  const translate = useTranslate()
  const { filterValues, displayedFilters, setFilters } = useListContext()
  const missingCoverArtOn =
    filterValues?.fetched === false || filterValues?.fetched === 'false'

  const toggleFetchedFilter = () => {
    if (missingCoverArtOn) {
      const { fetched, ...rest } = filterValues || {}
      setFilters(rest, displayedFilters)
      return
    }
    setFilters({ ...(filterValues || {}), fetched: false }, displayedFilters)
  }

  return (
    <Tooltip
      title={`${translate('resources.covertart.fields.missingCoverArt')}: ${
        missingCoverArtOn ? 'On' : 'Off'
      }`}
    >
      <IconButton
        color={missingCoverArtOn ? 'primary' : 'default'}
        aria-label={translate('resources.covertart.fields.missingCoverArt')}
        onClick={toggleFetchedFilter}
      >
        <ImageOutlinedIcon />
      </IconButton>
    </Tooltip>
  )
}

const CovertartListActions = (props) => (
  <TopToolbar {...props}>
    <FetchedToggleButton />
    <ToggleFieldsMenu resource="covertart" />
  </TopToolbar>
)

const CovertartFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const CovertartList = (props) => {
  const classes = useStyles()
  const [confidenceEntries, setConfidenceEntries] = useState([])

  const loadConfidenceEntries = useCallback(() => {
    httpClient('/api/metadata/musicbrainz/spotify/confidence')
      .then(({ json }) => setConfidenceEntries(json?.items || []))
      .catch(() => setConfidenceEntries([]))
  }, [])

  useEffect(() => {
    loadConfidenceEntries()
    const intervalId = window.setInterval(loadConfidenceEntries, 5000)
    return () => window.clearInterval(intervalId)
  }, [loadConfidenceEntries])

  const confidenceBySong = useMemo(() => {
    const map = new Map()
    confidenceEntries.forEach((entry) => {
      if (entry?.songId) {
        map.set(entry.songId, entry)
      }
    })
    return map
  }, [confidenceEntries])

  const toggleableFields = useMemo(
    () => ({
      coverArt: (
        <FunctionField
          label="Cover Art"
          sortBy="title"
          render={(record) => {
            const coverSrc =
              subsonic.getCoverArtUrl(record, 50, true) || '/default-cover.png'

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
      ),
      title: <TextField source="title" />,
      artist: <TextField source="artist" label="Artist" />,
      album: <TextField source="album" label="Album" />,
      createdAt: <DateField source="createdAt" sortBy="recently_added" showTime />,
      year: <TextField source="year" label="Release Year" />,
      genre: <TextField source="genre" label="Genre" />,
      fetched: (
        <FunctionField
          label="Fetched"
          sortBy="fetched"
          render={(record) =>
            record?.hasCoverArt || Boolean(record?.coverPath) ? 'Yes' : 'No'
          }
        />
      ),
      confidence: (
        <FunctionField
          label="Confidence"
          sortBy="title"
          render={(record) => {
            const value = confidenceBySong.get(record.id)?.confidence
            if (typeof value !== 'number') {
              return ''
            }
            return value.toFixed(3)
          }}
        />
      ),
      spotifyMatch: (
        <FunctionField
          label="Spotify Match"
          sortBy="title"
          render={(record) => {
            const entry = confidenceBySong.get(record.id)
            if (!entry?.spotifyMatch) {
              return ''
            }
            const label = entry?.spotifyArtist
              ? `${entry.spotifyMatch} - ${entry.spotifyArtist}`
              : entry.spotifyMatch

            if (!entry?.spotifyUrl) {
              return label
            }

            return (
              <a href={entry.spotifyUrl} target="_blank" rel="noopener noreferrer">
                {label}
              </a>
            )
          }}
        />
      ),
      recordingMbid: (
        <FunctionField
          label="Recording MBID"
          sortBy="mbzRecordingID"
          render={(record) => {
            const recordingId = record?.mbzRecordingID

            if (!recordingId) {
              return <span className={classes.mbidText}></span>
            }

            return (
              <a
                href={`https://musicbrainz.org/recording/${recordingId}/tags`}
                target="_blank"
                rel="noopener noreferrer"
                className={classes.mbidText}
              >
                {recordingId}
              </a>
            )
          }}
        />
      ),
      releaseMbid: (
        <FunctionField
          label="Release MBID"
          sortBy="mbzReleaseId"
          render={(record) => {
            const releaseId = record?.mbzReleaseId

            if (!releaseId) {
              return <span className={classes.mbidText}></span>
            }

            return (
              <a
                href={`https://coverartarchive.org/release/${releaseId}/front`}
                target="_blank"
                rel="noopener noreferrer"
                className={classes.mbidText}
              >
                {releaseId}
              </a>
            )
          }}
        />
      ),
      duration: <DurationField source="duration" />,
    }),
    [classes.mbidText, confidenceBySong],
  )

  const columns = useSelectedFields({
    resource: 'covertart',
    columns: toggleableFields,
  })

  return (
    <List
      {...props}
      sort={{ field: 'title', order: 'ASC' }}
      filter={{ hascoverart: false }}
      actions={<CovertartListActions />}
      filters={<CovertartFilter />}
      exporter={false}
      perPage={50}
      pagination={<Pagination />}
      bulkActionButtons={
        <CovertartSongBulkActions onSpotifyCoverUpdated={loadConfidenceEntries} />
      }
    >
      <Datagrid rowClick={false}>
        {columns}
      </Datagrid>
    </List>
  )
}

export default CovertartList
