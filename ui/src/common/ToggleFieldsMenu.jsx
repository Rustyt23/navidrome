import React, { useState } from 'react'
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
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'
import ReactDragListView from 'react-drag-listview'

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
  menuItem: {
    display: 'flex',
    alignItems: 'center',
  },
  dragHandle: {
    marginRight: '0.5rem',
    display: 'flex',
    alignItems: 'center',
    color: 'inherit',
    cursor: 'grab',
  },
})

export const ToggleFieldsMenu = ({
  resource,
  topbarComponent: TopBarComponent,
  hideColumns,
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

  const handleOpen = (event) => {
    setAnchorEl(event.currentTarget)
  }
  const handleClose = () => {
    setAnchorEl(null)
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

  const visibleColumns = Object.entries(toggleableColumns || {}).filter(
    ([key]) => !omittedColumns.includes(key),
  )

  const handleReorder = (fromIndex, toIndex) => {
    if (
      fromIndex === toIndex ||
      !toggleableColumns ||
      !visibleColumns.length
    )
      return

    const allEntries = Object.entries(toggleableColumns)
    const reorderedVisible = [...visibleColumns]
    const [moved] = reorderedVisible.splice(fromIndex, 1)
    reorderedVisible.splice(toIndex, 0, moved)

    const merged = []
    const queue = [...reorderedVisible]

    for (const entry of allEntries) {
      if (omittedColumns.includes(entry[0])) {
        merged.push(entry)
      } else {
        merged.push(queue.shift())
      }
    }

    dispatch(setToggleableFields({ [resource]: Object.fromEntries(merged) }))
  }

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
              <ReactDragListView
                onDragEnd={handleReorder}
                nodeSelector=".MuiMenuItem-root"
                handleSelector=".drag-handle"
              >
                <div>
                  {visibleColumns.map(([key, val]) => (
                    <MenuItem
                      key={key}
                      onClick={() => handleClick(key)}
                      className={classes.menuItem}
                    >
                      <span className={`drag-handle ${classes.dragHandle}`}>
                        <DragIndicatorIcon fontSize="small" />
                      </span>
                      <Checkbox checked={val} />
                      {translate(`resources.${resource}.fields.${key}`)}
                    </MenuItem>
                  ))}
                </div>
              </ReactDragListView>
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
}
