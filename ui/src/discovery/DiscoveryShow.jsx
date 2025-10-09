import React, { useState, useCallback } from 'react'
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
import { Title, useResourceRefresh } from '../common'
import PlaylistDetails from '../playlist/PlaylistDetails'
import DiscoverySongs from './DiscoverySongs'
import DiscoveryActions from './DiscoveryActions'

const DiscoveryShowLayout = (props) => {
  const { loading, ...context } = useShowContext(props)
  const { record } = context
  const [searchTerm, setSearchTerm] = useState('')
  useResourceRefresh('discovery')

  const handleSearchChange = useCallback((eventOrValue) => {
    const value =
      typeof eventOrValue === 'string'
        ? eventOrValue
        : eventOrValue?.target?.value ?? ''

    setSearchTerm(value)
  }, [])

  if (loading) {
    return null
  }

  return (
    <>
      {record && <RaTitle title={<Title subTitle={record.name} />} />}
      {record && <PlaylistDetails {...context} />}
      {record && (
        <>
          <Filter variant="outlined">
            <SearchInput
              id="search"
              source="q"
              alwaysOn
              value={searchTerm}
              onChange={handleSearchChange}
            />
          </Filter>
          <ReferenceManyField
            {...context}
            addLabel={false}
            reference="discoveryTrack"
            target="discovery_id"
            sort={{ field: 'id', order: 'ASC' }}
            perPage={50}
            filter={{ discovery_id: props.id, q: searchTerm }}
          >
            <DiscoverySongs
              discoveryId={record.id}
              searchTerm={searchTerm}
              title={<Title subTitle={record.name} />}
              actions={<DiscoveryActions record={record} />}
              pagination={
                <Pagination rowsPerPageOptions={[25, 50, 100, 200]} perPage={50} />
              }
            />
          </ReferenceManyField>
        </>
      )}
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
