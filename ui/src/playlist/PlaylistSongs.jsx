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
import config from '../config'

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
  const classes = useStyles({ isDesktop })
  const dispatch = useDispatch()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const version = useVersion()
  useResourceRefresh('song', 'playlist')

  const prevShowDuplicates = React.useRef(showDuplicatesOnly)

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

  const [loadedRecords, setLoadedRecords] = useState({})

  useEffect(() => {
    setLoadedRecords({})
  }, [playlistId])
  
  const handleSelect = useCallback(
    (idsToSelect) => {
      if (!contextOnSelect) {
        return
      }

      if (!Array.isArray(idsToSelect)) {
        contextOnSelect(idsToSelect)
        return
      }

      const pageIds = ids || []
      const newlyAddedIds = idsToSelect.filter(
        (id) => !selectedIds.includes(id),
      )

      const isSelectingCurrentPage =
        newlyAddedIds.length > 0 &&
        newlyAddedIds.every((id) => pageIds.includes(id))

      const isSelectingEntirePage =
        Array.isArray(pageIds) &&
        pageIds.length > 0 &&
        pageIds.every((id) => idsToSelect.includes(id))

      const shouldLoadAllIds =
        isSelectingCurrentPage &&
        isSelectingEntirePage &&
        typeof contextTotal === 'number' &&
        contextTotal > idsToSelect.length

      if (shouldLoadAllIds) {
        const filter = { ...filterValues, playlist_id: playlistId }
        const sort =
          currentSort && currentSort.field
            ? currentSort
            : { field: 'id', order: 'ASC' }

        dataProvider
          .getList('playlistTrack', {
            filter,
            pagination: {
              page: 1,
              perPage:
                contextTotal && contextTotal > 0
                  ? contextTotal
                  : idsToSelect.length,
            },
            sort: sort,
          })
          .then(({ data: records }) => {
            const recordsById = records.reduce((acc, record) => {
              acc[record.id] = record
              return acc
            }, {})
            setLoadedRecords((prev) => ({ ...prev, ...recordsById }))

            const preservedIds = idsToSelect.filter(
              (id) => !pageIds.includes(id),
            )
            const allIds = records.map((record) => record.id)
            contextOnSelect([...new Set([...preservedIds, ...allIds])])
          })
          .catch(() => {
            contextOnSelect(idsToSelect)
          })

        return
      }

      contextOnSelect(idsToSelect)
    },
    [
      contextOnSelect,
      ids,
      selectedIds,
      contextTotal,
      filterValues,
      playlistId,
      currentSort,
      dataProvider,
    ],
  )

  const filteredListContext = useMemo(
    () => ({
      ...listContext,
      data: { ...contextData, ...loadedRecords },
      selectedIds,
      onSelect: handleSelect,
    }),
    [listContext, selectedIds, handleSelect, contextData, loadedRecords],

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
      dataProvider
        .update('playlistTrack', {
          id,
          data: { insert_before: newPos },
          filter: { playlist_id: playlistId },
        })
        .then(() => {
          refetch()
        })
        .catch(() => {
          notify('ra.page.error', 'warning')
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

  const toggleableFields = useMemo(() => {
    return {
      trackNumber: isDesktop && <TextField source="id" label={'#'} />,
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
        <DateField source="createdAt" showTime sortable={false} />
      ),
      quality: isDesktop && <QualityInfo source="quality" sortable={false} />,
      channels: isDesktop && <NumberField source="channels" />,
      bpm: isDesktop && <NumberField source="bpm" />,
      genre: <TextField source="genre" />,
      comment: <TextField source="comment" />,
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

  // Disable playlist reordering when multiple tracks are selected so the
  // native drag events can be handled by React DnD for multi-track moves.
  const isReorderEnabled = !readOnly && selectedIds.length <= 1

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
            <ReorderableList
              readOnly={!isReorderEnabled}
              onDragEnd={handleDragEnd}
              nodeSelector={'tr'}
              handleSelector={'.draggable'}
            >
              <SongDatagrid
                rowClick={handleRowClick}
                {...filteredListContext}
                hasBulkActions={!readOnly}
                contextAlwaysVisible={!isDesktop}
                classes={{ row: clsx(classes.row, classes.draggable) }}
              >
                {columns}
                <SongContextMenu
                  onAddToPlaylist={onAddToPlaylist}
                  showLove={true}
                  className={classes.contextMenu}
                />
              </SongDatagrid>
            </ReorderableList>
          </Card>
        </div>
      </ListContextProvider>
      <ExpandInfoDialog content={<SongInfo />} />
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
