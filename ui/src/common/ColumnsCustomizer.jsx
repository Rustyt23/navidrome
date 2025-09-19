import React, { useState, useCallback } from 'react'
import PropTypes from 'prop-types'
import { makeStyles } from '@material-ui/core/styles'
import { List, ListItem, Checkbox, ListItemIcon, ListItemText, Button } from '@material-ui/core'
import DragHandleIcon from '@material-ui/icons/DragHandle'
import { useDrag, useDrop } from 'react-dnd'
import { DraggableTypes } from '../consts'

const useStyles = makeStyles({
  list: { maxHeight: '21rem', overflow: 'auto' },
  handle: { cursor: 'move' },
  reset: { margin: '0.5rem' },
})

const ColumnsCustomizer = ({ columns, onChange, onReset }) => {
  const classes = useStyles()
  const [items, setItems] = useState(columns)

  const moveItem = useCallback((dragIndex, hoverIndex) => {
    const updated = [...items]
    const [removed] = updated.splice(dragIndex, 1)
    updated.splice(hoverIndex, 0, removed)
    setItems(updated)
    onChange(updated)
  }, [items, onChange])

  const toggleItem = (index) => () => {
    const updated = items.map((c, i) => i === index ? { ...c, visible: !c.visible } : c)
    setItems(updated)
    onChange(updated)
  }

  const ColumnItem = ({ item, index }) => {
    const [{ isDragging }, drag] = useDrag(() => ({
      type: DraggableTypes.COLUMN,
      item: { index },
      collect: (monitor) => ({ isDragging: monitor.isDragging() }),
    }), [index])

    const [, drop] = useDrop(() => ({
      accept: DraggableTypes.COLUMN,
      hover: (dragged) => {
        if (dragged.index !== index) {
          moveItem(dragged.index, index)
          dragged.index = index
        }
      },
    }), [moveItem])

    return (
      <ListItem
        ref={(node) => drag(drop(node))}
        key={item.id}
        dense
        button
        onClick={toggleItem(index)}
        style={{ opacity: isDragging ? 0.5 : 1 }}
      >
        <ListItemIcon
          className={classes.handle}
          onClick={(e) => e.stopPropagation()}
        >
          <DragHandleIcon />
        </ListItemIcon>
        <Checkbox edge="start" checked={item.visible} tabIndex={-1} disableRipple />
        <ListItemText primary={item.label} />
      </ListItem>
    )
  }

  const handleReset = () => {
    onReset()
  }

  return (
    <div>
      <List className={classes.list}>
        {items.map((item, index) => (
          <ColumnItem key={item.id} item={item} index={index} />
        ))}
      </List>
      {onReset && (
        <Button className={classes.reset} onClick={handleReset} variant="outlined" size="small">
          Reset
        </Button>
      )}
    </div>
  )
}

ColumnsCustomizer.propTypes = {
  columns: PropTypes.arrayOf(PropTypes.shape({
    id: PropTypes.string.isRequired,
    label: PropTypes.string.isRequired,
    visible: PropTypes.bool,
  })).isRequired,
  onChange: PropTypes.func.isRequired,
  onReset: PropTypes.func,
}

export default ColumnsCustomizer
