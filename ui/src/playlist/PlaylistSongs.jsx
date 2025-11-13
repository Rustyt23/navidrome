import React, { useCallback, useEffect, useMemo, useState } from 'react'
import {
  BulkActionsToolbar,
  ListToolbar,
  TextField,
  NumberField,
  useDataProvider,
  useNotify,
  useVersion,
  useListContext,
  FunctionField,
  ListContextProvider,
} from 'react-admin'
import clsx from 'clsx'
import { useDispatch } from 'react-redux'
import { Card, useMediaQuery, LinearProgress } from '@material-ui/core'
import { makeStyles } from '@material-ui/core/styles'
import ReactDragListView from 'react-drag-listview'
import {
  DurationField,
  SongInfo,
  SongContextMenu,
  SongDatagrid,
  SongTitleField,
  SongSimpleList,
  QualityInfo,
  useSelectedFields,
  useResourceRefresh,
  DateField,
  ArtistLinkField,
  PathField,
  RatingField,
} from '../common'
import { AlbumLinkField } from '../song/AlbumLinkField'
import { playTracks } from '../actions'
import PlaylistSongBulkActions from './PlaylistSongBulkActions'
import ExpandInfoDialog from '../dialogs/ExpandInfoDialog'
import PlaylistTrackPositionDialog from './PlaylistTrackPositionDialog'
import config from '../config'

/* eslint-disable-next-line react-refresh/only-export-components */
export const selectPlaylistTrackIds = ({
  idsToSelect,
  pageIds = [],
  selectedIds = [],
  contextTotal,
  filterValues = {},
  playlistId,
  currentSort,
  dataProvider,
  onSelect,
}) => {
  if (!onSelect) {
    return Promise.resolve()
  }

  if (!Array.isArray(idsToSelect)) {
    onSelect(idsToSelect)
    return Promise.resolve()
  }

  const newlyAddedIds = idsToSelect.filter((id) => !selectedIds.includes(id))

  const isSelectingEntirePage =
    pageIds.length > 0 && pageIds.every((id) => idsToSelect.includes(id))

  const shouldLoadAllIds =
    isSelectingEntirePage &&
    newlyAddedIds.length > 0 &&
    typeof contextTotal === 'number' &&
    contextTotal > idsToSelect.length

  if (!shouldLoadAllIds) {
    onSelect(idsToSelect)
    return Promise.resolve()
  }

  const filter = { ...filterValues, playlist_id: playlistId }
  const sort =
    currentSort && currentSort.field ? currentSort : { field: 'id', order: 'ASC' }

  return dataProvider
    .getList('playlistTrack', {
      filter,
      pagination: {
        page: 1,
        // perPage: 0 tells the API to return the full playlist regardless of the
        // pagination limit so that the "Select all" checkbox truly selects
        // every track, not only the ones visible on the current page.
        perPage: 0,
      },
      sort: sort,
    })
    .then(({ data: records }) => {
      const preservedIds = idsToSelect.filter((id) => !pageIds.includes(id))
      const playableIds = records
        .filter((record) => !record?.missing)
        .map((record) => record.id)
      onSelect([...new Set([...preservedIds, ...playableIds])])
    })
    .catch(() => {
      onSelect(idsToSelect)
    })
}

const useStyles = makeStyles(
  (theme) => ({
    root: {},
    main: {
      display: 'flex',
    },
    content: {
      marginTop: 0,
      transition: theme.transitions.create('margin-top'),
      position: 'relative',
      flex: '1 1 auto',
      [theme.breakpoints.down('xs')]: {
        boxShadow: 'none',
      },
    },
    bulkActionsDisplayed: {
      marginTop: -theme.spacing(8),
      transition: theme.transitions.create('margin-top'),
    },
    actions: {
      zIndex: 2,
      display: 'flex',
      justifyContent: 'flex-end',
      flexWrap: 'wrap',
    },
    noResults: { padding: 20 },
    toolbar: {
      justifyContent: 'flex-start',
    },
    row: {
      '&:hover': {
        '& $contextMenu': {
          visibility: 'visible',
        },
        '& $ratingField': {
          visibility: 'visible',
        },
      },
    },
    contextMenu: {
      visibility: (props) => (props.isDesktop ? 'hidden' : 'visible'),
    },
    ratingField: {
      visibility: 'hidden',
    },
    draggable: {
      cursor: 'move',
    },
  }),
  { name: 'RaList' },
)

