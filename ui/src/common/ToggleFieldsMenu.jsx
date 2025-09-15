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
import ReactDragListView from 'react-drag-listview'
import { setToggleableFields, setColumnsOrder } from '../actions'

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
}) => {
  const [anchorEl, setAnchorEl] = useState(null)
  const dispatch = useDispatch()
  const translate = useTranslate()
  const toggleableColumns = useSelector(
    (state) => state.settings.toggleableFields[resource],
  )
  const omittedColumns =
    useSelector((state) => state.settings.omittedFields[resource]) || []
  const columnsOrder = useSelector(
    (state) => state.settings.columnsOrder[resource],
  ) || Object.keys(toggleableColumns || {})

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

  const handleDragEnd = (fromIndex, toIndex) => {
    const updated = [...columnsOrder]
    const [moved] = updated.splice(fromIndex, 1)
    updated.splice(toIndex, 0, moved)
    dispatch(setColumnsOrder({ [resource]: updated }))
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
                nodeSelector={'li'}
                handleSelector={'.dragHandle'}
                onDragEnd={handleDragEnd}
              >
                {columnsOrder.map((key) =>
                  !omittedColumns.includes(key) ? (
                    <MenuItem key={key} onClick={() => handleClick(key)}>
                      <DragIndicatorIcon className="dragHandle" />
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
