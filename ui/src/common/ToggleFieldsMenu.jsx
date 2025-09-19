import React, { useState, useCallback } from 'react'
import PropTypes from 'prop-types'
import IconButton from '@material-ui/core/IconButton'
import Menu from '@material-ui/core/Menu'
import MenuItem from '@material-ui/core/MenuItem'
import { makeStyles, Typography } from '@material-ui/core'
import MoreVertIcon from '@material-ui/icons/MoreVert'
import Checkbox from '@material-ui/core/Checkbox'
import { useDispatch, useSelector } from 'react-redux'
import { useTranslate } from 'react-admin'
import { setToggleableFields, setFieldsOrder } from '../actions'
import ReactDragListView from 'react-drag-listview'
import DragIndicatorIcon from '@material-ui/icons/DragIndicator'

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
  dragHandle: {
    cursor: 'move',
    marginRight: '0.5rem',
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
  const fieldsOrder = useSelector(
    (state) => state.settings.fieldsOrder[resource] || [],
  )

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

  const handleDragEnd = useCallback(
    (from, to) => {
      const newOrder = fieldsOrder.slice()
      const item = newOrder.splice(from, 1)[0]
      newOrder.splice(to, 0, item)
      dispatch(setFieldsOrder({ [resource]: newOrder }))
    },
    [dispatch, fieldsOrder, resource],
  )

  const orderedColumns = fieldsOrder.filter((k) => k in toggleableColumns)
    .concat(Object.keys(toggleableColumns).filter((k) => !fieldsOrder.includes(k)))

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
                nodeSelector="li"
                handleSelector=".drag-handle"
                onDragEnd={handleDragEnd}
              >
                {orderedColumns.map((key) =>
                  !omittedColumns.includes(key) ? (
                    <MenuItem key={key} onClick={() => handleClick(key)}>
                      <DragIndicatorIcon className={`drag-handle ${classes.dragHandle}`} />
                      <Checkbox checked={toggleableColumns[key]} />
                      {translate(`resources.${resource}.fields.${key}`)}
                    </MenuItem>
                  ) : null,
                )}
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
