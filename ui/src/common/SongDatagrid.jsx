import React, {
  isValidElement,
  useMemo,
  useCallback,
  forwardRef,
  useState,
  useEffect,
  useRef,
  useContext,
  createContext,
} from 'react'
import { useDispatch } from 'react-redux'
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
import { useDrag, useDrop } from 'react-dnd'
import { playTracks } from '../actions'
import { AlbumContextMenu } from '../common'
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'
import { DraggableTypes } from '../consts'
import { formatFullDate } from '../utils'

const ORDER_KEY = 'nd:songs:columnOrder'
const VISIBLE_KEY = 'nd:songs:visibleColumns'

const SongColumnsContext = createContext()
export const useSongColumns = () => useContext(SongColumnsContext)

const useStyles = makeStyles({
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
    '&:hover': {
      '& $contextMenu': {
        visibility: 'visible',
      },
    },
  },
  missingRow: {
    cursor: 'inherit',
    opacity: 0.3,
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
  contextMenu: {
    visibility: (props) => (props.isDesktop ? 'hidden' : 'visible'),
  },
})

const ColumnHeader = ({ id, index, moveColumn, label }) => {
  const ref = useRef(null)
  const [, drop] = useDrop({
    accept: DraggableTypes.COLUMN,
    drop: (item) => {
      if (item.id !== id) moveColumn(item.id, index)
    },
  })
  drop(ref)

  const [, drag] = useDrag({
    type: DraggableTypes.COLUMN,
    item: { id },
  })

  return (
    <span
      ref={ref}
      style={{ display: 'inline-flex', alignItems: 'center' }}
      onClick={(e) => e.stopPropagation()}
    >
      <span
        ref={drag}
        onMouseDown={(e) => e.stopPropagation()}
        style={{ cursor: 'grab', display: 'inline-flex', marginRight: 4 }}
      >
        <DragIndicatorIcon fontSize="small" />
      </span>
      {label}
    </span>
  )
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
  const fields = React.Children.toArray(children).filter((c) =>
    isValidElement(c),
  )

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
      item: { ids: [record?.mediaFileId || record?.id] },
      options: { dropEffect: 'copy' },
    }),
    [record],
  )

  if (!record || !record.title) {
    return null
  }

  const rowClick = record.missing ? undefined : rest.rowClick

  const computedClasses = clsx(
    className,
    classes.row,
    record.missing && classes.missingRow,
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
        ref={dragSongRef}
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
  children,
  ...rest
}) => {
  const classes = useStyles()
  const translate = useTranslate()

  const childArray = React.Children.toArray(children).filter((c) =>
    isValidElement(c),
  )

  const columns = childArray.map((child, index) => {
    const id = child.props.source || child.props.id || `col${index}`
    const label =
      child.props.label ||
      (child.props.source
        ? translate(`resources.song.fields.${child.props.source}`)
        : id)
    const pinned = child.props.pinned || index === childArray.length - 1
    return { id, label, renderCell: child, visible: true, pinned }
  })

  const defaultOrder = columns.filter((c) => !c.pinned).map((c) => c.id)

  const [columnOrder, setColumnOrder] = useState(() => {
    try {
      const stored = JSON.parse(localStorage.getItem(ORDER_KEY))
      if (stored && Array.isArray(stored)) return stored
    } catch (e) {}
    return defaultOrder
  })

  const [visibleColumns, setVisibleColumns] = useState(() => {
    try {
      const stored = JSON.parse(localStorage.getItem(VISIBLE_KEY))
      if (stored && Array.isArray(stored)) return new Set(stored)
    } catch (e) {}
    return new Set(columns.map((c) => c.id))
  })

  useEffect(() => {
    localStorage.setItem(ORDER_KEY, JSON.stringify(columnOrder))
  }, [columnOrder])

  useEffect(() => {
    localStorage.setItem(
      VISIBLE_KEY,
      JSON.stringify(Array.from(visibleColumns)),
    )
  }, [visibleColumns])

  const moveColumn = useCallback((id, toIndex) => {
    setColumnOrder((prev) => {
      const order = prev.filter((c) => c !== id)
      order.splice(toIndex, 0, id)
      return [...order]
    })
  }, [])

  const toggleColumn = useCallback((id) => {
    setVisibleColumns((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }, [])

  const resetColumns = useCallback(() => {
    setColumnOrder(defaultOrder)
    setVisibleColumns(new Set(columns.map((c) => c.id)))
  }, [columns, defaultOrder])

  const firstMovable = columns.findIndex((c) => !c.pinned)
  const pinnedStart = []
  const pinnedEnd = []
  const movable = []
  columns.forEach((c, idx) => {
    if (c.pinned) {
      if (idx < firstMovable) pinnedStart.push(c)
      else pinnedEnd.push(c)
    } else movable.push(c)
  })
  const orderedMovable = columnOrder
    .map((id) => movable.find((c) => c.id === id))
    .filter(Boolean)
  const orderedColumns = [...pinnedStart, ...orderedMovable, ...pinnedEnd]

  const elements = []
  orderedColumns.forEach((c) => {
    if (!visibleColumns.has(c.id)) return
    if (c.pinned) {
      elements.push(
        React.cloneElement(c.renderCell, {
          key: c.id,
          label: c.label,
        }),
      )
    } else {
      const index = orderedMovable.findIndex((m) => m.id === c.id)
      elements.push(
        React.cloneElement(c.renderCell, {
          key: c.id,
          label: (
            <ColumnHeader
              id={c.id}
              index={index}
              moveColumn={moveColumn}
              label={c.label}
            />
          ),
        }),
      )
    }
  })

  const providerValue = {
    columnOrder,
    visibleColumns,
    moveColumn,
    toggleColumn,
    resetColumns,
    columns,
  }

  return (
    <SongColumnsContext.Provider value={providerValue}>
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
      >
        {elements}
      </Datagrid>
    </SongColumnsContext.Provider>
  )
}

SongDatagrid.propTypes = {
  contextAlwaysVisible: PropTypes.bool,
  showDiscSubtitles: PropTypes.bool,
  classes: PropTypes.object,
  children: PropTypes.node,
}
