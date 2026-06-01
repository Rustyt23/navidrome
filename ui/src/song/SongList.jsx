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


const getLufsValue = (song) => {
  const tags = song?.tags || {}
  const rawTags = song?.rawTags || {}

  const direct =
    tags.loudnorm_final_lufs?.[0] ??
    tags.final_lufs?.[0] ??
    tags.finallufs?.[0] ??
    tags.lufs?.[0]
  if (direct !== undefined && direct !== null && direct !== '') return direct

  const merged = { ...tags, ...rawTags }
  for (const [key, values] of Object.entries(merged)) {
    const normalized = key.toLowerCase()
    if (
      normalized.includes('loudnorm_final_lufs') ||
      normalized.includes('final_lufs') ||
      normalized.includes('finallufs')
    ) {
      return Array.isArray(values) ? values[0] ?? '' : values ?? ''
    }
  }

  return ''
}

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
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('song')

  const songsState =
    useSelector((state) => state.admin.resources.song) ?? {}
  const songsData = songsState.data
  const listIds = songsState.list?.ids

  const handleRowClick = useCallback(
    (id, basePath, record) => {
      const normalizedSongs = Array.isArray(songsData)
        ? songsData
        : songsData
        ? Object.values(songsData)
        : []

      if (normalizedSongs.length === 0 || !Array.isArray(listIds)) {
        return
      }

      const filteredSongs = normalizedSongs.filter((song) =>
        listIds.includes(song.id),
      )

      if (filteredSongs.length === 0) {
        return
      }

      if (!record?.id) {
        return
      }

      const index = filteredSongs.findIndex((song) => song.id === record.id)

      if (index === -1) {
        return
      }

      const orderedSongs = [
        ...filteredSongs.slice(index),
        ...filteredSongs.slice(0, index),
      ]

      const updatedSongs = Object.fromEntries(
        orderedSongs.map((song, idx) => [idx, song]),
      )

      dispatch(playTracks(updatedSongs, 0))
    },
    [dispatch, songsData, listIds],
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
      loudnessFinalLUFS: (
        <FunctionField
          label="LUFS"
          source="tags.loudnorm_final_lufs"
          render={(r) => getLufsValue(r)}
          sortBy="lufs"
        />
      ),
      genre: <TextField source="genre" sortBy="genre" />,
      mood: isDesktop && (
        <FunctionField
          source="mood"
          render={(r) => r.tags?.mood?.[0] || ''}
          sortable={false}
        />
      ),
      comment: <TextField source="comment" sortBy="comment" />,
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
        perPage={50}
      >
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
      </List>
      <ExpandInfoDialog content={<SongInfo />} />
    </>
  )
}

export default SongList
