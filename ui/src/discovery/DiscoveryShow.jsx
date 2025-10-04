import React, { useCallback, useMemo, useState } from 'react'
import {
  Filter,
  Pagination,
  ReferenceManyField,
  SearchInput,
  ShowContextProvider,
  Title as RaTitle,
  useShowContext,
  useShowController,
} from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import DiscoveryDetails from './DiscoveryDetails'
import DiscoverySongs from './DiscoverySongs'
import DiscoveryActions from './DiscoveryActions'
import { Title, useResourceRefresh } from '../common'

const useStyles = makeStyles(
  () => ({
    discoveryActions: {
      width: '100%',
    },
  }),
  { name: 'NDDiscoveryShow' },
)

const DiscoveryFilter = ({ value, onChange }) => (
  <Filter variant="outlined">
    <SearchInput
      id="search"
      source="q"
      alwaysOn
      value={value}
      onChange={onChange}
    />
  </Filter>
)

const DiscoveryShowLayout = (props) => {
  const context = useShowContext(props)
  const { record } = context
  const classes = useStyles()
  useResourceRefresh('song')
  const [searchTerm, setSearchTerm] = useState('')

  const handleSearchChange = useCallback((event) => {
    setSearchTerm(event.target.value)
  }, [])

  const filters = useMemo(
    () => (
      <DiscoveryFilter value={searchTerm} onChange={handleSearchChange} />
    ),
    [handleSearchChange, searchTerm],
  )

  if (!record) {
    return null
  }

  return (
    <>
      {record && <RaTitle title={<Title subTitle={record.name} />} />}
      <DiscoveryDetails {...context} />
      {filters}
      <ReferenceManyField
        {...context}
        addLabel={false}
        reference="discoveryTrack"
        target="discovery_id"
        sort={{ field: 'position', order: 'ASC' }}
        perPage={50}
        filter={{ discovery_id: record.id, q: searchTerm }}
      >
        <DiscoverySongs
          actions={
            <DiscoveryActions
              className={classes.discoveryActions}
              record={record}
            />
          }
          filters={filters}
          discoveryId={record.id}
          pagination={
            <Pagination
              rowsPerPageOptions={[25, 50, 100, 200]}
              perPage={50}
            />
          }
        />
      </ReferenceManyField>
    </>
  )
}

const DiscoveryShow = (props) => {
  const controllerProps = useShowController(props)
  return (
    <ShowContextProvider value={controllerProps}>
      <DiscoveryShowLayout {...props} {...controllerProps} />
    </ShowContextProvider>
  )
}

export default DiscoveryShow
