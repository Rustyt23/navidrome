import React, { cloneElement, useEffect, useMemo, useState } from 'react'
import { makeStyles, useMediaQuery } from '@material-ui/core'
import IconButton from '@material-ui/core/IconButton'
import Tooltip from '@material-ui/core/Tooltip'
import ImageOutlinedIcon from '@material-ui/icons/ImageOutlined'
import {
  Datagrid,
  DateField,
  Filter,
  FunctionField,
  List,
  sanitizeListRestProps,
  SearchInput,
  TextField,
  TopToolbar,
  useListContext,
  useTranslate,
} from 'react-admin'
import {
  DurationField,
  Pagination,
  ToggleFieldsMenu,
  useSelectedFields,
} from '../common'
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

const CovertartListActions = ({
  className,
  filters,
  resource,
  showFilter,
  displayedFilters,
  filterValues,
  ...rest
}) => {
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      <FetchedToggleButton />
      {filters &&
        cloneElement(filters, {
          resource,
          showFilter,
          displayedFilters,
          filterValues,
          context: 'button',
        })}
      {isNotSmall && <ToggleFieldsMenu resource="covertart" />}
    </TopToolbar>
  )
}

const CovertartFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const CovertartList = (props) => {
  const classes = useStyles()
  const [confidenceEntries, setConfidenceEntries] = useState([])

  useEffect(() => {
    httpClient('/api/metadata/musicbrainz/spotify/confidence')
      .then(({ json }) => setConfidenceEntries(json?.items || []))
      .catch(() => setConfidenceEntries([]))
  }, [])

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
          key="coverArt"
          source="coverArt"
          label="Cover Art"
          sortable={false}
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
      title: <TextField key="title" source="title" sortByOrder={'ASC'} />,
      artist: (
        <TextField
          key="artist"
          source="artist"
          label="Artist"
          sortBy="artist"
        />
      ),
      album: (
        <TextField key="album" source="album" label="Album" sortBy="album" />
      ),
      year: (
        <TextField
          key="year"
          source="year"
          label="Release Year"
          sortByOrder={'DESC'}
        />
      ),
      genre: (
        <TextField key="genre" source="genre" label="Genre" sortBy="genre" />
      ),
      createdAt: (
        <DateField
          key="createdAt"
          source="createdAt"
          label="Date Added"
          sortBy="recently_added"
          showTime
        />
      ),
      fetched: (
        <FunctionField
          key="fetched"
          source="fetched"
          label="Fetched"
          sortBy="fetched"
          render={(record) =>
            record?.hasCoverArt || Boolean(record?.coverPath) ? 'Yes' : 'No'
          }
        />
      ),
      confidence: (
        <FunctionField
          key="confidence"
          source="confidence"
          label="Confidence"
          sortBy="confidence"
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
          key="spotifyMatch"
          source="spotifyMatch"
          label="Spotify Match"
          sortBy="spotify_match"
          render={(record) => {
            const entry = confidenceBySong.get(record.id)
            if (!entry?.spotifyMatch) {
              return ''
            }
            if (!entry?.spotifyArtist) {
              return entry.spotifyMatch
            }
            return `${entry.spotifyMatch} - ${entry.spotifyArtist}`
          }}
        />
      ),
      mbzRecordingID: (
        <FunctionField
          key="mbzRecordingID"
          source="mbzRecordingID"
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
      mbzReleaseId: (
        <FunctionField
          key="mbzReleaseId"
          source="mbzReleaseId"
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
      duration: (
        <DurationField key="duration" source="duration" sortByOrder={'DESC'} />
      ),
    }),
    [classes.mbidText, confidenceBySong],
  )

  const columns = useSelectedFields({
    resource: 'covertart',
    columns: toggleableFields,
    defaultOff: [
      'confidence',
      'spotifyMatch',
      'mbzRecordingID',
      'mbzReleaseId',
    ],
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
    >
      <Datagrid
        rowClick={false}
        bulkActionButtons={<CovertartSongBulkActions />}
      >
        {columns}
      </Datagrid>
    </List>
  )
}

export default CovertartList
