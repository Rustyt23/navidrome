import React, { useState } from 'react'
import PropTypes from 'prop-types'
import IconButton from '@material-ui/core/IconButton'
import Menu from '@material-ui/core/Menu'
import MenuItem from '@material-ui/core/MenuItem'
import { makeStyles, Typography } from '@material-ui/core'
import MoreVertIcon from '@material-ui/icons/MoreVert'
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'
import Checkbox from '@material-ui/core/Checkbox'
import { useDispatch, useSelector } from 'react-redux'
import { useTranslate } from 'react-admin'
import {
  DndContext,
  useDraggable,
  useDroppable,
  DragOverlay,
} from '@dnd-kit/core'
import { setToggleableFields, setColumnsOrder } from '../actions'

const useStyles = makeStyles({
  menuIcon: {
    position: 'relative',
    top: '-0.5em',
  },
  menu: {
    width: '22rem',
    maxWidth: 'calc(100vw - 32px)',
  },
  columns: {
    maxHeight: '21rem',
    overflow: 'auto',
  },
  title: {
    margin: '1rem',
  },
  item: {
    whiteSpace: 'normal',
    '& .dragHandle, & .MuiCheckbox-root': {
      flexShrink: 0,
    },
  },
  label: {
    minWidth: 0,
    overflowWrap: 'break-word',
  },
})

export const ToggleFieldsMenu = ({
  resource,
  preferenceKey = resource,
  topbarComponent: TopBarComponent,
  hideColumns,
}) => {
  const [anchorEl, setAnchorEl] = useState(null)
  const [activeId, setActiveId] = useState(null)
  const dispatch = useDispatch()
  const translate = useTranslate()
  const toggleableColumns = useSelector(
    (state) => state.settings.toggleableFields[preferenceKey],
  )
  const omittedColumns =
    useSelector((state) => state.settings.omittedFields[preferenceKey]) || []
  const columnsOrder =
    useSelector((state) => state.settings.columnsOrder[preferenceKey]) ||
    Object.keys(toggleableColumns || {})

  const classes = useStyles()
  const open = Boolean(anchorEl)

  const handleOpen = (event) => {
    setAnchorEl(event.currentTarget)
  }
  const handleClose = () => {
    setAnchorEl(null)
  }

  const handleClick = (selectedColumn) => {
    dispatch(
      setToggleableFields({
        [preferenceKey]: {
          ...toggleableColumns,
          [selectedColumn]: !toggleableColumns[selectedColumn],
        },
      }),
    )
  }

  const handleDragEnd = ({ active, over }) => {
    if (over && active.id !== over.id) {
      const updated = [...columnsOrder]
      const fromIndex = updated.indexOf(active.id)
      const toIndex = updated.indexOf(over.id)
      updated.splice(fromIndex, 1)
      updated.splice(toIndex, 0, active.id)
      dispatch(setColumnsOrder({ [preferenceKey]: updated }))
    }
    setActiveId(null)
  }

  const renderOverlay = (id) => (
    <MenuItem className={classes.item}>
      <DragIndicatorIcon className="dragHandle" />
      <Checkbox checked={toggleableColumns[id]} />
      <span className={classes.label}>
        {translate(`resources.${resource}.fields.${id}`)}
      </span>
    </MenuItem>
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
              <DndContext
                onDragStart={({ active }) => setActiveId(active.id)}
                onDragEnd={handleDragEnd}
              >
                {columnsOrder.map((key) =>
                  !omittedColumns.includes(key) ? (
                    <DraggableMenuItem
                      key={key}
                      id={key}
                      onClick={() => handleClick(key)}
                      checked={toggleableColumns[key]}
                      label={translate(`resources.${resource}.fields.${key}`)}
                      className={classes.item}
                      labelClassName={classes.label}
                    />
                  ) : null,
                )}
                <DragOverlay>
                  {activeId ? renderOverlay(activeId) : null}
                </DragOverlay>
              </DndContext>
            </div>
          </div>
        ) : null}
      </Menu>
    </div>
  )
}

ToggleFieldsMenu.propTypes = {
  resource: PropTypes.string.isRequired,
  preferenceKey: PropTypes.string,
  topbarComponent: PropTypes.elementType,
  hideColumns: PropTypes.bool,
}

const DraggableMenuItem = ({
  id,
  onClick,
  checked,
  label,
  className,
  labelClassName,
}) => {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id })
  const { setNodeRef: setDroppableRef } = useDroppable({ id })

  const ref = (node) => {
    setNodeRef(node)
    setDroppableRef(node)
  }

  return (
    <MenuItem
      ref={ref}
      className={className}
      style={{ opacity: isDragging ? 0.5 : 1 }}
      onClick={onClick}
    >
      <DragIndicatorIcon
        className="dragHandle"
        {...listeners}
        {...attributes}
      />
      <Checkbox checked={checked} />
      <span className={labelClassName}>{label}</span>
    </MenuItem>
  )
}

DraggableMenuItem.propTypes = {
  id: PropTypes.string.isRequired,
  onClick: PropTypes.func.isRequired,
  checked: PropTypes.bool,
  label: PropTypes.node.isRequired,
  className: PropTypes.string,
  labelClassName: PropTypes.string,
}
