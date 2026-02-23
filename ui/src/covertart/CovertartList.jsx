import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  Button,
  Datagrid,
  Filter,
  FunctionField,
  List,
  Pagination,
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
  const [job, setJob] = useState(null)

  const progress = useMemo(() => {
    const response = job?.response || {}
    return {
      updated: response.updated || 0,
      skipped: response.skipped || 0,
      failed: response.failed || 0,
      processed: response.processed || 0,
    }
  }, [job])

  const fetchStatus = useCallback(async () => {
    const response = await httpClient(`${REST_URL}/song/metadata/spotify/status`, {
      method: 'GET',
    })
    setJob(response?.json || null)
    if (response?.json?.running === false) {
      setLoading(false)
      refresh()
    }
  }, [refresh])

  useEffect(() => {
    fetchStatus().catch(() => {})
  }, [fetchStatus])

  useEffect(() => {
    if (!job?.running) {
      setLoading(false)
      return undefined
    }

    setLoading(true)
    const id = setInterval(() => {
      fetchStatus().catch(() => {})
    }, 1000)

    return () => clearInterval(id)
  }, [fetchStatus, job?.running])

  const handleFetchSpotify = async () => {
    setLoading(true)
    try {
      const response = await httpClient(`${REST_URL}/song/metadata/spotify`, {
        method: 'POST',
      })

      const data = response?.json || null
      setJob(data)
      if (data?.running === false) {
        setLoading(false)
      }
      notify('Spotify metadata job started', 'info')
    } catch (error) {
      notify(error.message || 'Spotify metadata batch failed', 'warning')
      setLoading(false)
    }
  }

  return (
    <TopToolbar style={{ display: 'block' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 12 }}>
        <Button
          label="Fetch Missing Metadata (Spotify)"
          onClick={handleFetchSpotify}
          disabled={loading}
        />
        <span style={{ alignSelf: 'center' }}>
          Updated: {progress.updated} / Skipped: {progress.skipped} / Failed:{' '}
          {progress.failed} / Processed: {progress.processed}
        </span>
      </div>
    </TopToolbar>
  )
}

const CovertartPagination = (props) => <Pagination rowsPerPageOptions={[50, 100, 200, 500]} {...props} />

const CovertartList = (props) => (
  <List
    {...props}
    sort={{ field: 'title', order: 'ASC' }}
    filters={<CovertartFilter />}
    actions={<CovertartActions />}
    exporter={false}
    bulkActionButtons={false}
    perPage={50}
    pagination={<CovertartPagination />}
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
      <TextField source="releaseYear" label="Release Year" />
      <TextField source="genre" label="Genre" />
      <DurationField source="duration" />
    </Datagrid>
  </List>
)

export default CovertartList
