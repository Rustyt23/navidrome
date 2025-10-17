import React, { useState, useCallback } from 'react'
import {
  ReferenceManyField,
  ShowContextProvider,
  useShowContext,
  useShowController,
  SearchInput,
  Filter,
  Pagination,
  Title as RaTitle,
} 
from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import PlaylistDetails from './PlaylistDetails'
import PlaylistSongs from './PlaylistSongs'
import PlaylistActions from './PlaylistActions'
import { Title, canChangeTracks, useResourceRefresh } from '../common'

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

  // Store search query in state to prevent losing focus
  const [searchTerm, setSearchTerm] = useState('')
  const [showDuplicatesOnly, setShowDuplicatesOnly] = useState(false)
  const [sort, setSort] = useState({ field: 'title', order: 'ASC' })

  // Handle search change
  const handleSearchChange = useCallback((eventOrValue) => {
    const value =
      typeof eventOrValue === 'string'
        ? eventOrValue
        : eventOrValue?.target?.value ?? ''

    setSearchTerm(value)
  }, [])

  const handleToggleDuplicates = useCallback(() => {
    setShowDuplicatesOnly((prev) => !prev)
  }, [])

  React.useEffect(() => {
    setShowDuplicatesOnly(false)
    setSort({ field: 'title', order: 'ASC' })
  }, [record?.id])

  const handleSortChange = useCallback((nextSort) => {
    if (!nextSort) {
      return
    }
    setSort(nextSort)
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
              source="q"
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
            sort={sort}
            perPage={50}
            filter={{
              playlist_id: props.id,
              q: searchTerm,
              ...(showDuplicatesOnly ? { duplicatesOnly: true } : {}),
            }} // Pass searchTerm as a filter
          >
            <PlaylistSongs
              {...props}
              readOnly={!canChangeTracks(record)}
              title={<Title subTitle={record.name} />}
              actions={
                <PlaylistActions
                  className={classes.playlistActions}
                  record={record}
                  showDuplicatesOnly={showDuplicatesOnly}
                  onToggleDuplicates={handleToggleDuplicates}
                />
              }
              resource={'playlistTrack'}
              exporter={false}
              pagination={<Pagination rowsPerPageOptions={[50, 100, 200, 500]}
              perPage={50}
                />}
              searchTerm={searchTerm} // Pass search term to child
              showDuplicatesOnly={showDuplicatesOnly}
              sort={sort}
              onSortChange={handleSortChange}
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
