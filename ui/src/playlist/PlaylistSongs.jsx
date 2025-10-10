import React, { useCallback, useEffect, useMemo } from 'react'
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
  searchTerm = '',
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
  const listVersion = listContext.version ?? 0
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  const classes = useStyles({ isDesktop })
  const dispatch = useDispatch()
  const dataProvider = useDataProvider()
  const notify = useNotify()
  const version = useVersion()
  useResourceRefresh('song', 'playlist')
  const searchValue = useMemo(
    () => (typeof searchTerm === 'string' ? searchTerm.trim() : ''),
    [searchTerm],
  )
  const fullDataCache = React.useRef({})
  const [fullDataset, setFullDataset] = React.useState({
    playlistId: null,
    search: '',
    ids: [],
    data: {},
    version: null,
    loading: false,
    ready: false,
  })
  const noopSetPage = useCallback(() => {}, [])

  useEffect(() => {
    if (!showDuplicatesOnly || !playlistId) {
      return
    }

    const cacheKey = `${playlistId}::${searchValue}`
    const cached = fullDataCache.current[cacheKey]

    if (cached && cached.version === listVersion) {
      setFullDataset({
        playlistId,
        search: searchValue,
        ids: cached.ids,
        data: cached.data,
        version: cached.version,
        loading: false,
        ready: true,
      })
      return
    }

    let isActive = true

    setFullDataset({
      playlistId,
      search: searchValue,
      ids: [],
      data: {},
      version: listVersion,
      loading: true,
      ready: false,
    })

    dataProvider
      .getList('playlistTrack', {
        pagination: { page: 1, perPage: 0 },
        sort: { field: 'id', order: 'ASC' },
        filter: {
          playlist_id: playlistId,
          ...(searchValue ? { q: searchValue } : {}),
        },
      })
      .then(({ data }) => {
        if (!isActive) {
          return
        }

        const mappedData = data.reduce((acc, track) => {
          acc[track.id] = track
          return acc
        }, {})
        const ids = data.map((track) => track.id)
        const entry = { ids, data: mappedData, version: listVersion }

        fullDataCache.current[cacheKey] = entry

        setFullDataset({
          playlistId,
          search: searchValue,
          ids,
          data: mappedData,
          version: listVersion,
          loading: false,
          ready: true,
        })
      })
      .catch(() => {
        if (!isActive) {
          return
        }

        notify('ra.page.error', 'warning')

        setFullDataset({
          playlistId,
          search: searchValue,
          ids: [],
          data: {},
          version: listVersion,
          loading: false,
          ready: true,
        })
      })

    return () => {
      isActive = false
    }
  }, [
    showDuplicatesOnly,
    playlistId,
    searchValue,
    dataProvider,
    notify,
    listVersion,
  ])

  const usingFullData =
    showDuplicatesOnly &&
    fullDataset.ready &&
    fullDataset.playlistId === playlistId &&
    fullDataset.search === searchValue

  const duplicateIds = useMemo(() => {
    if (!showDuplicatesOnly || !usingFullData) {
      return []
    }

    const normalizeMeta = (value) => {
      if (typeof value !== 'string') {
        return ''
      }
      const normalized = value.trim().toLowerCase()
      if (
        !normalized ||
        normalized === 'unknown' ||
        normalized === 'unknown artist' ||
        normalized === 'unknown artists'
      ) {
        return ''
      }
      return normalized
    }

    const normalizePath = (value) =>
      typeof value === 'string' ? value.trim().toLowerCase() : ''

    const seen = new Map()
    const duplicates = []

    fullDataset.ids.forEach((id) => {
      const track = fullDataset.data[id]
      if (!track) {
        return
      }

      const title = normalizeMeta(track.title)
      const artist = normalizeMeta(track.artist)
      const path = normalizePath(track.path)

      let key = null
      if (title || artist) {
        key = `meta:${title}|${artist}`
      } else if (path) {
        key = `path:${path}`
      }

      if (!key) {
        return
      }

      if (!seen.has(key)) {
        seen.set(key, [])
      }

      const list = seen.get(key)
      list.push(id)
      if (list.length > 1) {
        duplicates.push(id)
      }
    })

    return duplicates
  }, [showDuplicatesOnly, usingFullData, fullDataset.ids, fullDataset.data])

  const ids = useMemo(() => {
    if (!showDuplicatesOnly) {
      return contextIds
    }

    return duplicateIds
  }, [contextIds, duplicateIds, showDuplicatesOnly])

  const data = useMemo(() => {
    if (!showDuplicatesOnly) {
      return contextData
    }

    if (!usingFullData) {
      return {}
    }

    return duplicateIds.reduce((acc, id) => {
      const track = fullDataset.data[id]
      if (track) {
        acc[id] = track
      }
      return acc
    }, {})
  }, [
    contextData,
    duplicateIds,
    fullDataset.data,
    showDuplicatesOnly,
    usingFullData,
  ])

  const displayedIdSet = useMemo(() => new Set(ids), [ids])

  const selectedIds = useMemo(
    () => contextSelectedIds.filter((id) => displayedIdSet.has(id)),
    [contextSelectedIds, displayedIdSet],
  )

  const duplicateLoading =
    showDuplicatesOnly && (!usingFullData || fullDataset.loading)

  const filteredListContext = useMemo(
    () => ({
      ...listContext,
      ids,
      data,
      selectedIds,
      total: showDuplicatesOnly ? ids.length : listContext.total,
      page: showDuplicatesOnly ? 1 : listContext.page,
      perPage: showDuplicatesOnly
        ? ids.length > 0
          ? ids.length
          : listContext.perPage
        : listContext.perPage,
      loading: showDuplicatesOnly ? duplicateLoading : listContext.loading,
      loaded: showDuplicatesOnly ? usingFullData : listContext.loaded,
      setPage: showDuplicatesOnly ? noopSetPage : listContext.setPage,
    }),
    [
      data,
      duplicateLoading,
      ids,
      listContext,
      noopSetPage,
      selectedIds,
      showDuplicatesOnly,
      usingFullData,
    ],
  )

  useEffect(() => {
    setContextPage(1)
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }, [playlistId, setContextPage])

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
            {showDuplicatesOnly && duplicateLoading && <LinearProgress />}
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
