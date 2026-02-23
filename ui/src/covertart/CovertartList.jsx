import React, { useCallback, useEffect, useMemo, useState } from 'react'
import Collapse from '@material-ui/core/Collapse'
import IconButton from '@material-ui/core/IconButton'
import PhotoIcon from '@material-ui/icons/Photo'
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

const defaultFieldStats = {
  alreadyExist: 0,
  missing: 0,
  fetching: 0,
  fetched: 0,
  updated: 0,
  toBeFetch: 0,
  couldntFetch: 0,
}

const statCards = [
  { key: 'album', label: 'Album' },
  { key: 'year', label: 'Year' },
  { key: 'coverArt', label: 'Cover Art' },
]

const StatCard = ({ title, stats = defaultFieldStats }) => (
  <div
    style={{
      border: '1px solid rgba(255,255,255,0.12)',
      borderRadius: 6,
      padding: 16,
      minWidth: 250,
      background: 'rgba(255,255,255,0.02)',
    }}
  >
    <div style={{ fontSize: 24, marginBottom: 8 }}>{title}</div>
    {[
      ['Already exist', stats.alreadyExist],
      ['Missing', stats.missing],
      ['Fetching', stats.fetching],
      ['Fetched', stats.fetched],
      ['Updated', stats.updated],
      ['To be fetch', stats.toBeFetch],
      ["Couldn't fetched", stats.couldntFetch],
    ].map(([label, value]) => (
      <div
        key={label}
        style={{ display: 'flex', justifyContent: 'space-between', lineHeight: 1.8 }}
      >
        <span>{label}</span>
        <span>{value}</span>
      </div>
    ))}
  </div>
)

const CovertartActions = () => {
  const notify = useNotify()
  const refresh = useRefresh()
  const [loading, setLoading] = useState(false)
  const [job, setJob] = useState(null)
  const [showStats, setShowStats] = useState(false)

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
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <Button
            label="Fetch Missing Metadata (Spotify)"
            onClick={handleFetchSpotify}
            disabled={loading}
          />
          <IconButton onClick={() => setShowStats((v) => !v)} title="Toggle Cover Art metadata panel">
            <PhotoIcon />
          </IconButton>
        </div>
        <span style={{ alignSelf: 'center' }}>
          Updated: {progress.updated} / Skipped: {progress.skipped} / Failed:{' '}
          {progress.failed} / Processed: {progress.processed}
        </span>
      </div>

      <Collapse in={showStats} timeout="auto" unmountOnExit>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(250px, 1fr))', gap: 12 }}>
          {statCards.map((card) => (
            <StatCard
              key={card.key}
              title={card.label}
              stats={job?.stats?.[card.key] || defaultFieldStats}
            />
          ))}
        </div>
      </Collapse>
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
