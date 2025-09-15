import React, { useState, useCallback, useMemo, useEffect } from 'react'
import {
  ReferenceManyField,
  ShowContextProvider,
  useShowContext,
  useShowController,
  SearchInput,
  Filter,
  Pagination,
  Title as RaTitle,
} from 'react-admin'
import { makeStyles } from '@material-ui/core/styles'
import { useMediaQuery } from '@material-ui/core'
import PlaylistDetails from './PlaylistDetails'
import PlaylistSongs from './PlaylistSongs'
import PlaylistActions from './PlaylistActions'
import { Title, canChangeTracks, useResourceRefresh } from '../common'
import PlaylistMobilePagination from './PlaylistMobilePagination'
import {
  DEFAULT_PLAYLIST_PER_PAGE,
  PLAYLIST_DESKTOP_ROWS_PER_PAGE_OPTIONS,
  PLAYLIST_MOBILE_ROWS_PER_PAGE_OPTIONS,
  PLAYLIST_PER_PAGE_STORAGE_KEY,
} from './playlistPaginationConfig'
import { usePerPagePreference } from '../common/hooks/usePerPagePreference'

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
  const isSmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const { perPage: storedPerPage, setPerPage: setStoredPerPage } =
    usePerPagePreference(
      PLAYLIST_PER_PAGE_STORAGE_KEY,
      DEFAULT_PLAYLIST_PER_PAGE,
    )

  const perPageOptions = useMemo(
    () =>
      isSmall
        ? PLAYLIST_MOBILE_ROWS_PER_PAGE_OPTIONS
        : PLAYLIST_DESKTOP_ROWS_PER_PAGE_OPTIONS,
    [isSmall],
  )

  const effectivePerPage = useMemo(() => {
    if (perPageOptions.includes(storedPerPage)) {
      return storedPerPage
    }
    return DEFAULT_PLAYLIST_PER_PAGE
  }, [perPageOptions, storedPerPage])

  useEffect(() => {
    if (effectivePerPage !== storedPerPage) {
      setStoredPerPage(effectivePerPage)
    }
  }, [effectivePerPage, storedPerPage, setStoredPerPage])

  const paginationComponent = useMemo(() => {
    if (isSmall) {
      return (
        <PlaylistMobilePagination
          rowsPerPageOptions={PLAYLIST_MOBILE_ROWS_PER_PAGE_OPTIONS}
        />
      )
    }
    return (
      <Pagination rowsPerPageOptions={PLAYLIST_DESKTOP_ROWS_PER_PAGE_OPTIONS} />
    )
  }, [isSmall])

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
            perPage={effectivePerPage}
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
              pagination={paginationComponent}
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
