import React, {
  isValidElement,
  useMemo,
  useCallback,
  forwardRef,
  useState,
  Children,
} from 'react'
import { useDispatch, useSelector } from 'react-redux'
import {
  Datagrid,
  PureDatagridBody,
  PureDatagridRow,
  useTranslate,
  useListContext,
  useResourceContext,
  DatagridHeaderCell,
} from 'react-admin'
import {
  TableCell,
  TableRow,
  Typography,
  useMediaQuery,
  TableHead,
  Checkbox,
  Tooltip,
  TableSortLabel,
} from '@material-ui/core'
import PropTypes from 'prop-types'
import { makeStyles } from '@material-ui/core/styles'
import AlbumIcon from '@material-ui/icons/Album'
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'
import clsx from 'clsx'
import { useDrag } from 'react-dnd'
import { DndContext, useDraggable, useDroppable, DragOverlay } from '@dnd-kit/core'
import { FieldTitle } from 'ra-core'
import { playTracks, setColumnsOrder } from '../actions'
import { AlbumContextMenu } from '../common'
import { DraggableTypes } from '../consts'
import { formatFullDate } from '../utils'

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

const useHeaderCellStyles = makeStyles({
  icon: {
    display: 'none',
  },
  active: {
    '& $icon': {
      display: 'inline',
    },
  },
})

const DraggableHeaderCell = ({
  id,
  field,
  currentSort,
  updateSort,
  resource,
  className,
}) => {
  const classes = useHeaderCellStyles()
  const translate = useTranslate()
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id })
  const { setNodeRef: setDroppableRef } = useDroppable({ id })
  const ref = (node) => {
    setNodeRef(node)
    setDroppableRef(node)
  }

  return (
    <TableCell
      ref={ref}
      className={clsx(className, field.props.headerClassName)}
      align={field.props.textAlign}
      variant="head"
      style={{ opacity: isDragging ? 0.5 : 1 }}
    >
      <div style={{ display: 'flex', alignItems: 'center' }}>
        <DragIndicatorIcon className="dragHandle" {...listeners} {...attributes} />
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
                currentSort.field === (field.props.sortBy || field.props.source)
              }
              direction={currentSort.order === 'ASC' ? 'asc' : 'desc'}
              data-sort={field.props.sortBy || field.props.source}
              data-field={field.props.sortBy || field.props.source}
              data-order={field.props.sortByOrder || 'ASC'}
              onClick={updateSort}
              classes={classes}
            >
              <FieldTitle
                label={field.props.label}
                source={field.props.source}
                resource={resource}
              />
            </TableSortLabel>
          </Tooltip>
        ) : (
          <FieldTitle
            label={field.props.label}
            source={field.props.source}
            resource={resource}
          />
        )}
      </div>
    </TableCell>
  )
}

const SongDatagridHeader = (props) => {
  const {
    children,
    classes,
    className,
    hasExpand = false,
    hasBulkActions = false,
    isRowSelectable,
  } = props

  const resource = useResourceContext(props)
  const translate = useTranslate()
  const dispatch = useDispatch()
  const columnsOrder =
    useSelector((state) => state.settings.columnsOrder?.[resource]) || []
  const [activeId, setActiveId] = useState(null)
  const { currentSort, data, ids, onSelect, selectedIds, setSort } =
    useListContext(props)

  const updateSortCallback = useCallback(
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
  const updateSort = setSort ? updateSortCallback : null

  const handleSelectAll = useCallback(
    (event) => {
      onSelect(
        event.target.checked
          ? ids
              .filter((id) =>
                isRowSelectable ? isRowSelectable(data[id]) : true,
              )
              .concat(selectedIds.filter((id) => !ids.includes(id)))
          : [],
      )
    },
    [data, ids, onSelect, isRowSelectable, selectedIds],
  )

  const selectableIds = isRowSelectable
    ? ids.filter((id) => isRowSelectable(data[id]))
    : ids

  const childArray = Children.toArray(children).filter((c) => isValidElement(c))
  const draggable = childArray.filter((f) =>
    columnsOrder.includes(f.props.source),
  )

  const staticBefore = []
  const staticAfter = []
  let pastDraggable = false
  childArray.forEach((field) => {
    if (columnsOrder.includes(field.props.source)) {
      pastDraggable = true
    } else if (!pastDraggable) {
      staticBefore.push(field)
    } else {
      staticAfter.push(field)
    }
  })

  const visibleIds = draggable.map((f) => f.props.source)

  const handleDragEnd = ({ active, over }) => {
    if (over && active.id !== over.id) {
      const visible = [...visibleIds]
      const fromIndex = visible.indexOf(active.id)
      const toIndex = visible.indexOf(over.id)
      visible.splice(fromIndex, 1)
      visible.splice(toIndex, 0, active.id)
      const updated = [...columnsOrder]
      let vi = 0
      for (let i = 0; i < updated.length; i++) {
        if (visibleIds.includes(updated[i])) {
          updated[i] = visible[vi++]
        }
      }
      dispatch(setColumnsOrder({ [resource]: updated }))
    }
    setActiveId(null)
  }

  const renderOverlay = (id) => {
    const field = draggable.find((f) => f.props.source === id)
    if (!field) return null
    return (
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          padding: '15px',
          background: 'white',
          boxShadow: '0px 3px 3px rgba(0,0,0,0.15)',
        }}
      >
        <DragIndicatorIcon className="dragHandle" />
        <FieldTitle
          label={field.props.label}
          source={field.props.source}
          resource={resource}
        />
      </div>
    )
  }

  return (
    <TableHead className={clsx(className, classes.thead)}>
      <TableRow className={clsx(classes.row, classes.headerRow)}>
        {hasExpand && (
          <TableCell
            padding="none"
            className={clsx(classes.headerCell, classes.expandHeader)}
          />
        )}
        {hasBulkActions && selectedIds && (
          <TableCell padding="checkbox" className={classes.headerCell}>
            <Checkbox
              aria-label={translate('ra.action.select_all', { _: 'Select all' })}
              className="select-all"
              color="primary"
              checked={
                selectedIds.length > 0 &&
                selectableIds.length > 0 &&
                selectableIds.every((id) => selectedIds.includes(id))
              }
              onChange={handleSelectAll}
            />
          </TableCell>
        )}
        {staticBefore.map((field, index) => (
          <DatagridHeaderCell
            key={field.props.source || index}
            className={classes.headerCell}
            currentSort={currentSort}
            field={field}
            isSorting={
              currentSort.field ===
              (field.props.sortBy || field.props.source)
            }
            resource={resource}
            updateSort={updateSort}
          />
        ))}
        <DndContext onDragStart={({ active }) => setActiveId(active.id)} onDragEnd={handleDragEnd}>
          {draggable.map((field) => (
            <DraggableHeaderCell
              key={field.props.source}
              id={field.props.source}
              field={field}
              currentSort={currentSort}
              updateSort={updateSort}
              resource={resource}
              className={classes.headerCell}
            />
          ))}
          <DragOverlay>{activeId ? renderOverlay(activeId) : null}</DragOverlay>
        </DndContext>
        {staticAfter.map((field, index) => (
          <DatagridHeaderCell
            key={field.props.source || index}
            className={classes.headerCell}
            currentSort={currentSort}
            field={field}
            isSorting={
              currentSort.field ===
              (field.props.sortBy || field.props.source)
            }
            resource={resource}
            updateSort={updateSort}
          />
        ))}
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
      header={<SongDatagridHeader />}
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
