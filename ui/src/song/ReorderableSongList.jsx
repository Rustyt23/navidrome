import { useMemo, useCallback, useEffect, useState } from 'react'
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
import { SongListActions } from './SongListActions'
import { AlbumLinkField } from './AlbumLinkField'
import { SongBulkActions, QualityInfo, useSelectedFields } from '../common'
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

const ensureOrderHasKeys = (order, keys) => {
  const filtered = order.filter((key) => keys.includes(key))
  const missing = keys.filter((key) => !filtered.includes(key))
  return [...filtered, ...missing]
}

const ReorderableSongList = (props) => {
  const classes = useStyles()
  const dispatch = useDispatch()
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('song')

  const songs = useSelector((state) => state.admin.resources.song)

  const handleRowClick = useCallback(
    (id, basePath, record) => {
      const songsArray = Array.isArray(songs.data)
        ? songs.data
        : Object.values(songs.data)

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

  const toggleableFields = useMemo(() => {
    return {
      album: isDesktop && <AlbumLinkField source="album" sortByOrder={'ASC'} />,
      artist: <ArtistLinkField source="artist" />,
      albumArtist: <ArtistLinkField source="albumArtist" />,
      trackNumber: isDesktop && <NumberField source="trackNumber" />,
      playCount: isDesktop && (
        <NumberField source="playCount" sortByOrder={'DESC'} />
      ),
      playDate: <DateField source="playDate" sortByOrder={'DESC'} showTime />,
      year: isDesktop && (
        <FunctionField
          source="year"
          render={(r) => r.year || ''}
          sortByOrder={'DESC'}
        />
      ),
      quality: isDesktop && <QualityInfo source="quality" sortable={false} />,
      channels: isDesktop && (
        <NumberField source="channels" sortByOrder={'ASC'} />
      ),
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
      bpm: isDesktop && <NumberField source="bpm" />,
      genre: <TextField source="genre" />,
      mood: isDesktop && (
        <FunctionField
          source="mood"
          render={(r) => r.tags?.mood?.[0] || ''}
          sortable={false}
        />
      ),
      comment: <TextField source="comment" />,
      path: <PathField source="path" />,
      createdAt: (
        <DateField source="createdAt" sortBy="recently_added" showTime />
      ),
    }
  }, [isDesktop, classes.ratingField])

  const [columnOrder, setColumnOrder] = useState(() =>
    Object.keys(toggleableFields),
  )

  useEffect(() => {
    const keys = Object.keys(toggleableFields)
    setColumnOrder((prev) => {
      if (!prev.length) return keys
      const next = ensureOrderHasKeys(prev, keys)
      if (next.length !== prev.length || prev.some((k, i) => k !== next[i])) {
        return next
      }
      return prev
    })
  }, [toggleableFields])

  const orderedColumns = useMemo(() => {
    const keys = columnOrder.length
      ? columnOrder
      : Object.keys(toggleableFields)
    const ordered = {}
    for (const key of keys) {
      if (key in toggleableFields) {
        ordered[key] = toggleableFields[key]
      }
    }
    for (const key of Object.keys(toggleableFields)) {
      if (!(key in ordered)) {
        ordered[key] = toggleableFields[key]
      }
    }
    return ordered
  }, [columnOrder, toggleableFields])

  const handleColumnOrderChange = useCallback(
    (order) => {
      const keys = Object.keys(toggleableFields)
      setColumnOrder((prev) => {
        const next = ensureOrderHasKeys(order, keys)
        if (prev.length === next.length && prev.every((k, i) => k === next[i])) {
          return prev
        }
        return next
      })
    },
    [toggleableFields],
  )

  const columns = useSelectedFields({
    resource: 'song',
    columns: orderedColumns,
    defaultOff: [
      'channels',
      'bpm',
      'playDate',
      'albumArtist',
      'genre',
      'mood',
      'comment',
      'path',
      'createdAt',
    ],
  })

  return (
    <>
      <List
        {...props}
        sort={{ field: 'title', order: 'ASC' }}
        exporter={false}
        bulkActionButtons={<SongBulkActions />}
        actions={
          <SongListActions
            columnsOrder={columnOrder}
            onColumnsOrderChange={handleColumnOrderChange}
          />
        }
        filters={<SongFilter />}
        perPage={isXsmall ? 50 : 15}
      >
        {isXsmall ? (
          <SongSimpleList />
        ) : (
          <SongDatagrid
            rowClick={handleRowClick}
            contextAlwaysVisible={!isDesktop}
            classes={{ row: classes.row }}
          >
            <SongTitleField source="title" showTrackNumbers={false} />
            {columns}
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
