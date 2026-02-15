import React from 'react'
import {
  Datagrid,
  Filter,
  FunctionField,
  SearchInput,
  TextField,
  TopToolbar,
  sanitizeListRestProps,
  useDataProvider,
  useNotify,
  useRefresh,
} from 'react-admin'
import { Box, Chip } from '@material-ui/core'
import Button from '@material-ui/core/Button'
import RefreshIcon from '@material-ui/icons/Refresh'
import { makeStyles } from '@material-ui/core/styles'
import { DurationField, List } from '../common'

const useStyles = makeStyles(() => ({
  cover: {
    width: 80,
    height: 80,
    objectFit: 'cover',
    borderRadius: 4,
    backgroundColor: '#f4f4f4',
  },
  statusRow: {
    display: 'flex',
    flexWrap: 'wrap',
    gap: 8,
    alignItems: 'center',
    marginLeft: 8,
  },
}))

const CoverArtFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="q" alwaysOn />
  </Filter>
)

const defaultStatus = {
  running: false,
  total: 0,
  completed: 0,
  remaining: 0,
  updated: 0,
  notFound: 0,
  missingCoverSongs: 0,
  missingMetadataSongs: 0,
}

const CoverArtActions = ({ className, filters, resource, showFilter, displayedFilters, filterValues, ...rest }) => {
  const classes = useStyles()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const refresh = useRefresh()
  const [status, setStatus] = React.useState(defaultStatus)

  const loadStatus = React.useCallback(async () => {
    try {
      const response = await dataProvider.getCoverArtStatus()
      setStatus({ ...defaultStatus, ...(response?.data || {}) })
    } catch (_error) {
      // ignore polling failures
    }
  }, [dataProvider])

  React.useEffect(() => {
    loadStatus()
    const interval = setInterval(loadStatus, 2000)
    return () => clearInterval(interval)
  }, [loadStatus])

  const handleRefresh = async () => {
    try {
      const response = await dataProvider.refreshCoverArtMissing({
        filter: { ...filterValues },
      })
      setStatus({ ...defaultStatus, ...(response?.data || {}) })
      notify('Sequential refresh started (up to 50 songs)', { type: 'info' })
      refresh()
      loadStatus()
    } catch (_error) {
      notify('Failed to start refresh', { type: 'warning' })
    }
  }

  return (
    <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
      {filters &&
        React.cloneElement(filters, {
          resource,
          showFilter,
          displayedFilters,
          filterValues,
          context: 'button',
        })}
      <Button onClick={handleRefresh} startIcon={<RefreshIcon />}>
        Refresh Missing Cover Art
      </Button>
      <Box className={classes.statusRow}>
        <Chip label={`Done: ${status.completed}/${status.total}`} size="small" />
        <Chip label={`Left: ${status.remaining}`} size="small" />
        <Chip label={`Not Found: ${status.notFound}`} size="small" />
        <Chip label={`Updated: ${status.updated}`} size="small" />
        <Chip label={`Missing Covers: ${status.missingCoverSongs}`} size="small" />
        <Chip label={`Missing Metadata: ${status.missingMetadataSongs}`} size="small" />
        {status.running && <Chip label="Refreshing..." color="secondary" size="small" />}
      </Box>
    </TopToolbar>
  )
}

const CoverArtImageField = ({ record }) => {
  const classes = useStyles()
  const [src, setSrc] = React.useState(record?.coverArtUrl || '/musicmatters.png')

  React.useEffect(() => {
    setSrc(record?.coverArtUrl || '/musicmatters.png')
  }, [record?.coverArtUrl])

  return (
    <img
      src={src}
      alt={record?.title || 'cover'}
      width={80}
      height={80}
      loading="lazy"
      className={classes.cover}
      onError={() => setSrc('/musicmatters.png')}
    />
  )
}

const CoverArtPage = (props) => (
  <List
    {...props}
    sort={{ field: 'title', order: 'ASC' }}
    filters={<CoverArtFilter />}
    actions={<CoverArtActions />}
    perPage={50}
    exporter={false}
  >
    <Datagrid rowClick={false}>
      <FunctionField
        label="Cover Art"
        source="coverArtUrl"
        render={(record) => <CoverArtImageField record={record} />}
        sortable={false}
      />
      <TextField source="title" label="Title" />
      <TextField source="artist" label="Artist Name" />
      <TextField source="album" label="Album" />
      <TextField source="genre" label="Genre" />
      <TextField source="year" label="Year of Release" />
      <DurationField source="duration" label="Duration" />
    </Datagrid>
  </List>
)

export default CoverArtPage
