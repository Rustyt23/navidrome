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
  useListContext,
  useNotify,
  useRefresh,
} from 'react-admin'
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
}))

const CoverArtFilter = (props) => (
  <Filter {...props} variant={'outlined'}>
    <SearchInput source="q" alwaysOn />
  </Filter>
)

const RefreshMissingCoverArtButton = () => {
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const refresh = useRefresh()
  const { filterValues = {} } = useListContext()

  const handleClick = async () => {
    try {
      await dataProvider.getList('coverart', {
        pagination: { page: 1, perPage: 50 },
        sort: { field: 'title', order: 'ASC' },
        filter: { ...filterValues, refreshMissingCoverArt: true },
      })
      notify('Refreshing missing cover art in background', { type: 'info' })
      refresh()
    } catch (error) {
      notify('Failed to refresh missing cover art', { type: 'warning' })
    }
  }

  return (
    <Button onClick={handleClick} startIcon={<RefreshIcon />}>
      Refresh Missing Cover Art
    </Button>
  )
}

const CoverArtActions = ({ className, filters, resource, showFilter, displayedFilters, filterValues, ...rest }) => (
  <TopToolbar className={className} {...sanitizeListRestProps(rest)}>
    {filters &&
      React.cloneElement(filters, {
        resource,
        showFilter,
        displayedFilters,
        filterValues,
        context: 'button',
      })}
    <RefreshMissingCoverArtButton />
  </TopToolbar>
)

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

const CoverArtPage = (props) => {
  return (
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
}

export default CoverArtPage
