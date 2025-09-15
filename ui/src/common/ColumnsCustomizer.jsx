import React, { useState, useRef } from 'react'
import PropTypes from 'prop-types'
import IconButton from '@material-ui/core/IconButton'
import Menu from '@material-ui/core/Menu'
import MenuItem from '@material-ui/core/MenuItem'
import Checkbox from '@material-ui/core/Checkbox'
import MoreVertIcon from '@material-ui/icons/MoreVert'
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'
import Button from '@material-ui/core/Button'
import { makeStyles } from '@material-ui/core/styles'
import { useTranslate } from 'react-admin'
import { useDrag, useDrop } from 'react-dnd'
import { useSongColumns } from './SongDatagrid'
import { DraggableTypes } from '../consts'

const useStyles = makeStyles({
  menu: { width: '24ch' },
  columns: { maxHeight: '21rem', overflow: 'auto' },
  row: { display: 'flex', alignItems: 'center' },
  grip: { cursor: 'grab', marginRight: 8 },
  reset: { margin: '0.5rem' },
})

export const ColumnsCustomizer = () => {
  const [anchorEl, setAnchorEl] = useState(null)
  const translate = useTranslate()
  const classes = useStyles()
  const open = Boolean(anchorEl)

  const { columns, columnOrder, moveColumn, visibleColumns, toggleColumn, resetColumns } =
    useSongColumns()

  const movableColumns = columns.filter((c) => !c.pinned)
  const orderedColumns = columnOrder
    .map((id) => movableColumns.find((c) => c.id === id))
    .filter(Boolean)

  const handleOpen = (event) => setAnchorEl(event.currentTarget)
  const handleClose = () => setAnchorEl(null)

  const Row = ({ col, index }) => {
    const ref = useRef(null)
    const [, drop] = useDrop({
      accept: DraggableTypes.COLUMN,
      hover: (item) => {
        if (item.id !== col.id) moveColumn(item.id, index)
      },
    })
    const [, drag] = useDrag({ type: DraggableTypes.COLUMN, item: { id: col.id } })
    drag(drop(ref))

    const onKeyDown = (e) => {
      if (e.key === ' ') {
        e.preventDefault()
        toggleColumn(col.id)
      } else if (e.ctrlKey && e.key === 'ArrowUp') {
        e.preventDefault()
        moveColumn(col.id, Math.max(index - 1, 0))
      } else if (e.ctrlKey && e.key === 'ArrowDown') {
        e.preventDefault()
        moveColumn(col.id, Math.min(index + 1, orderedColumns.length - 1))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        ref.current?.previousSibling?.focus()
      } else if (e.key === 'ArrowDown') {
        e.preventDefault()
        ref.current?.nextSibling?.focus()
      }
    }

    return (
      <MenuItem ref={ref} onKeyDown={onKeyDown} tabIndex={0}>
        <DragIndicatorIcon className={classes.grip} />
        <Checkbox
          checked={visibleColumns.has(col.id)}
          onChange={() => toggleColumn(col.id)}
        />
        {col.label}
      </MenuItem>
    )
  }

  Row.propTypes = {
    col: PropTypes.object,
    index: PropTypes.number,
  }

  return (
    <div>
      <IconButton aria-label="more" aria-haspopup="true" onClick={handleOpen}>
        <MoreVertIcon />
      </IconButton>
      <Menu
        anchorEl={anchorEl}
        keepMounted
        open={open}
        onClose={handleClose}
        classes={{ paper: classes.menu }}
      >
        <div className={classes.columns}>
          {orderedColumns.map((col, index) => (
            <Row key={col.id} col={col} index={index} />
          ))}
        </div>
        <Button onClick={resetColumns} className={classes.reset}>
          {translate('ra.action.reset')}
        </Button>
      </Menu>
    </div>
  )
}

ColumnsCustomizer.propTypes = {}
