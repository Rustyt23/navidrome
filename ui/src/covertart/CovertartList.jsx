import React from 'react'
import IconButton from '@material-ui/core/IconButton'
import Tooltip from '@material-ui/core/Tooltip'
import ImageOutlinedIcon from '@material-ui/icons/ImageOutlined'
import {
  Datagrid,
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
import { DurationField, Pagination } from '../common'
import subsonic from '../subsonic'

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
  </TopToolbar>
)

const CovertartFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const CovertartList = (props) => {
  const classes = useStyles()

  return (
    <List
      {...props}
      sort={{ field: 'title', order: 'ASC' }}
      actions={<CovertartListActions />}
      filters={<CovertartFilter />}
      exporter={false}
      bulkActionButtons={false}
      perPage={50}
      pagination={<Pagination />}
    >
      <Datagrid rowClick={false}>
        <FunctionField
          label="Cover Art"
          sortable={false}
          render={(record) => {
            const coverSrc = subsonic.getCoverArtUrl(record, 50, true) || '/default-cover.png'

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
          label="Fetched"
          sortBy="fetched"
          render={(record) =>
            record?.hasCoverArt || Boolean(record?.coverPath) ? 'Yes' : 'No'
          }
        />
        <FunctionField
          label="Recording MBID"
          sortable={false}
          render={(record) => (
            <span className={classes.mbidText}>
              {record?.mbzRecordingID || ''}
            </span>
          )}
        />
        <FunctionField
          label="Release MBID"
          sortable={false}
          render={(record) => (
            <span className={classes.mbidText}>
              {record?.mbzReleaseId || ''}
            </span>
          )}
        />
        <DurationField source="duration" />
      </Datagrid>
    </List>
  )
}

export default CovertartList
