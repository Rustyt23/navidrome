import React, {
  isValidElement,
  useMemo,
  useCallback,
  forwardRef,
  useState,
  useEffect,
  useRef,
} from 'react'
import { useDispatch } from 'react-redux'
import {
  Datagrid,
  PureDatagridBody,
  PureDatagridRow,
  useTranslate,
  FieldTitle,
  useListContext,
  useResourceContext,
} from 'react-admin'
import {
  TableCell,
  TableRow,
  Typography,
  useMediaQuery,
  Tooltip,
  TableSortLabel,
  TableHead,
  Checkbox,
} from '@material-ui/core'
import PropTypes from 'prop-types'
import { makeStyles } from '@material-ui/core/styles'
import AlbumIcon from '@material-ui/icons/Album'
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'
import clsx from 'clsx'
import { useDrag, useDrop } from 'react-dnd'
import { playTracks } from '../actions'
import { AlbumContextMenu } from '../common'
import { ColumnsCustomizer } from './ColumnsCustomizer'
import { DraggableTypes } from '../consts'
import { formatFullDate } from '../utils'

const ORDER_KEY = 'nd:songs:columnOrder'
const VISIBLE_KEY = 'nd:songs:visibleColumns'
const COLUMN_DND_TYPE = 'COLUMN'

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

const SongDatagridHeaderCell = ({
  className,
  field,
  currentSort,
  updateSort,
  id,
  moveColumn,
  isPinned,
  resource,
}) => {
  const ref = useRef(null)
  const [{ isDragging }, drag] = useDrag({
    type: COLUMN_DND_TYPE,
    item: { id },
    canDrag: !isPinned,
    collect: (monitor) => ({ isDragging: monitor.isDragging() }),
  })
  const [, drop] = useDrop({
    accept: COLUMN_DND_TYPE,
    hover(item) {
      if (!ref.current) return
      if (item.id === id) return
      moveColumn(item.id, id)
      item.id = id
    },
  })
  drag(drop(ref))
  const translate = useTranslate()
  const handle = !isPinned ? (
    <span
      onClick={(e) => e.stopPropagation()}
      style={{ cursor: 'move', display: 'inline-flex', verticalAlign: 'middle' }}
    >
      <DragIndicatorIcon fontSize="small" />
    </span>
  ) : null
  return (
    <TableCell
      ref={ref}
      className={className}
      align={field.props.textAlign}
      variant="head"
      style={{ opacity: isDragging ? 0.5 : 1 }}
    >
      {updateSort &&
      field.props.sortable !== false &&
      (field.props.sortBy || field.props.source) ? (
        <Tooltip
          title={translate('ra.action.sort')}
          placement={
            field.props.textAlign === 'right' ? 'bottom-end' : 'bottom-start'
          }
          enterDelay={300}
        >
          <TableSortLabel
            active={
              currentSort.field ===
              (field.props.sortBy || field.props.source)
            }
            direction={currentSort.order === 'ASC' ? 'asc' : 'desc'}
            data-sort={field.props.sortBy || field.props.source}
            data-field={field.props.sortBy || field.props.source}
            data-order={field.props.sortByOrder || 'ASC'}
            onClick={updateSort}
          >
            {handle}
            <FieldTitle
              label={field.props.label}
              source={field.props.source}
              resource={resource}
            />
          </TableSortLabel>
        </Tooltip>
      ) : (
        <span>
          {handle}
          <FieldTitle
            label={field.props.label}
            source={field.props.source}
            resource={resource}
          />
        </span>
      )}
    </TableCell>
  )
}

SongDatagridHeaderCell.propTypes = {
  className: PropTypes.string,
  field: PropTypes.element,
  currentSort: PropTypes.object,
  updateSort: PropTypes.func,
  id: PropTypes.string,
  moveColumn: PropTypes.func,
  isPinned: PropTypes.bool,
  resource: PropTypes.string,
}

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

