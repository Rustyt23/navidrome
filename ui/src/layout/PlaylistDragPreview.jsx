import React from 'react'
import { makeStyles } from '@material-ui/core'
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
    zIndex: theme.zIndex.modal + 4,
  },
  previewWrapper: {
    transformOrigin: 'top left',
  },
  preview: {
    backgroundColor: 'rgba(255, 43, 138, 0.45)', // pink w/ transparency
    color: '#FFFFFF',
    fontWeight: 400,
    borderRadius: theme.shape.borderRadius * 2,
    boxShadow: '0 18px 40px rgba(0, 0, 0, 0.25)',
    padding: theme.spacing(1, 2.5),
    minWidth: 120,
    maxWidth: 320,
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    fontSize: theme.typography.pxToRem(14),
    letterSpacing: 0.2,
    textAlign: 'center',
    backdropFilter: 'blur(6px)',
    gap: theme.spacing(1.5),
  },
  iconWrapper: {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: 0,
  },
}))

// Precise positioning — preview follows cursor accurately
const getItemStyles = (currentOffset) => {
  if (!currentOffset) return { display: 'none' }

  const { x, y } = currentOffset
  const transform = `translate(${x}px, ${y}px)` // exact cursor alignment
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

  if (!isDragging || itemType !== DraggableTypes.PLAYLIST || !item?.name) {
    return null
  }

  return (
    <div className={classes.layer}>
      <div className={classes.previewWrapper} style={getItemStyles(currentOffset)}>
        <div className={classes.preview}>
          <div className={classes.iconWrapper}>
            <RiPlayListFill size={18} />
          </div>
          {item.name}
        </div>
      </div>
    </div>
  )
}

export default PlaylistDragPreview
