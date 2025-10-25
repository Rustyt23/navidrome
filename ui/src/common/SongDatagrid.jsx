import React, { isValidElement, useMemo, useCallback, forwardRef } from 'react'
import { useDispatch, useSelector } from 'react-redux'
import {
  Datagrid,
  PureDatagridBody,
  PureDatagridRow,
  useListContext,
} from 'react-admin'
import {
  TableCell,
  TableRow,
  Typography,
  useMediaQuery,
} from '@material-ui/core'
import PropTypes from 'prop-types'
import { makeStyles } from '@material-ui/core/styles'
import AlbumIcon from '@material-ui/icons/Album'
import clsx from 'clsx'
import { useDrag } from 'react-dnd'
import { playTracks } from '../actions'
import { AlbumContextMenu } from '../common'
import { DraggableTypes } from '../consts'
import { formatFullDate } from '../utils'

const useStyles = makeStyles((theme) => ({
  subtitle: {
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    verticalAlign: 'middle',
  },
  discIcon: {
    verticalAlign: 'text-top',
    marginRight: '4px',
  },
  row: {
    cursor: 'pointer',
    // ↓ shrink row height by reducing vertical padding on all table cells
    '& td, & th, & .MuiTableCell-root': {
      paddingTop: 3,
      paddingBottom: 3,
    },
    '&:hover': {
      '& $contextMenu': {
        visibility: 'visible',
      },
    },
  },
  currentRow: {
    backgroundColor: theme.palette.action.hover,
    '& td, & th, & .MuiTableCell-root': {
      color: '#ff007f',
    },
    '& a': {
      color: '#ff007f',
    },
    '& svg': {
      fill: '#ff007f',
      color: '#ff007f',
    },
    '& $contextMenu': {
      visibility: 'visible',
    },
  },
  missingRow: {
    cursor: 'inherit',
    opacity: 0.3,
    '& td, & th, & .MuiTableCell-root, & a, & svg': {
      color: theme.palette.text.disabled,
    },
    '& .draggable': {
      cursor: 'default',
      pointerEvents: 'none',
    },
  },
  headerStyle: {
    '& thead': {
      boxShadow: '0px 3px 3px rgba(0, 0, 0, 0.15)',
    },
    '& th': {
      fontWeight: 'bold',
      padding: '15px',
    },
  },
  contextMenu: (props) => ({
    visibility: props?.isDesktop ? 'hidden' : 'visible',
  }),
}))

const toComparableId = (value) => {
  if (value === undefined || value === null) {
    return null
  }

  return String(value)
}

const extractTrackId = (candidate) => {
  if (!candidate || candidate.missing) {
    return null
  }

  const id = candidate.mediaFileId ?? candidate.id
  return id ?? null
}

const DiscSubtitleRow = forwardRef(
  ({ record, onClick, colSpan, contextAlwaysVisible }, ref) => {
    const isDesktop = useMediaQuery((theme) => theme.breakpoints.up('md'))
    const classes = useStyles({ isDesktop })
    const handlePlaySubset = (discNumber) => () => {
      onClick(discNumber)
    }

    let subtitle = []
    if (record.discNumber > 0) {
      subtitle.push(record.discNumber)
    }
    if (record.discSubtitle) {
      subtitle.push(record.discSubtitle)
    }

    return (
      <TableRow
        hover
        ref={ref}
        onClick={handlePlaySubset(record.discNumber)}
        className={classes.row}
      >
        <TableCell colSpan={colSpan}>
          <Typography variant="h6" className={classes.subtitle}>
            <AlbumIcon className={classes.discIcon} fontSize={'small'} />
            {subtitle.join(': ')}
          </Typography>
        </TableCell>
        <TableCell>
          <AlbumContextMenu
            record={{ id: record.albumId }}
            discNumber={record.discNumber}
            showLove={false}
            className={classes.contextMenu}
            hideShare={true}
            hideInfo={true}
            visible={contextAlwaysVisible}
          />
        </TableCell>
      </TableRow>
    )
  },
)

DiscSubtitleRow.displayName = 'DiscSubtitleRow'