const SongDatagridHeader = ({
  children,
  classes,
  className,
  hasExpand = false,
  hasBulkActions = false,
  isRowSelectable,
  moveColumn,
  pinnedEndId,
}) => {
  const resource = useResourceContext()
  const translate = useTranslate()
  const { currentSort, data, ids, onSelect, selectedIds, setSort } =
    useListContext()

  const updateSort = useCallback(
    (event) => {
      event.stopPropagation()
      const newField = event.currentTarget.dataset.field
      const newOrder =
        currentSort.field === newField
          ? currentSort.order === 'ASC'
            ? 'DESC'
            : 'ASC'
          : event.currentTarget.dataset.order
      setSort(newField, newOrder)
    },
    [currentSort.field, currentSort.order, setSort],
  )

  const handleSelectAll = useCallback(
    (event) => {
      onSelect(
        event.target.checked
          ? ids.filter((id) =>
              isRowSelectable ? isRowSelectable(data[id]) : true,
            )
          : [],
      )
    },
    [data, ids, onSelect, isRowSelectable],
  )

  return (
    <TableHead className={className}>
      <TableRow className={clsx(classes && classes.row, classes && classes.headerRow)}>
        {hasExpand && <TableCell padding="none" />}
        {hasBulkActions && selectedIds && (
          <TableCell padding="checkbox" className={classes && classes.headerCell}>
            <Checkbox
              aria-label={translate('ra.action.select_all', { _: 'Select all' })}
              className="select-all"
              color="primary"
              checked={
                selectedIds.length > 0 &&
                ids.length > 0 &&
                ids.every((id) =>
                  isRowSelectable ? isRowSelectable(data[id]) : true,
                )
              }
              onChange={handleSelectAll}
            />
          </TableCell>
        )}
        {React.Children.map(children, (field) => {
          if (!isValidElement(field)) return null
          const id = field.props.source || field.props.id
          const isPinned = id === pinnedEndId
          return (
            <SongDatagridHeaderCell
              key={id}
              className={classes && classes.headerCell}
              field={field}
              currentSort={currentSort}
              updateSort={setSort ? updateSort : null}
              id={id}
              moveColumn={moveColumn}
              isPinned={isPinned}
              resource={resource}
            />
          )
        })}
      </TableRow>
    </TableHead>
  )
}

