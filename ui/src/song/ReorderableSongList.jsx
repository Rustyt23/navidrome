import React, {
  useMemo,
  useCallback,
  useEffect,
  useState,
  Children,
} from 'react'
import {
  AutocompleteArrayInput,
  Filter,
  FunctionField,
  NumberField,
  ReferenceArrayInput,
  SearchInput,
  TextField,
  useTranslate,
  NullableBooleanInput,
  usePermissions,
} from 'react-admin'
import { useMediaQuery } from '@material-ui/core'
import FavoriteIcon from '@material-ui/icons/Favorite'
import {
  DateField,
  DurationField,
  List,
  SongContextMenu,
  SongDatagrid,
  SongInfo,
  QuickFilter,
  SongTitleField,
  SongSimpleList,
  RatingField,
  useResourceRefresh,
  ArtistLinkField,
  PathField,
} from '../common'
import { useSelector, useDispatch } from 'react-redux'
import { makeStyles } from '@material-ui/core/styles'
import FavoriteBorderIcon from '@material-ui/icons/FavoriteBorder'
import { playTracks } from '../actions'
import { setColumnsOrder, setOmittedFields, setToggleableFields } from '../actions/settings'
import { SongListActions } from './SongListActions'
import { AlbumLinkField } from './AlbumLinkField'
import { SongBulkActions, QualityInfo } from '../common'
import config from '../config'
import ExpandInfoDialog from '../dialogs/ExpandInfoDialog'

const useStyles = makeStyles({
  contextHeader: {
    marginLeft: '3px',
    marginTop: '-2px',
    verticalAlign: 'text-top',
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
    visibility: 'hidden',
  },
  ratingField: {
    visibility: 'hidden',
  },
  chip: {
    margin: 0,
    height: '24px',
  },
})

const DEFAULT_OFF_COLUMNS = [
  'channels',
  'bpm',
  'playDate',
  'albumArtist',
  'genre',
  'mood',
  'comment',
  'path',
  'createdAt',
]

const INITIAL_OMITTED = []

const arraysEqual = (a = [], b = []) =>
  a.length === b.length && a.every((item, index) => item === b[index])

const sanitizeOrder = (order, keys) => {
  if (!order) return keys
  const filtered = order.filter((key) => keys.includes(key))
  const missing = keys.filter((key) => !filtered.includes(key))
  return [...filtered, ...missing]
}

const SongFilter = (props) => {
  const classes = useStyles()
  const translate = useTranslate()
  const { permissions } = usePermissions()
  const isAdmin = permissions === 'admin'
  return (
    <Filter {...props} variant={'outlined'}>
      <SearchInput source="title" alwaysOn />
      <ReferenceArrayInput
        label={translate('resources.song.fields.genre')}
        source="genre_id"
        reference="genre"
        perPage={0}
        sort={{ field: 'name', order: 'ASC' }}
        filterToQuery={(searchText) => ({ name: [searchText] })}
      >
        <AutocompleteArrayInput emptyText="-- None --" classes={classes} />
      </ReferenceArrayInput>
      <ReferenceArrayInput
        label={translate('resources.song.fields.grouping')}
        source="grouping"
        reference="tag"
        perPage={0}
        sort={{ field: 'tagValue', order: 'ASC' }}
        filter={{ tag_name: 'grouping' }}
        filterToQuery={(searchText) => ({
          tag_value: [searchText],
        })}
      >
        <AutocompleteArrayInput
          emptyText="-- None --"
          classes={classes}
          optionText="tagValue"
        />
      </ReferenceArrayInput>
      <ReferenceArrayInput
        label={translate('resources.song.fields.mood')}
        source="mood"
        reference="tag"
        perPage={0}
        sort={{ field: 'tagValue', order: 'ASC' }}
        filter={{ tag_name: 'mood' }}
        filterToQuery={(searchText) => ({
          tag_value: [searchText],
        })}
      >
        <AutocompleteArrayInput
          emptyText="-- None --"
          classes={classes}
          optionText="tagValue"
        />
      </ReferenceArrayInput>
      {config.enableFavourites && (
        <QuickFilter
          source="starred"
          label={<FavoriteIcon fontSize={'small'} />}
          defaultValue={true}
        />
      )}
      {isAdmin && <NullableBooleanInput source="missing" />}
    </Filter>
  )
}

