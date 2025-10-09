import { useMemo, useCallback } from 'react'
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
import { playTracks, setTrack } from '../actions'
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

const SongList = (props) => {
  const classes = useStyles()
  const dispatch = useDispatch()
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  const isMobileLayout = useMediaQuery('(max-width:768px)')
  useResourceRefresh('song')

  const songs = useSelector((state) => state.admin.resources.song)

  const normalizedSongsData = useMemo(() => {
    if (!songs?.data) {
      return undefined
    }
    if (Array.isArray(songs.data)) {
      return songs.data.reduce((acc, song) => {
        if (song?.id !== undefined) {
          acc[song.id] = song
        }
        return acc
      }, {})
    }
    return songs.data
  }, [songs?.data])

  const queueIds = useMemo(() => {
    if (Array.isArray(songs?.list?.ids) && songs.list.ids.length) {
      return songs.list.ids
    }
    if (normalizedSongsData) {
      return Object.keys(normalizedSongsData)
    }
    return []
  }, [songs?.list?.ids, normalizedSongsData])

  const findMatchingKey = useCallback(
    (record, fallback) => {
      if (!record) {
        return fallback ?? queueIds[0]
      }
      const recordId = record.id != null ? record.id.toString() : undefined
      const mediaId =
        record.mediaFileId != null ? record.mediaFileId.toString() : undefined
      const matchRecord =
        recordId &&
        queueIds.find((itemId) => itemId?.toString() === recordId)
      if (matchRecord !== undefined) {
        return matchRecord
      }
      const matchMedia =
        mediaId && queueIds.find((itemId) => itemId?.toString() === mediaId)
      if (matchMedia !== undefined) {
        return matchMedia
      }
      return fallback ?? recordId ?? mediaId ?? queueIds[0]
    },
    [queueIds],
  )

  const queuePlayback = useCallback(
    (selectedKey, record) => {
      if (!normalizedSongsData || !queueIds.length) {
        if (record) {
          dispatch(setTrack(record))
        }
        return
      }

      const selectedKeyStr = selectedKey != null ? selectedKey.toString() : undefined
      const matchKey =
        (selectedKeyStr
          ? queueIds.find((itemId) => itemId?.toString() === selectedKeyStr)
          : undefined) ?? findMatchingKey(record, queueIds[0])

      if (matchKey == null) {
        if (record) {
          dispatch(setTrack(record))
        }
        return
      }

      dispatch(playTracks(normalizedSongsData, queueIds, matchKey))
    },
    [dispatch, normalizedSongsData, queueIds, findMatchingKey],
  )

  const handleRowClick = useCallback(
    (id, basePath, record) => {
      queuePlayback(id, record)
    },
    [queuePlayback],
  )

  const handleMobileItemClick = useCallback(
    ({ id: itemId, record }) => {
      queuePlayback(itemId, record)
    },
    [queuePlayback],
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
      rating: config.enableStarRating && (
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

  const columns = useSelectedFields({
    resource: 'song',
    columns: toggleableFields,
    defaultOff: [
      'channels',
      'bpm',
      'playDate',
      'albumArtist',
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
        actions={<SongListActions />}
        filters={<SongFilter />}
        perPage={isXsmall ? 50 : 50}
      >
        {isXsmall ? (
          <SongSimpleList
            onItemClick={isMobileLayout ? handleMobileItemClick : undefined}
            highlightCurrentTrack={isMobileLayout}
          />
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

export default SongList
