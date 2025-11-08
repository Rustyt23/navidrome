import { useCallback } from 'react'
import { useDrag, useDrop } from 'react-dnd'

const useDragAndDrop = (type, item, accepts, onDrop) => {
  const [{ isDragging }, dragRef, previewRef] = useDrag(() => ({
    type,
    item,
    collect: (monitor) => ({ isDragging: !!monitor.isDragging() }),
    options: { dropEffect: 'move' },
  }))

  const [, dropRef] = useDrop(() => ({
    accept: accepts,
    drop: onDrop,
  }))

  const dragDropRef = useCallback(
    (node) => {
      dragRef(dropRef(node))
    },
    [dragRef, dropRef],
  )

  const setDragPreview = useCallback(
    (node, options) => {
      if (typeof previewRef === 'function') {
        previewRef(node, options)
      }
    },
    [previewRef],
  )

  return {
    dragDropRef,
    isDragging,
    setDragPreview,
  }
}

export default useDragAndDrop