const ReorderableSongList = (props) => {
  const classes = useStyles()
  const dispatch = useDispatch()
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('song')

  const songs = useSelector((state) => state.admin.resources.song)
  const perPage = Number(songs?.list?.params?.perPage) || 50
  const toggleableColumnsState = useSelector(
    (state) => state.settings.toggleableFields.song,
  )
  const omittedColumnsState = useSelector(
    (state) => state.settings.omittedFields.song,
  )
  const storedColumnsOrder = useSelector(
    (state) => state.settings.columnsOrder.song,
  )

  const handleRowClick = useCallback(
    (id, basePath, record) => {
      const songsArray = Array.isArray(songs.data)
        ? songs.data
        : Object.values(songs.data || {})

      if (songsArray.length > 0 && Array.isArray(songs.list?.ids)) {
        const filteredSongs = songsArray.filter((song) =>
          songs.list.ids.includes(song.id),
        )

        const index = filteredSongs.findIndex((song) => song.id === record.id)

        if (index !== -1) {
          const orderedSongs = [
            ...filteredSongs.slice(index),
            ...filteredSongs.slice(0, index),
          ]

          const updatedSongs = Object.fromEntries(
            orderedSongs.map((song, idx) => [idx, song]),
          )

          dispatch(playTracks(updatedSongs, 0))
        }
      }
    },
    [dispatch, songs.data, songs.list?.ids],
  )

  const songListIds = songs?.list?.ids
  const getRowNumber = useCallback(
    (record) => {
      if (!record) return ''
      if (!Array.isArray(songListIds)) {
        return record.trackNumber ?? ''
      }

      const recordId = record.id
      const index = songListIds.findIndex(
        (id) => String(id) === String(recordId),
      )

      if (index === -1) {
        return record.trackNumber ?? ''
      }

      return index + 1
    },
    [songListIds],
  )

  const toggleableFields = useMemo(() => {
    return {
      title: <SongTitleField source="title" showTrackNumbers={false} />,
      album: isDesktop ? <AlbumLinkField source="album" sortByOrder={'ASC'} /> : null,
      artist: <ArtistLinkField source="artist" />,
      albumArtist: isDesktop ? <ArtistLinkField source="albumArtist" /> : null,
      trackNumber: isDesktop ? (
        <FunctionField
          source="trackNumber"
          sortable={false}
          render={(record) => getRowNumber(record)}
        />
      ) : null,
      playCount: isDesktop ? (
        <NumberField source="playCount" sortByOrder={'DESC'} />
      ) : null,
      playDate: <DateField source="playDate" sortByOrder={'DESC'} showTime />,
      year: isDesktop ? (
        <FunctionField
          source="year"
          render={(r) => r.year || ''}
          sortByOrder={'DESC'}
        />
      ) : null,
      quality: isDesktop ? <QualityInfo source="quality" sortable={false} /> : null,
      channels: isDesktop ? (
        <NumberField source="channels" sortByOrder={'ASC'} />
      ) : null,
      duration: <DurationField source="duration" />,
      rating:
        config.enableStarRating && (
          <RatingField
            source="rating"
            sortByOrder={'DESC'}
            resource={'song'}
            className={classes.ratingField}
          />
        ),
      bpm: isDesktop ? <NumberField source="bpm" /> : null,
      genre: <TextField source="genre" />,
      mood: isDesktop ? (
        <FunctionField
          source="mood"
          render={(r) => r.tags?.mood?.[0] || ''}
          sortable={false}
        />
      ) : null,
      comment: <TextField source="comment" />,
      path: <PathField source="path" />,
      createdAt: <DateField source="createdAt" sortBy="recently_added" showTime />,
    }
  }, [getRowNumber, isDesktop, classes.ratingField])

  const columnKeys = useMemo(() => Object.keys(toggleableFields), [toggleableFields])

  useEffect(() => {
    if (
      !toggleableColumnsState ||
      Object.keys(toggleableColumnsState).length !== columnKeys.length ||
      !columnKeys.every((key) => key in toggleableColumnsState)
    ) {
      const obj = {}
      for (const key of columnKeys) {
        obj[key] = !DEFAULT_OFF_COLUMNS.includes(key)
      }
      dispatch(setToggleableFields({ song: obj }))
    }
  }, [toggleableColumnsState, columnKeys, dispatch])

  useEffect(() => {
    if (!omittedColumnsState) {
      dispatch(setOmittedFields({ song: INITIAL_OMITTED }))
    }
  }, [omittedColumnsState, dispatch])

  useEffect(() => {
    if (
      !storedColumnsOrder ||
      storedColumnsOrder.length !== columnKeys.length ||
      !columnKeys.every((key) => storedColumnsOrder.includes(key))
    ) {
      dispatch(setColumnsOrder({ song: columnKeys }))
    }
  }, [storedColumnsOrder, columnKeys, dispatch])

  const [columnOrder, setColumnOrder] = useState(() =>
    sanitizeOrder(storedColumnsOrder, columnKeys),
  )

  useEffect(() => {
    const nextOrder = sanitizeOrder(storedColumnsOrder, columnKeys)
    if (!arraysEqual(columnOrder, nextOrder)) {
      setColumnOrder(nextOrder)
    }
  }, [storedColumnsOrder, columnKeys, columnOrder])

  const { visibleColumns, computedOmitted } = useMemo(() => {
    const omitted = [...INITIAL_OMITTED]
    const visible = []
    for (const key of columnOrder) {
      const column = toggleableFields[key]
      if (!column) {
        omitted.push(key)
        continue
      }
      const isVisible = toggleableColumnsState
        ? toggleableColumnsState[key]
        : !DEFAULT_OFF_COLUMNS.includes(key)
      if (isVisible) {
        visible.push(column)
      }
    }
    return { visibleColumns: Children.toArray(visible), computedOmitted: omitted }
  }, [columnOrder, toggleableFields, toggleableColumnsState])

  useEffect(() => {
    const current = omittedColumnsState || []
    if (
      current.length !== computedOmitted.length ||
      computedOmitted.some((field, index) => field !== current[index])
    ) {
      dispatch(setOmittedFields({ song: computedOmitted }))
    }
  }, [computedOmitted, omittedColumnsState, dispatch])

  return (
    <>
      <List
        {...props}
        sort={{ field: 'title', order: 'ASC' }}
        exporter={false}
        bulkActionButtons={<SongBulkActions />}
        actions={<SongListActions />}
        filters={<SongFilter />}
        perPage={perPage}
      >
        {isXsmall ? (
          <SongSimpleList />
        ) : (
          <SongDatagrid
            rowClick={handleRowClick}
            contextAlwaysVisible={!isDesktop}
            classes={{ row: classes.row }}
          >
            {visibleColumns}
            <SongContextMenu
              source={'starred_at'}
              sortByOrder={'DESC'}
              sortable={config.enableFavourites}
              className={classes.contextMenu}
              label={
                config.enableFavourites && (
                  <FavoriteBorderIcon
                    fontSize={'small'}
                    className={classes.contextHeader}
                  />
                )
              }
            />
          </SongDatagrid>
        )}
      </List>
      <ExpandInfoDialog content={<SongInfo />} />
    </>
  )
}

export default ReorderableSongList
