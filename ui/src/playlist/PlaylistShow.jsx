import React, { useMemo, useState, useCallback } from 'react'
import {
  ReferenceManyField,
  ShowContextProvider,
  useShowContext,
  useShowController,
  SearchInput,
  Filter,
  Title as RaTitle,
} from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import PlaylistDetails from './PlaylistDetails'
import PlaylistSongs from './PlaylistSongs'
import PlaylistActions from './PlaylistActions'
import {
  Pagination,
  Title,
  canChangeTracks,
  getStoredPageSize,
  useResourceRefresh,
} from '../common'

const useStyles = makeStyles(
  (theme) => ({
    playlistActions: {
      width: '100%',
    },
  }),
  {
    name: 'NDPlaylistShow',
  },
)

const PlaylistShowLayout = (props) => {
  const { loading, ...context } = useShowContext(props)
  const { record } = context
  const classes = useStyles()
  useResourceRefresh('song')

  const perPage = useMemo(() => getStoredPageSize('playlistTracks'), [])

  // Store search query in state to prevent losing focus
  const [searchTerm, setSearchTerm] = useState('')

  // Handle search change
  const handleSearchChange = useCallback((event) => {
    setSearchTerm(event.target.value)
  }, [])

  return (
    <>
      {record && <RaTitle title={<Title subTitle={record.name} />} />}
      {record && <PlaylistDetails {...context} />}
      {record && (
        <>
          {/* Pass search state and handler to Filter */}
          <Filter variant="outlined">
            <SearchInput
              id="search"
              source="title"
              alwaysOn
              value={searchTerm}
              onChange={handleSearchChange} // Update parent state on change
            />
          </Filter>

          <ReferenceManyField
            {...context}
            addLabel={false}
            reference="playlistTrack"
            target="playlist_id"
            sort={{ field: 'id', order: 'ASC' }}
            perPage={perPage}
            filter={{ playlist_id: props.id, title: searchTerm }} // Pass searchTerm as a filter
          >
            <PlaylistSongs
              {...props}
              readOnly={!canChangeTracks(record)}
              title={<Title subTitle={record.name} />}
              actions={
                <PlaylistActions
                  className={classes.playlistActions}
                  record={record}
                />
              }
              resource={'playlistTrack'}
              exporter={false}
              pagination={<Pagination scope="playlistTracks" />}
              searchTerm={searchTerm} // Pass search term to child
            />
          </ReferenceManyField>
        </>
      )}
    </>
  )
}

const PlaylistShow = (props) => {
  const controllerProps = useShowController(props)
  return (
    <ShowContextProvider value={controllerProps}>
      <PlaylistShowLayout {...props} {...controllerProps} />
    </ShowContextProvider>
  )
}

export default PlaylistShow