export const SongDatagridRow = ({
  record,
  children,
  firstTracksOfDiscs,
  contextAlwaysVisible,
  onClickSubset,
  className,
  ...rest
}) => {
  const classes = useStyles()
  const currentTrack = useSelector((state) => state?.player?.current || {})
  const currentId = currentTrack.trackId
  const fields = React.Children.toArray(children).filter((c) =>
    isValidElement(c),
  )

  const listContext = useListContext()
  const selectedIds = listContext?.selectedIds ?? rest?.selectedIds ?? []
  const listData = listContext?.data ?? rest?.data
  const listIds = listContext?.ids ?? rest?.ids ?? []

  const comparableIdsForRecord = useMemo(() => {
    const set = new Set()
    const register = (value) => {
      const comparable = toComparableId(value)
      if (comparable !== null) {
        set.add(comparable)
      }
    }

    register(record?.id)
    register(record?.mediaFileId)

    return set
  }, [record?.id, record?.mediaFileId])

  const recordLookup = useMemo(() => {
    const map = new Map()

    const registerRecord = (candidate) => {
      if (!candidate) {
        return
      }

      const attach = (value) => {
        const comparable = toComparableId(value)
        if (comparable !== null && !map.has(comparable)) {
          map.set(comparable, candidate)
        }
      }

      attach(candidate.id)
      attach(candidate.mediaFileId)
    }

    if (Array.isArray(listData)) {
      listData.forEach(registerRecord)
    } else if (listData && typeof listData === 'object') {
      Object.values(listData).forEach(registerRecord)
    }

    registerRecord(record)

    return map
  }, [listData, record])

  const draggedSongIds = useMemo(() => {
    const baseTrackId = extractTrackId(record)
    if (baseTrackId == null) {
      return []
    }

    if (!Array.isArray(selectedIds) || selectedIds.length < 2) {
      return [baseTrackId]
    }

    const selectionComparables = selectedIds
      .map((id) => toComparableId(id))
      .filter((id) => id !== null)

    if (selectionComparables.length === 0) {
      return [baseTrackId]
    }

    const selectionSet = new Set(selectionComparables)
    const recordIsInSelection = selectionComparables.some((id) =>
      comparableIdsForRecord.has(id),
    )

    if (!recordIsInSelection) {
      return [baseTrackId]
    }

    const orderSource =
      Array.isArray(listIds) && listIds.length > 0 ? listIds : selectionComparables

    const collected = []
    const seenTracks = new Set()

    const pushCandidate = (candidate) => {
      const trackId = extractTrackId(candidate)
      if (trackId == null) {
        return
      }

      const comparableTrack = toComparableId(trackId)
      if (comparableTrack === null || seenTracks.has(comparableTrack)) {
        return
      }

      seenTracks.add(comparableTrack)
      collected.push(trackId)
    }

    orderSource.forEach((id) => {
      const comparable = toComparableId(id)
      if (comparable === null || !selectionSet.has(comparable)) {
        return
      }

      const candidate = recordLookup.get(comparable)
      pushCandidate(candidate)
    })

    if (collected.length === 0) {
      pushCandidate(record)
    }

    return collected.length > 0 ? collected : [baseTrackId]
  }, [
    comparableIdsForRecord,
    listIds,
    record,
    recordLookup,
    selectedIds,
  ])

  const [, dragDiscRef] = useDrag(
    () => ({
      type: DraggableTypes.DISC,
      item: {
        discs: [
          {
            albumId: record?.albumId,
            discNumber: record?.discNumber,
          },
        ],
      },
      options: { dropEffect: 'copy' },
    }),
    [record],
  )

  const [, dragSongRef] = useDrag(
    () => ({
      type: DraggableTypes.SONG,
      item: { ids: draggedSongIds },
      canDrag: draggedSongIds.length > 0,
      options: { dropEffect: 'copy' },
    }),
    [draggedSongIds],
  )

  if (!record || !record.title) {
    return null
  }

  const rowClick = record.missing ? undefined : rest.rowClick

  const isCurrent =
    currentId && (currentId === record.id || currentId === record.mediaFileId)

  const computedClasses = clsx(
    className,
    classes.row,
    record.missing && classes.missingRow,
    isCurrent && classes.currentRow,
  )
  const childCount = fields.length
  return (
    <>
      {firstTracksOfDiscs.has(record.id) && (
        <DiscSubtitleRow
          ref={dragDiscRef}
          record={record}
          onClick={onClickSubset}
          contextAlwaysVisible={contextAlwaysVisible}
          colSpan={childCount + (rest.expand ? 1 : 0)}
        />
      )}
      <PureDatagridRow
        ref={record?.missing ? undefined : dragSongRef}
        record={record}
        {...rest}
        rowClick={rowClick}
        className={computedClasses}
      >
        {fields}
      </PureDatagridRow>
    </>
  )
}

SongDatagridRow.propTypes = {
  record: PropTypes.object,
  children: PropTypes.node,
  firstTracksOfDiscs: PropTypes.instanceOf(Set),
  contextAlwaysVisible: PropTypes.bool,
  onClickSubset: PropTypes.func,
}

SongDatagridRow.defaultProps = {
  onClickSubset: () => {},
}

const SongDatagridBody = ({
  contextAlwaysVisible,
  showDiscSubtitles,
  ...rest
}) => {
  const dispatch = useDispatch()
  const { ids, data } = rest

  const playSubset = useCallback(
    (discNumber) => {
      let idsToPlay = []
      if (discNumber !== undefined) {
        idsToPlay = ids.filter((id) => data[id].discNumber === discNumber)
      }
      dispatch(
        playTracks(
          data,
          idsToPlay?.filter((id) => !data[id].missing),
        ),
      )
    },
    [dispatch, data, ids],
  )

  const firstTracksOfDiscs = useMemo(() => {
    if (!ids) {
      return new Set()
    }
    let foundSubtitle = false
    const set = new Set(
      ids
        .filter((i) => data[i])
        .reduce((acc, id) => {
          const last = acc && acc[acc.length - 1]
          foundSubtitle = foundSubtitle || data[id].discSubtitle
          if (
            acc.length === 0 ||
            (last && data[id].discNumber !== data[last].discNumber)
          ) {
            acc.push(id)
          }
          return acc
        }, []),
    )
    if (!showDiscSubtitles || (set.size < 2 && !foundSubtitle)) {
      set.clear()
    }
    return set
  }, [ids, data, showDiscSubtitles])

  return (
    <PureDatagridBody
      {...rest}
      row={
        <SongDatagridRow
          firstTracksOfDiscs={firstTracksOfDiscs}
          contextAlwaysVisible={contextAlwaysVisible}
          onClickSubset={playSubset}
        />
      }
    />
  )
}

export const SongDatagrid = ({
  contextAlwaysVisible,
  showDiscSubtitles,
  ...rest
}) => {
  const classes = useStyles()
  return (
    <Datagrid
      className={classes.headerStyle}
      isRowSelectable={(r) => !r?.missing}
      {...rest}
      body={
        <SongDatagridBody
          contextAlwaysVisible={contextAlwaysVisible}
          showDiscSubtitles={showDiscSubtitles}
        />
      }
    />
  )
}

SongDatagrid.propTypes = {
  contextAlwaysVisible: PropTypes.bool,
  showDiscSubtitles: PropTypes.bool,
  classes: PropTypes.object,
}
