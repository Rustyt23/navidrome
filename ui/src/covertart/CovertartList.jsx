import React from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  List,
  SearchInput,
  TextField,
} from 'react-admin'
import { DurationField } from '../common'
import subsonic from '../subsonic'

const CovertartFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const CovertartList = (props) => (
  <List
    {...props}
    sort={{ field: 'title', order: 'ASC' }}
    filters={<CovertartFilter />}
    exporter={false}
    bulkActionButtons={false}
    perPage={50}
  >
    <Datagrid rowClick={false}>
      <FunctionField
        label="Cover Art"
        sortable={false}
        render={(record) => (
          <img
            src={subsonic.getCoverArtUrl(record, 64, true)}
            alt={record.title || 'cover art'}
            width="64"
            height="64"
          />
        )}
      />
      <TextField source="title" />
      <TextField source="artist" label="Artist" />
      <TextField source="album" label="Album" />
      <TextField source="year" label="Release Year" />
      <TextField source="genre" label="Genre" />
      <DurationField source="duration" />
    </Datagrid>
  </List>
)

export default CovertartList
