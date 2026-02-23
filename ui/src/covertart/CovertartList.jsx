import React, { useState } from 'react'
import {
  Button,
  Datagrid,
  Filter,
  FunctionField,
  List,
  SearchInput,
  TextField,
  TopToolbar,
  useNotify,
  useRefresh,
} from 'react-admin'
import { DurationField } from '../common'
import subsonic from '../subsonic'
import { REST_URL } from '../consts'
import { httpClient } from '../dataProvider'

const CovertartFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="title" alwaysOn />
  </Filter>
)

const CovertartActions = () => {
  const notify = useNotify()
  const refresh = useRefresh()
  const [loading, setLoading] = useState(false)
  const [progress, setProgress] = useState({ updated: 0, skipped: 0, failed: 0 })

  const handleFetchSpotify = async () => {
    setLoading(true)
    try {
      const response = await httpClient(`${REST_URL}/song/metadata/spotify?limit=50`, {
        method: 'POST',
      })

      const data = response?.json || {}
      setProgress({
        updated: data.updated || 0,
        skipped: data.skipped || 0,
        failed: data.failed || 0,
      })
      notify('Spotify metadata batch completed', 'info')
      refresh()
    } catch (error) {
      notify(error.message || 'Spotify metadata batch failed', 'warning')
    } finally {
      setLoading(false)
    }
  }

  return (
    <TopToolbar>
      <Button
        label="Fetch Missing Metadata (Spotify)"
        onClick={handleFetchSpotify}
        disabled={loading}
      />
      <span style={{ marginLeft: 12, alignSelf: 'center' }}>
        Updated: {progress.updated} / Skipped: {progress.skipped} / Failed:{' '}
        {progress.failed}
      </span>
    </TopToolbar>
  )
}

const CovertartList = (props) => (
  <List
    {...props}
    sort={{ field: 'title', order: 'ASC' }}
    filters={<CovertartFilter />}
    actions={<CovertartActions />}
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
