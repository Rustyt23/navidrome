import React, {
  cloneElement,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
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
  sanitizeListRestProps,
  TopToolbar,
} from 'react-admin'
import { useMediaQuery, IconButton, Menu, MenuItem, Checkbox, Typography } from '@material-ui/core'
import FavoriteIcon from '@material-ui/icons/Favorite'
import FavoriteBorderIcon from '@material-ui/icons/FavoriteBorder'
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'
import MoreVertIcon from '@material-ui/icons/MoreVert'
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
  SongBulkActions,
  QualityInfo,
  useSelectedFields,
  ShuffleAllButton,
} from '../common'
import { useSelector, useDispatch } from 'react-redux'
import { makeStyles } from '@material-ui/core/styles'
import { useDrag, useDrop } from 'react-dnd'
import { setToggleableFields } from '../actions'
import { playTracks } from '../actions'
import { AlbumLinkField } from './AlbumLinkField'
import config from '../config'
import ExpandInfoDialog from '../dialogs/ExpandInfoDialog'

const useRowStyles = makeStyles({
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

const useMenuStyles = makeStyles((theme) => ({
  menuIcon: {
    position: 'relative',
    top: '-0.5em',
  },
  menu: {
    width: '24ch',
  },
  columns: {
    maxHeight: '21rem',
    overflow: 'auto',
    paddingBottom: theme.spacing ? theme.spacing(1) : 8,
  },
  title: {
    margin: '1rem',
  },
  menuItem: {
    display: 'flex',
    alignItems: 'center',
    cursor: 'grab',
    gap: theme.spacing ? theme.spacing(1) : 8,
    transition: 'transform 180ms ease, background-color 180ms ease, opacity 180ms ease',
  },
  dragging: {
    transform: 'scale(1.01)',
    opacity: 0.75,
  },
  dragHandle: {
    display: 'flex',
    alignItems: 'center',
    color: theme.palette?.text?.secondary || '#888',
  },
  label: {
    flex: 1,
    display: 'flex',
    alignItems: 'center',
  },
}))

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

const ITEM_TYPE = 'reorderable-column'

const arraysEqual = (a, b) =>
  a.length === b.length && a.every((value, index) => value === b[index])

const ReorderableMenuItem = ({
  id,
  index,
  label,
  checked,
  onToggle,
  moveItem,
  classes,
}) => {
  const ref = useRef(null)
  const dragState = useRef(false)
  const [, drop] = useDrop({
    accept: ITEM_TYPE,
    hover(item) {
      if (!ref.current || item.id === id) return
      if (item.id === id) return
      moveItem(item.id, id)
      item.index = index
    },
  })

  const [{ isDragging }, drag] = useDrag({
    type: ITEM_TYPE,
    item: () => {
      dragState.current = true
      return { id, index }
    },
    end: () => {
      setTimeout(() => {
        dragState.current = false
      }, 0)
    },
    collect: (monitor) => ({
      isDragging: monitor.isDragging(),
    }),
  })

  drag(drop(ref))

  const handleClick = () => {
    if (dragState.current) return
    onToggle(id)
  }

  return (
    <MenuItem
      ref={ref}
      className={`${classes.menuItem} ${isDragging ? classes.dragging : ''}`}
      onClick={handleClick}
    >
      <span className={classes.dragHandle} aria-hidden>
        <DragIndicatorIcon fontSize="small" />
      </span>
      <Checkbox checked={checked} tabIndex={-1} />
      <span className={classes.label}>{label}</span>
    </MenuItem>
  )
}

const ReorderableToggleFieldsMenu = ({
  resource,
  toggleableColumns,
  omittedColumns,
  columnOrder,
  onToggleColumn,
  onMoveColumn,
}) => {
  const [anchorEl, setAnchorEl] = useState(null)
  const classes = useMenuStyles()
  const translate = useTranslate()

  const open = Boolean(anchorEl)

  const handleOpen = (event) => {
    setAnchorEl(event.currentTarget)
  }

  const handleClose = () => {
    setAnchorEl(null)
  }

  const visibleColumns = useMemo(() => {
    if (!columnOrder) return []
    return columnOrder.filter(
      (key) =>
        !omittedColumns.includes(key) &&
        toggleableColumns &&
        Object.prototype.hasOwnProperty.call(toggleableColumns, key),
    )
  }, [columnOrder, omittedColumns, toggleableColumns])

  return (
    <div className={classes.menuIcon}>
      <IconButton aria-label="more" aria-haspopup="true" onClick={handleOpen}>
        <MoreVertIcon />
      </IconButton>
      <Menu
        id="reorderable-columns-menu"
        anchorEl={anchorEl}
        keepMounted
        open={open}
        onClose={handleClose}
        classes={{
          paper: classes.menu,
        }}
      >
        <Typography className={classes.title}>
          {translate('ra.toggleFieldsMenu.columnsToDisplay')}
        </Typography>
        <div className={classes.columns}>
          {visibleColumns.map((key, index) => (
            <ReorderableMenuItem
              key={key}
              id={key}
              index={index}
              label={translate(`resources.${resource}.fields.${key}`)}
              checked={Boolean(toggleableColumns?.[key])}
              onToggle={onToggleColumn}
              moveItem={onMoveColumn}
              classes={classes}
            />
          ))}
        </div>
      </Menu>
    </div>
  )
}

const SongFilter = (props) => {
  const classes = useRowStyles()
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

const reorderList = (list, dragId, hoverId) => {
  const dragIndex = list.indexOf(dragId)
  const hoverIndex = list.indexOf(hoverId)
  if (dragIndex === -1 || hoverIndex === -1 || dragIndex === hoverIndex) {
    return null
  }
  const updated = [...list]
  updated.splice(dragIndex, 1)
  updated.splice(hoverIndex, 0, dragId)
  return updated
}

const ReorderableSongListActions = ({
  filters,
  displayedFilters,
  filterValues,
  resource,
  showFilter,
  columnOrder,
  toggleableColumns,
  omittedColumns,
  onToggleColumn,
  onMoveColumn,
  ...rest
}) => {
  const isNotSmall = useMediaQuery((theme) => theme.breakpoints.up('sm'))
  return (
    <TopToolbar {...sanitizeListRestProps(rest)}>
      <ShuffleAllButton filters={filterValues} />
      {filters &&
        cloneElement(filters, {
          resource,
          showFilter,
          displayedFilters,
          filterValues,
          context: 'button',
        })}
      {isNotSmall && (
        <ReorderableToggleFieldsMenu
          resource="song"
          toggleableColumns={toggleableColumns}
          omittedColumns={omittedColumns}
          columnOrder={columnOrder}
          onToggleColumn={onToggleColumn}
          onMoveColumn={onMoveColumn}
        />
      )}
    </TopToolbar>
  )
}

const ReorderableSongList = (props) => {
  const rowClasses = useRowStyles()
  const dispatch = useDispatch()
  const isXsmall = useMediaQuery((theme) => theme.breakpoints.down('xs'))
  const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
  useResourceRefresh('song')

  const songs = useSelector((state) => state.admin.resources.song)
  const toggleableState = useSelector(
    (state) => state.settings.toggleableFields?.song,
  )
  const omittedColumns = useSelector(
    (state) => state.settings.omittedFields?.song || [],
  )

  const [columnOrder, setColumnOrder] = useState([])

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

  const toggleableComponents = useMemo(() => {
    return {
      album: isDesktop && <AlbumLinkField source="album" sortByOrder={'ASC'} />,
      artist: <ArtistLinkField source="artist" />,
      albumArtist: <ArtistLinkField source="albumArtist" />,
      trackNumber: isDesktop && <NumberField source="trackNumber" />,
      playCount:
        isDesktop && <NumberField source="playCount" sortByOrder={'DESC'} />,
      playDate: <DateField source="playDate" sortByOrder={'DESC'} showTime />,
      year:
        isDesktop && (
          <FunctionField
            source="year"
            render={(r) => r.year || ''}
            sortByOrder={'DESC'}
          />
        ),
      quality: isDesktop && <QualityInfo source="quality" sortable={false} />,
      channels: isDesktop && <NumberField source="channels" sortByOrder={'ASC'} />,
      duration: <DurationField source="duration" />,
      rating:
        config.enableStarRating && (
          <RatingField
            source="rating"
            sortByOrder={'DESC'}
            resource={'song'}
            className={rowClasses.ratingField}
          />
        ),
      bpm: isDesktop && <NumberField source="bpm" />,
      genre: <TextField source="genre" />,
      mood:
        isDesktop && (
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
  }, [isDesktop, rowClasses.ratingField])

  useEffect(() => {
    const componentKeys = Object.keys(toggleableComponents)
    const stateKeys = toggleableState ? Object.keys(toggleableState) : []
    const merged = []

    stateKeys.forEach((key) => {
      if (
        Object.prototype.hasOwnProperty.call(toggleableComponents, key) &&
        !merged.includes(key)
      ) {
        merged.push(key)
      }
    })

    componentKeys.forEach((key) => {
      if (!merged.includes(key)) {
        merged.push(key)
      }
    })

    if (!arraysEqual(merged, columnOrder)) {
      setColumnOrder(merged)
    }
  }, [toggleableComponents, toggleableState, columnOrder])

  const orderedColumns = useMemo(() => {
    const ordered = {}
    columnOrder.forEach((key) => {
      if (Object.prototype.hasOwnProperty.call(toggleableComponents, key)) {
        ordered[key] = toggleableComponents[key]
      }
    })
    return ordered
  }, [columnOrder, toggleableComponents])

  const columns = useSelectedFields({
    resource: 'song',
    columns: orderedColumns,
    defaultOff: DEFAULT_OFF_COLUMNS,
  })

  const commitOrder = useCallback(
    (order) => {
      if (!toggleableState) return order
      const combinedOrder = [
        ...order,
        ...Object.keys(toggleableState).filter((key) => !order.includes(key)),
      ]
      const next = {}
      combinedOrder.forEach((key) => {
        if (Object.prototype.hasOwnProperty.call(toggleableState, key)) {
          next[key] = toggleableState[key]
        } else {
          next[key] = !DEFAULT_OFF_COLUMNS.includes(key)
        }
      })
      dispatch(setToggleableFields({ song: next }))
      return combinedOrder
    },
    [dispatch, toggleableState],
  )

  const handleMoveColumn = useCallback(
    (dragId, hoverId) => {
      setColumnOrder((prevOrder) => {
        const reordered = reorderList(prevOrder, dragId, hoverId)
        if (!reordered) {
          return prevOrder
        }
        return commitOrder(reordered)
      })
    },
    [commitOrder],
  )

  const handleToggleColumn = useCallback(
    (column) => {
      if (!toggleableState) return
      const updated = {
        ...toggleableState,
        [column]: !toggleableState[column],
      }
      const orderedKeys = [
        ...columnOrder.filter((key) => Object.prototype.hasOwnProperty.call(updated, key)),
        ...Object.keys(updated).filter((key) => !columnOrder.includes(key)),
      ]
      const next = {}
      orderedKeys.forEach((key) => {
        next[key] = updated[key]
      })
      dispatch(setToggleableFields({ song: next }))
      setColumnOrder(orderedKeys)
    },
    [dispatch, toggleableState, columnOrder],
  )

  const actions = useMemo(
    () => (
      <ReorderableSongListActions
        columnOrder={columnOrder}
        toggleableColumns={toggleableState}
        omittedColumns={omittedColumns}
        onToggleColumn={handleToggleColumn}
        onMoveColumn={handleMoveColumn}
      />
    ),
    [
      columnOrder,
      toggleableState,
      omittedColumns,
      handleToggleColumn,
      handleMoveColumn,
    ],
  )

  return (
    <>
      <List
        {...props}
        sort={{ field: 'title', order: 'ASC' }}
        exporter={false}
        bulkActionButtons={<SongBulkActions />}
        actions={actions}
        filters={<SongFilter />}
        perPage={isXsmall ? 50 : 15}
      >
        {isXsmall ? (
          <SongSimpleList />
        ) : (
          <SongDatagrid
            rowClick={handleRowClick}
            contextAlwaysVisible={!isDesktop}
            classes={{ row: rowClasses.row }}
          >
            <SongTitleField source="title" showTrackNumbers={false} />
            {columns}
            <SongContextMenu
              source={'starred_at'}
              sortByOrder={'DESC'}
              sortable={config.enableFavourites}
              className={rowClasses.contextMenu}
              label={
                config.enableFavourites && (
                  <FavoriteBorderIcon
                    fontSize={'small'}
                    className={rowClasses.contextHeader}
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