const ReorderableList = ({ readOnly, children, ...rest }) => {
  if (readOnly) {
    return children
  }
  return <ReactDragListView {...rest}>{children}</ReactDragListView>
}

const PlaylistSongs = ({
  playlistId,
  readOnly,
  actions,
  showDuplicatesOnly,
  ...props
}) => {
  const listContext = useListContext()
  const {
    data: contextData = {},
    ids: contextIds = [],
    selectedIds: contextSelectedIds = [],
    onUnselectItems,
    refetch,
    setPage: setContextPage,
  } = listContext
  const ids = contextIds
  const data = contextData
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  const isMobile = useMediaQuery('(max-width:768px)')
  const classes = useStyles({ isDesktop })
  const dispatch = useDispatch()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const version = useVersion()
  useResourceRefresh('song', 'playlist')

  const prevShowDuplicates = React.useRef(showDuplicatesOnly)
  const [positionDialogOpen, setPositionDialogOpen] = useState(false)
  const [positionDialogTrack, setPositionDialogTrack] = useState(null)

  useEffect(() => {
    if (prevShowDuplicates.current !== showDuplicatesOnly) {
      refetch()
    }
    prevShowDuplicates.current = showDuplicatesOnly
  }, [showDuplicatesOnly, refetch])

  useEffect(() => {
    setContextPage(1)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [playlistId, showDuplicatesOnly, setContextPage])

  const selectedIds = contextSelectedIds

  const {
    onSelect: contextOnSelect,
    filterValues = {},
    currentSort,
    total: contextTotal,
  } = listContext

  const handleSelect = useCallback(
    (idsToSelect) =>
      selectPlaylistTrackIds({
        idsToSelect,
        pageIds: ids,
        selectedIds,
        contextTotal,
        filterValues,
        playlistId,
        currentSort,
        dataProvider,
        onSelect: contextOnSelect,
      }),
    [
      ids,
      selectedIds,
      contextTotal,
      filterValues,
      playlistId,
      currentSort,
      dataProvider,
      contextOnSelect,
    ],
  )

  const filteredListContext = useMemo(
    () => ({
      ...listContext,
      selectedIds,
      onSelect: handleSelect,
    }),
    [listContext, selectedIds, handleSelect],
  )

  const onAddToPlaylist = useCallback(
    (pls) => {
      if (pls.id === playlistId) {
        refetch()
      }
    },
    [playlistId, refetch],
  )

  const reorder = useCallback(
    (playlistId, id, newPos) => {
      if (newPos == null) {
        return Promise.resolve(false)
      }

      return dataProvider
        .update('playlistTrack', {
          id,
          data: { insert_before: `${newPos}` },
          filter: { playlist_id: playlistId },
        })
        .then(() => {
          refetch()
          return true
        })
        .catch(() => {
          notify('ra.page.error', 'warning')
          return false
        })
    },
    [dataProvider, notify, refetch],
  )

  const handleDragEnd = useCallback(
    (from, to) => {
      const toId = ids[to]
      const fromId = ids[from]
      reorder(playlistId, fromId, toId)
    },
    [playlistId, reorder, ids],
  )

  const handleRequestPositionChange = useCallback((track) => {
    setPositionDialogTrack(track)
    setPositionDialogOpen(true)
  }, [])

  const handleClosePositionDialog = useCallback(() => {
    setPositionDialogOpen(false)
    setPositionDialogTrack(null)
  }, [])

  const handlePositionSubmit = useCallback(
    (track, newPosition) => {
      if (!track) {
        return
      }

      reorder(playlistId, track.id, newPosition).then((success) => {
        if (success) {
          handleClosePositionDialog()
        }
      })
    },
    [playlistId, reorder, handleClosePositionDialog],
  )

  const contextMenuProps = useMemo(() => {
    const baseProps = {
      onAddToPlaylist,
      showLove: true,
      className: classes.contextMenu,
    }

    if (readOnly) {
      return baseProps
    }

    return {
      ...baseProps,
      onRequestPositionChange: handleRequestPositionChange,
    }
  }, [onAddToPlaylist, classes.contextMenu, readOnly, handleRequestPositionChange])

  const toggleableFields = useMemo(() => {
    return {
      trackNumber:
        isDesktop && (
          <FunctionField
            source="id"
            label={'#'}
            sortBy={'id'}
            render={(record) => {
              const value = record?.id
              if (value == null) {
                return ''
              }
              if (typeof value === 'string') {
                return value.replace(/^_+/, '')
              }
              return value
            }}
          />
        ),
      title: <SongTitleField source="title" showTrackNumbers={false} />,
      album: isDesktop && <AlbumLinkField source="album" />,
      artist: isDesktop && <ArtistLinkField source="artist" />,
      albumArtist: isDesktop && <ArtistLinkField source="albumArtist" />,
      duration: (
        <DurationField source="duration" className={classes.draggable} />
      ),
      year: isDesktop && (
        <FunctionField
          source="year"
          render={(r) => r.year || ''}
          sortByOrder={'DESC'}
        />
      ),
      playCount: isDesktop && (
        <NumberField source="playCount" sortByOrder={'DESC'} />
      ),
      playDate: isDesktop && (
        <DateField source="playDate" sortByOrder={'DESC'} showTime />
      ),
      createdAt: (
        <DateField
          source="createdAt"
          sortBy="playlist_tracks.created_at"
          showTime
        />
      ),
      quality: isDesktop && <QualityInfo source="quality" sortable={false} />,
      channels: isDesktop && <NumberField source="channels" />,
      bpm: isDesktop && <NumberField source="bpm" />,
      genre: <TextField source="genre" sortBy="genre" />,
      comment: <TextField source="comment" sortBy="comment" />,
      path: <PathField source="path" />,
      rating: config.enableStarRating && (
        <RatingField
          source="rating"
          sortByOrder={'DESC'}
          resource={'song'}
          className={classes.ratingField}
        />
      ),
    }
  }, [isDesktop, classes.draggable, classes.ratingField])

  const columns = useSelectedFields({
    resource: 'playlistTrack',
    columns: toggleableFields,
    defaultOff: [
      'channels',
      'bpm',
      'year',
      'playCount',
      'comment',
      'playDate',
      'createdAt',
      'albumArtist',
      'rating',
    ],
  })

  const handleRowClick = useCallback(
    (id) => {
      if (!ids || ids.length === 0) {
        dispatch(playTracks(data, ids, id))
        return
      }

      const startIndex = ids.indexOf(id)
      if (startIndex === -1) {
        dispatch(playTracks(data, ids, id))
        return
      }

      const orderedIds = [
        ...ids.slice(startIndex),
        ...ids.slice(0, startIndex),
      ]

      dispatch(playTracks(data, orderedIds, id))
    },
    [dispatch, data, ids],
  )

  return (
    <>
      <ListContextProvider value={filteredListContext}>
        <ListToolbar
          classes={{ toolbar: classes.toolbar }}
          filters={props.filters}
          actions={actions}
        />
        <div className={classes.main}>
          <Card
            className={clsx(classes.content, {
              [classes.bulkActionsDisplayed]: selectedIds.length > 0,
            })}
            key={version}
          >
            <BulkActionsToolbar>
              <PlaylistSongBulkActions
                playlistId={playlistId}
                onUnselectItems={onUnselectItems}
                readOnly={readOnly}
              />
            </BulkActionsToolbar>
            {showDuplicatesOnly && listContext.loading && <LinearProgress />}
            {isMobile ? (
              <SongSimpleList
                {...filteredListContext}
                hasBulkActions={!readOnly}
                selectedIds={selectedIds}
                contextMenuProps={contextMenuProps}
              />
            ) : (
              <ReorderableList
                readOnly={readOnly}
                onDragEnd={handleDragEnd}
                nodeSelector={'tr'}
                handleSelector={'.draggable'}
              >
                <SongDatagrid
                  rowClick={handleRowClick}
                  {...filteredListContext}
                  hasBulkActions={!readOnly}
                  contextAlwaysVisible={!isDesktop}
                  classes={{ row: classes.row }}
                >
                  {columns}
                  <SongContextMenu
                    {...contextMenuProps}
                  />
                </SongDatagrid>
              </ReorderableList>
            )}
          </Card>
        </div>
      </ListContextProvider>
      <ExpandInfoDialog content={<SongInfo />} />
      <PlaylistTrackPositionDialog
        open={positionDialogOpen && !readOnly}
        track={positionDialogTrack}
        maxPosition={Math.max(ids.length, 1)}
        onCancel={handleClosePositionDialog}
        onSubmit={handlePositionSubmit}
      />
      {React.cloneElement(props.pagination, listContext)}
    </>
  )
}

const SanitizedPlaylistSongs = (props) => {
  const { loaded, ...rest } = props
  return (
    <>
      {loaded && (
        <PlaylistSongs
          playlistId={props.id}
          actions={props.actions}
          pagination={props.pagination}
          {...rest}
        />
      )}
    </>
  )
}

export default SanitizedPlaylistSongs
