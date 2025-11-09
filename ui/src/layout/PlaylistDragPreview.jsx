import React from 'react'
import { makeStyles, Typography } from '@material-ui/core'
import { useDragLayer } from 'react-dnd'
import { RiPlayListFill } from 'react-icons/ri'

import { DraggableTypes } from '../consts'

const useStyles = makeStyles((theme) => ({
  layer: {
    position: 'fixed',
    pointerEvents: 'none',
    top: 0,
    left: 0,
    width: '100%',
    height: '100%',
    zIndex: theme.zIndex.modal + 1,
  },
  previewWrapper: {
    transformOrigin: 'top left',
  },
  preview: {
    backgroundColor: theme.palette.background.paper,
    color: theme.palette.text.primary,
    borderRadius: theme.shape.borderRadius,
    boxShadow: theme.shadows[8],
    border: `1px solid ${theme.palette.divider}`,
    padding: theme.spacing(1, 1.75),
    minWidth: 160,
    maxWidth: 320,
    display: 'flex',
    alignItems: 'center',
    gap: theme.spacing(1),
  },
  iconWrapper: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    width: theme.spacing(4),
    height: theme.spacing(4),
    borderRadius: theme.shape.borderRadius,
    backgroundColor: theme.palette.action.hover,
    color: theme.palette.text.secondary,
    flexShrink: 0,
  },
  textContainer: {
    overflow: 'hidden',
  },
  title: {
    fontWeight: theme.typography.fontWeightMedium,
    fontSize: theme.typography.pxToRem(13),
    lineHeight: 1.2,
    whiteSpace: 'nowrap',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  },
  subtitle: {
    fontSize: theme.typography.pxToRem(11.5),
    color: theme.palette.text.secondary,
    marginTop: 2,
  },
}))

const getItemStyles = (currentOffset) => {
  if (!currentOffset) {
    return { display: 'none' }
  }

  const { x, y } = currentOffset
  const transform = `translate(${x + 12}px, ${y + 12}px)`

  return {
    transform,
    WebkitTransform: transform,
  }
}

const PlaylistDragPreview = () => {
  const classes = useStyles()
  const { itemType, item, isDragging, currentOffset } = useDragLayer((monitor) => ({
    item: monitor.getItem(),
    itemType: monitor.getItemType(),
    currentOffset: monitor.getClientOffset(),
    isDragging: monitor.isDragging(),
  }))

  if (!isDragging || itemType !== DraggableTypes.PLAYLIST) {
    return null
  }

  const title = item?.name || 'Playlist'

  return (
    <div className={classes.layer}>
      <div className={classes.previewWrapper} style={getItemStyles(currentOffset)}>
        <div className={classes.preview}>
          <div className={classes.iconWrapper}>
            <RiPlayListFill size={18} />
          </div>
          <div className={classes.textContainer}>
            <Typography component="div" className={classes.title}>
              {title}
            </Typography>
            <Typography component="div" className={classes.subtitle}>
              Drag to add or move
            </Typography>
          </div>
        </div>
      </div>
    </div>
  )
}

export default PlaylistDragPreview
