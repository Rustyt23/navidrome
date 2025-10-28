import React, {
  isValidElement,
  useMemo,
  useCallback,
  forwardRef,
  useEffect,
  useRef,
  useState,
} from 'react'
import { useDispatch, useSelector } from 'react-redux'
import {
  Datagrid,
  PureDatagridBody,
  PureDatagridRow,
  useTranslate,
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
    cursor: 'grab',
    WebkitUserDrag: 'element',
    userSelect: 'none',
    WebkitUserSelect: 'none',
    MozUserSelect: 'none',
    msUserSelect: 'none',
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
  rowDragging: {
    cursor: 'grabbing',
    userSelect: 'none',
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
  dragPreview: {
    position: 'fixed',
    top: 0,
    left: 0,
    zIndex: 99999,
    padding: '8px 12px',
    borderRadius: 8,
    background: 'rgba(40, 40, 40, 0.92)',
    color: '#fff',
    fontSize: 13,
    lineHeight: 1,
    boxShadow: '0 6px 24px rgba(0, 0, 0, 0.25)',
    pointerEvents: 'none',
  },
}))

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
  const [isDragging, setIsDragging] = useState(false)
  const rowRef = useRef(null)
  const dragPreviewRef = useRef(null)

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

  const resourceName = rest?.resource
  const selectedIds = useSelector(
    (state) =>
      (resourceName &&
        state?.admin?.resources?.[resourceName]?.list?.selectedIds) ||
      [],
  )
  const resourceRecords = useSelector(
    (state) =>
      (resourceName && state?.admin?.resources?.[resourceName]?.data) || {},
  )

  const recordId = record?.id
  const trackId = record?.mediaFileId || recordId

  const normalizedSelectedIds = useMemo(() => {
    const selection = Array.isArray(selectedIds) ? selectedIds : []
    const normalized = new Set()
    selection.forEach((id) => {
      if (id == null) {
        return
      }
      normalized.add(String(id))
    })
    return normalized
  }, [selectedIds])

  const getDragTrackIds = useCallback(() => {
    const selection = Array.isArray(selectedIds) ? selectedIds : []
    const normalizedRecordId = recordId == null ? null : String(recordId)
    const isSelected =
      normalizedRecordId != null && normalizedSelectedIds.has(normalizedRecordId)
    const baseIds = isSelected ? selection : [recordId]
    const seen = new Set()
    const ids = []

    baseIds.forEach((id) => {
      if (id == null) {
        return
      }
      const dataRecord = resourceRecords?.[id]
      const value =
        dataRecord?.mediaFileId ||
        dataRecord?.id ||
        (String(id) === normalizedRecordId ? trackId : id)
      if (!value || seen.has(value)) {
        return
      }
      seen.add(value)
      ids.push(value)
    })

    if (!ids.length && trackId && !seen.has(trackId)) {
      ids.push(trackId)
    }

    return ids
  }, [
    normalizedSelectedIds,
    selectedIds,
    recordId,
    resourceRecords,
    trackId,
  ])

  const [, dragSongRef] = useDrag(
    () => ({
      type: DraggableTypes.SONG,
      item: () => ({ ids: getDragTrackIds() }),
      options: { dropEffect: 'copy' },
    }),
    [getDragTrackIds],
  )

  useEffect(() => {
    const node = rowRef.current
    if (!node) {
      return undefined
    }
    const preventTextSelection = (event) => {
      if (
        event?.target?.closest(
          'button,input,textarea,select,a,[data-no-drag]',
        )
      ) {
        return
      }
      event.preventDefault()
    }
    node.addEventListener('selectstart', preventTextSelection)

    const interactiveSelector =
      'button,input,textarea,select,a,[data-no-drag]'
    const interactiveElements = Array.from(
      node.querySelectorAll(interactiveSelector),
    )
    const handleChildDragStart = (event) => {
      event.stopPropagation()
    }
    interactiveElements.forEach((element) => {
      element.setAttribute('draggable', 'false')
      element.addEventListener('dragstart', handleChildDragStart)
    })
    return () => {
      node.removeEventListener('selectstart', preventTextSelection)
      interactiveElements.forEach((element) => {
        element.removeEventListener('dragstart', handleChildDragStart)
      })
    }
  }, [record?.id])

  const handleDragStart = useCallback(
    (event) => {
      if (event?.target?.closest('button,input,textarea,select,a,[data-no-drag]')) {
        return
      }
      if (!event?.dataTransfer) {
        return
      }
      const ids = Array.from(new Set(getDragTrackIds()?.filter(Boolean) || []))
      if (!ids.length) {
        return
      }
      const payload = { kind: 'tracks', ids }
      try {
        event.dataTransfer.setData(
          'application/x-navidrome-tracks',
          JSON.stringify(payload),
        )
      } catch (error) {
        // Ignore serialization errors and fall back to text/plain
      }
      event.dataTransfer.setData('text/plain', ids.join(','))
      // multi-select drag payload: include all selected track IDs
      event.dataTransfer.effectAllowed = 'copy'

      const label =
        ids.length > 1 ? `${ids.length} items` : record?.title || '1 item'
      const ghost = document.createElement('div')
      ghost.className = classes.dragPreview
      ghost.textContent = label
      document.body.appendChild(ghost)
      dragPreviewRef.current = ghost
      if (typeof event.dataTransfer.setDragImage === 'function') {
        const { width, height } = ghost.getBoundingClientRect()
        event.dataTransfer.setDragImage(ghost, width / 2, height / 2)
      }
      setTimeout(() => {
        if (dragPreviewRef.current === ghost) {
          ghost.remove()
          dragPreviewRef.current = null
        }
      }, 0)
      setIsDragging(true)
    },
    [classes.dragPreview, getDragTrackIds, record?.title],
  )

  const handleDragEnd = useCallback(() => {
    if (dragPreviewRef.current) {
      dragPreviewRef.current.remove()
      dragPreviewRef.current = null
    }
    setIsDragging(false)
  }, [dragPreviewRef])

  const setRowRef = useCallback(
    (node) => {
      rowRef.current = node
      if (!record?.missing) {
        dragSongRef(node)
      }
    },
    [dragSongRef, record?.missing],
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
    isDragging && classes.rowDragging,
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
        ref={setRowRef}
        record={record}
        {...rest}
        rowClick={rowClick}
        className={computedClasses}
        onDragStart={handleDragStart}
        onDragEnd={handleDragEnd}
        draggable={!record?.missing}
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
