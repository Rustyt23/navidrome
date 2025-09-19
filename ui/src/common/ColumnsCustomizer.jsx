import React, { useState, useCallback } from 'react'
import PropTypes from 'prop-types'
import {
  IconButton,
  Menu,
  Typography,
  List,
  ListItem,
  ListItemIcon,
  ListItemText,
  Checkbox,
  Divider,
} from '@material-ui/core'
import ViewColumnIcon from '@material-ui/icons/ViewColumn'
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'
import { DndProvider, useDrag, useDrop } from 'react-dnd'
import { HTML5Backend } from 'react-dnd-html5-backend'

const COLUMN = 'COLUMN'

const MovableItem = ({ id, index, label, moveItem, visible, toggle }) => {
  const ref = React.useRef(null)
  const [, drop] = useDrop({
    accept: COLUMN,
    hover(item) {
      if (!ref.current) return
      if (item.index === index) return
      moveItem(item.index, index)
      item.index = index
    },
  })
  const [, drag] = useDrag({
    type: COLUMN,
    item: { id, index },
  })
  drag(drop(ref))
  return (
    <ListItem ref={ref} dense>
      <ListItemIcon
        onClick={(e) => e.stopPropagation()}
        style={{ cursor: 'move' }}
      >
        <DragIndicatorIcon fontSize="small" />
      </ListItemIcon>
      <Checkbox
        checked={visible}
        onClick={(e) => {
          e.stopPropagation()
          toggle(id)
        }}
      />
      <ListItemText primary={label} />
    </ListItem>
  )
}

MovableItem.propTypes = {
  id: PropTypes.string.isRequired,
  index: PropTypes.number.isRequired,
  label: PropTypes.node,
  moveItem: PropTypes.func.isRequired,
  visible: PropTypes.bool.isRequired,
  toggle: PropTypes.func.isRequired,
}

export const ColumnsCustomizer = ({
  columns,
  pinnedColumns,
  columnOrder,
  onOrderChange,
  visibleColumns,
  onVisibilityChange,
  onReset,
}) => {
  const [anchorEl, setAnchorEl] = useState(null)
  const open = Boolean(anchorEl)
  const handleOpen = (e) => setAnchorEl(e.currentTarget)
  const handleClose = () => setAnchorEl(null)

  const moveItem = useCallback(
    (from, to) => {
      const updated = [...columnOrder]
      const [moved] = updated.splice(from, 1)
      updated.splice(to, 0, moved)
      onOrderChange(updated)
    },
    [columnOrder, onOrderChange],
  )

  return (
    <div style={{ display: 'inline-block' }}>
      <IconButton onClick={handleOpen} size="small">
        <ViewColumnIcon />
      </IconButton>
      <Menu anchorEl={anchorEl} keepMounted open={open} onClose={handleClose}>
        <Typography style={{ margin: '0.5rem 1rem' }}>
          Columns To Display
        </Typography>
        <DndProvider backend={HTML5Backend}>
          <List dense>
            {pinnedColumns.map((col) => (
              <ListItem key={col.id} dense>
                <ListItemIcon
                  style={{ opacity: 0.3, cursor: 'not-allowed' }}
                  onClick={(e) => e.stopPropagation()}
                >
                  <DragIndicatorIcon fontSize="small" />
                </ListItemIcon>
                <Checkbox checked disabled />
                <ListItemText primary={col.label} />
              </ListItem>
            ))}
            {pinnedColumns.length > 0 && <Divider />}
            {columnOrder.map((id, index) => {
              const col = columns.find((c) => c.id === id)
              if (!col) return null
              return (
                <MovableItem
                  key={id}
                  id={id}
                  index={index}
                  label={col.label}
                  moveItem={moveItem}
                  visible={visibleColumns.has(id)}
                  toggle={onVisibilityChange}
                />
              )
            })}
          </List>
        </DndProvider>
        <Divider />
        <ListItem button onClick={onReset}>
          <ListItemText primary="Reset to default" />
        </ListItem>
      </Menu>
    </div>
  )
}

ColumnsCustomizer.propTypes = {
  columns: PropTypes.array.isRequired,
  pinnedColumns: PropTypes.array,
  columnOrder: PropTypes.array.isRequired,
  onOrderChange: PropTypes.func.isRequired,
  visibleColumns: PropTypes.instanceOf(Set).isRequired,
  onVisibilityChange: PropTypes.func.isRequired,
  onReset: PropTypes.func.isRequired,
}

export default ColumnsCustomizer
