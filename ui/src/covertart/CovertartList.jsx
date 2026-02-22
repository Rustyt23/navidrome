import React from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  List,
  SearchInput,
  TextField,
} from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import { DurationField, Pagination } from '../common'

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
      pagination={<Pagination />}
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
