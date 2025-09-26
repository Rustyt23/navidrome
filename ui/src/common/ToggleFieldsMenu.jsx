import React, { useCallback, useMemo, useRef, useState } from 'react'
import PropTypes from 'prop-types'
import IconButton from '@material-ui/core/IconButton'
import Menu from '@material-ui/core/Menu'
import MenuItem from '@material-ui/core/MenuItem'
import { makeStyles, Typography } from '@material-ui/core'
import MoreVertIcon from '@material-ui/icons/MoreVert'
import Checkbox from '@material-ui/core/Checkbox'
import { useDispatch, useSelector } from 'react-redux'
import { useTranslate } from 'react-admin'
import { setToggleableFields } from '../actions'
import { DndProvider, useDrag, useDrop } from 'react-dnd'
import { HTML5Backend } from 'react-dnd-html5-backend'

const COLUMN_ITEM_TYPE = 'toggle-field-column'

const DraggableMenuItem = ({
  id,
  index,
  moveItem,
  onToggle,
  checked,
  label,
  disabled,
  draggingRef,
}) => {
  const ref = useRef(null)

  const [, drop] = useDrop({
    accept: COLUMN_ITEM_TYPE,
    hover(item) {
      if (!ref.current) return
      if (item.index === index) return
      moveItem(item.index, index)
      item.index = index
    },
  })

  const [{ isDragging }, drag] = useDrag({
    type: COLUMN_ITEM_TYPE,
    item: () => {
      if (draggingRef) draggingRef.current = true
      return { id, index }
    },
    end: () => {
      if (draggingRef) draggingRef.current = false
    },
  })

  drag(drop(ref))

  const handleClick = (event) => {
    event.preventDefault()
    event.stopPropagation()
    if (draggingRef?.current) return
    onToggle()
  }

  return (
    <MenuItem
      ref={ref}
      key={id}
      onClick={handleClick}
      disabled={disabled}
      style={{ opacity: isDragging ? 0.6 : 1, cursor: 'move' }}
    >
      <Checkbox checked={checked} />
      {label}
    </MenuItem>
  )
}

DraggableMenuItem.propTypes = {
  id: PropTypes.string.isRequired,
  index: PropTypes.number.isRequired,
  moveItem: PropTypes.func.isRequired,
  onToggle: PropTypes.func.isRequired,
  checked: PropTypes.bool.isRequired,
  label: PropTypes.node.isRequired,
  disabled: PropTypes.bool,
  draggingRef: PropTypes.shape({ current: PropTypes.bool }),
}

const useStyles = makeStyles({
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
  },
  title: {
    margin: '1rem',
  },
})

export const ToggleFieldsMenu = ({
  resource,
  topbarComponent: TopBarComponent,
  hideColumns,
  columnsOrder,
  onColumnsOrderChange,
}) => {
  const [anchorEl, setAnchorEl] = useState(null)
  const dispatch = useDispatch()
  const translate = useTranslate()
  const toggleableColumns = useSelector(
    (state) => state.settings.toggleableFields[resource],
  )
  const omittedColumns =
    useSelector((state) => state.settings.omittedFields[resource]) || []

  const classes = useStyles()
  const open = Boolean(anchorEl)
  const draggingRef = useRef(false)

  const handleOpen = (event) => {
    setAnchorEl(event.currentTarget)
  }
  const handleClose = () => {
    setAnchorEl(null)
    if (draggingRef) draggingRef.current = false
  }

  const handleClick = (selectedColumn) => {
    dispatch(
      setToggleableFields({
        [resource]: {
          ...toggleableColumns,
          [selectedColumn]: !toggleableColumns[selectedColumn],
        },
      }),
    )
  }

  const orderedEntries = useMemo(() => {
    if (!toggleableColumns) return []
    if (!columnsOrder?.length)
      return Object.entries(toggleableColumns).filter(
        ([key]) => !omittedColumns.includes(key),
      )

    const knownEntries = columnsOrder
      .filter((key) => key in toggleableColumns)
      .map((key) => [key, toggleableColumns[key]])
      .filter(([key]) => !omittedColumns.includes(key))

    const remainingEntries = Object.entries(toggleableColumns).filter(
      ([key]) =>
        !omittedColumns.includes(key) && !columnsOrder.includes(key),
    )

    return [...knownEntries, ...remainingEntries]
  }, [columnsOrder, toggleableColumns, omittedColumns])

  const moveItem = useCallback(
    (fromIndex, toIndex) => {
      if (!onColumnsOrderChange) return
      if (fromIndex === toIndex) return
      const keys = orderedEntries.map(([key]) => key)
      const nextOrder = [...keys]
      const [removed] = nextOrder.splice(fromIndex, 1)
      nextOrder.splice(toIndex, 0, removed)
      if (
        keys.length !== nextOrder.length ||
        keys.some((key, idx) => key !== nextOrder[idx])
      ) {
        onColumnsOrderChange(nextOrder)
      }
    },
    [onColumnsOrderChange, orderedEntries],
  )

  return (
    <div className={classes.menuIcon}>
      <IconButton
        aria-label="more"
        aria-controls="long-menu"
        aria-haspopup="true"
        onClick={handleOpen}
      >
        <MoreVertIcon />
      </IconButton>
      <Menu
        id="long-menu"
        anchorEl={anchorEl}
        keepMounted
        open={open}
        onClose={handleClose}
        classes={{
          paper: classes.menu,
        }}
      >
        {TopBarComponent && <TopBarComponent />}
        {!hideColumns && toggleableColumns ? (
          <div>
            <Typography className={classes.title}>
              {translate('ra.toggleFieldsMenu.columnsToDisplay')}
            </Typography>
            <div className={classes.columns}>
              {onColumnsOrderChange ? (
                <DndProvider backend={HTML5Backend}>
                  {orderedEntries.map(([key, val], index) => (
                    <DraggableMenuItem
                      key={key}
                      id={key}
                      index={index}
                      moveItem={moveItem}
                      onToggle={() => handleClick(key)}
                      checked={Boolean(val)}
                      label={translate(
                        `resources.${resource}.fields.${key}`,
                      )}
                      disabled={false}
                      draggingRef={draggingRef}
                    />
                  ))}
                </DndProvider>
              ) : (
                orderedEntries.map(([key, val]) => (
                  <MenuItem key={key} onClick={() => handleClick(key)}>
                    <Checkbox checked={val} />
                    {translate(`resources.${resource}.fields.${key}`)}
                  </MenuItem>
                ))
              )}
            </div>
          </div>
        ) : null}
      </Menu>
    </div>
  )
}

ToggleFieldsMenu.propTypes = {
  resource: PropTypes.string.isRequired,
  topbarComponent: PropTypes.elementType,
  hideColumns: PropTypes.bool,
  columnsOrder: PropTypes.arrayOf(PropTypes.string),
  onColumnsOrderChange: PropTypes.func,
}