SongDatagridHeader.propTypes = {
  children: PropTypes.node,
  classes: PropTypes.object,
  className: PropTypes.string,
  hasExpand: PropTypes.bool,
  hasBulkActions: PropTypes.bool,
  isRowSelectable: PropTypes.func,
  moveColumn: PropTypes.func,
  pinnedEndId: PropTypes.string,
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
  const pinnedEnd = childArray[childArray.length - 1]
  const movableChildren = childArray.slice(0, -1)

  const columns = useMemo(
    () =>
      movableChildren.map((c) => ({
        id: c.props.source || c.props.id,
        label:
          c.props.label || translate(`resources.song.fields.${c.props.source}`),
        element: c,
      })),
    [movableChildren, translate],
  )

  const pinnedColumns = useMemo(() => {
    const endId = pinnedEnd.props.source || pinnedEnd.props.id || 'context'
    return [
      { id: 'select', label: translate('ra.action.select'), element: null },
      {
        id: endId,
        label:
          pinnedEnd.props.label ||
          translate(`resources.song.fields.${pinnedEnd.props.source}`),
        element: pinnedEnd,
      },
    ]
  }, [pinnedEnd, translate])

  const movableIds = columns.map((c) => c.id)
  const defaultOrder = movableIds
  const allIds = ['select', ...movableIds, pinnedColumns[1].id]
  const defaultVisible = allIds

  const [columnOrder, setColumnOrder] = useState(defaultOrder)
  const [visibleColumns, setVisibleColumns] = useState(new Set(defaultVisible))

  useEffect(() => {
    try {
      const storedOrder = JSON.parse(localStorage.getItem(ORDER_KEY) || '[]')
      let order = Array.isArray(storedOrder)
        ? storedOrder.filter((id) => movableIds.includes(id))
        : []
      movableIds.forEach((id) => {
        if (!order.includes(id)) order.push(id)
      })
      setColumnOrder(order)
      localStorage.setItem(ORDER_KEY, JSON.stringify(order))
    } catch {
      setColumnOrder(defaultOrder)
    }
    try {
      const storedVisible = JSON.parse(
        localStorage.getItem(VISIBLE_KEY) || '[]',
      )
      let vis = Array.isArray(storedVisible)
        ? storedVisible.filter((id) => allIds.includes(id))
        : []
      pinnedColumns.forEach((p) => {
        if (!vis.includes(p.id)) vis.push(p.id)
      })
      if (vis.length === 0) vis = defaultVisible
      setVisibleColumns(new Set(vis))
      localStorage.setItem(VISIBLE_KEY, JSON.stringify(vis))
    } catch {
      setVisibleColumns(new Set(defaultVisible))
    }
  }, [movableIds, allIds, pinnedColumns, defaultOrder, defaultVisible])

  const moveColumn = useCallback(
    (dragId, hoverId) => {
      const updated = [...columnOrder]
      const dragIndex = updated.indexOf(dragId)
      const hoverIndex = updated.indexOf(hoverId)
      if (dragIndex === -1 || hoverIndex === -1) return
      updated.splice(dragIndex, 1)
      updated.splice(hoverIndex, 0, dragId)
      setColumnOrder(updated)
      localStorage.setItem(ORDER_KEY, JSON.stringify(updated))
    },
    [columnOrder],
  )

  const handleSetOrder = useCallback((order) => {
    setColumnOrder(order)
    localStorage.setItem(ORDER_KEY, JSON.stringify(order))
  }, [])

  const handleToggleColumn = useCallback(
    (id) => {
      const updated = new Set(visibleColumns)
      if (updated.has(id)) {
        updated.delete(id)
      } else {
        updated.add(id)
      }
      pinnedColumns.forEach((p) => updated.add(p.id))
      const arr = Array.from(updated)
      setVisibleColumns(updated)
      localStorage.setItem(VISIBLE_KEY, JSON.stringify(arr))
    },
    [visibleColumns, pinnedColumns],
  )

  const handleReset = useCallback(() => {
    localStorage.removeItem(ORDER_KEY)
    localStorage.removeItem(VISIBLE_KEY)
    setColumnOrder(defaultOrder)
    setVisibleColumns(new Set(defaultVisible))
  }, [defaultOrder, defaultVisible])

  const orderedChildren = [
    ...columnOrder.map((id) => columns.find((c) => c.id === id)?.element),
    pinnedColumns[1].element,
  ].filter((c) => c)

  const renderedChildren = orderedChildren.filter((c) => {
    const id = c.props.source || c.props.id
    return visibleColumns.has(id)
  })

  return (
    <div>
      <ColumnsCustomizer
        columns={columns}
        pinnedColumns={pinnedColumns}
        columnOrder={columnOrder}
        onOrderChange={handleSetOrder}
        visibleColumns={visibleColumns}
        onVisibilityChange={handleToggleColumn}
        onReset={handleReset}
      />
      <Datagrid
        className={classes.headerStyle}
        isRowSelectable={(r) => !r?.missing}
        {...rest}
        header={
          <SongDatagridHeader
            moveColumn={moveColumn}
            pinnedEndId={pinnedColumns[1].id}
          />
        }
        body={
          <SongDatagridBody
            contextAlwaysVisible={contextAlwaysVisible}
            showDiscSubtitles={showDiscSubtitles}
          />
        }
      >
        {renderedChildren}
      </Datagrid>
    </div>
  )
}

SongDatagrid.propTypes = {
  contextAlwaysVisible: PropTypes.bool,
  showDiscSubtitles: PropTypes.bool,
  classes: PropTypes.object,
  children: PropTypes.node,
}
